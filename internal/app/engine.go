package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ninoleto/rf433-button-manager/internal/receiver"
	"sync"
	"time"
)

const Version = "0.4.0"

type App struct {
	reception                           ReceptionDiagnostics
	recording                           *ReceptionRecording
	pulseStream                         receiver.PulseStream
	discoverReceivers                   func() ([]receiver.DeviceInfo, error)
	lastDecodedAt                       string
	mu                                  sync.Mutex
	actionGate                          sync.RWMutex
	store                               *Store
	config                              Config
	receiver                            receiver.Status
	receiverMode                        string
	learning                            *Learning
	learningNotice                      *Learning
	terminalCapture                     *Learning // Retains cleanup evidence even after the dialog is dismissed.
	rfCapture                           *RFCapture
	captureEnabled                      bool
	captureSuppressed                   bool
	analysisActive                      bool
	analyzeCapture                      func(context.Context, receiver.AnalysisRequest) (receiver.AnalysisResult, error)
	rtlBinary, captureTempDir           string
	receiverGeneration, receiverApplied uint64
	events                              []Event
	inFlight                            map[string]bool
	actionSlots                         chan struct{}
	lastSeen                            map[string]time.Time
	lastTest                            map[string]time.Time
	muteUntil                           time.Time
	epoch                               uint64
	storageErr                          bool
	demo                                bool
	closed                              bool
	now                                 func() time.Time
	ctx                                 context.Context
	cancel                              context.CancelFunc
	wg                                  sync.WaitGroup
	settingsChanged                     chan struct{}
}

func New(store *Store, c Config, demo bool) (*App, error) {
	events, e := store.Events(150)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &App{discoverReceivers: receiver.Devices, store: store, config: c, inFlight: map[string]bool{}, actionSlots: make(chan struct{}, 8), events: events, lastSeen: map[string]time.Time{}, lastTest: map[string]time.Time{}, muteUntil: time.Now().Add(2 * time.Second), demo: demo, now: time.Now, ctx: ctx, cancel: cancel, settingsChanged: make(chan struct{}, 1), receiver: receiver.Status{State: "disconnected", Message: "Receiver has not started"}}, nil
}
func (a *App) Close() {
	a.mu.Lock()
	a.stopRecordingLocked("service_stopping")
	a.closed = true
	a.cancel()
	a.mu.Unlock()
	a.wg.Wait()
}
func (a *App) SettingsChanges() <-chan struct{} { return a.settingsChanged }
func (a *App) Settings() Settings               { a.mu.Lock(); defer a.mu.Unlock(); return clone(a.config.Settings) }
func (a *App) eventLocked(kind, message, button string, data json.RawMessage, levels ...*receiver.SignalLevels) bool {
	a.recordActionEventLocked(kind, message, button)
	if button != "" && (kind == "webhook_start" || kind == "webhook_ok" || kind == "webhook_error" || kind == "webhook_unknown" || kind == "webhook_busy") {
		if d := a.reception.Buttons[button]; d != nil {
			d.Action = message
			d.ActionAt = a.now().UTC().Format(time.RFC3339Nano)
		}
	}
	if len(data) > 8192 {
		data = nil
		message += " (large raw payload omitted from history)"
	}
	ev := Event{Time: a.now().UTC().Format(time.RFC3339Nano), Kind: kind, Message: message, ButtonID: button, Data: data}
	if len(levels) == 1 {
		ev.Levels = levels[0]
	}
	if kind == "rf" || kind == "matched" || kind == "receiver" {
		ev.Receiver = &ReceiverSource{Name: a.receiver.Name, Serial: a.receiver.Serial, USBPort: a.receiver.USBPort}
	}
	saved, e := a.store.Record(ev)
	if e != nil {
		a.storageErr = true
		a.recordDiagnosticLocked(RecordingEvent{Kind: "storage_error", ButtonID: button})
		ev.Kind = "storage_error"
		ev.Message = "Cannot persist events; actions paused until service restart"
		ev.Data = nil
		saved = ev
	}
	a.events = append([]Event{saved}, a.events...)
	if len(a.events) > 150 {
		a.events = a.events[:150]
	}
	return e == nil
}
func (a *App) SetReceiver(s receiver.Status) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.setReceiverLocked(s)
}
func (a *App) setReceiverLocked(s receiver.Status) {
	if a.closed {
		return
	}
	if a.receiver.Serial != s.Serial || a.receiver.USBPort != s.USBPort || s.State != "connected" {
		a.lastDecodedAt = ""
		if a.receiver.State == "connected" || a.receiver.Serial != s.Serial || a.receiver.USBPort != s.USBPort {
			a.resetReceptionLocked()
		}
	}
	if a.receiver.State != s.State || a.receiver.Message != s.Message || a.receiver.Serial != s.Serial || a.receiver.USBPort != s.USBPort {
		a.receiver = s
		a.eventLocked("receiver", s.State+": "+s.Message, "", nil)
	}
	a.receiver = s
	if s.State != "connected" {
		a.stopRecordingLocked("receiver_unavailable")
	}
}
func (a *App) expireLocked() {
	if a.recording != nil && a.recording.Active && !a.now().Before(a.recording.expires) {
		a.stopRecordingLocked("time_limit")
	}
	if a.learning != nil && !a.now().Before(a.learning.expires) {
		a.finishLearningLocked("timeout", "Learning timed out. No mapping changed. Retry when you are ready to press the button.")
	}
	if l := a.learning; l != nil && l.Automatic && (l.Stage == "waiting" || l.Stage == "receiving" || l.Stage == "analyzing") && !a.now().Before(l.armedAt.Add(learningPressWindow)) {
		l.Diagnostics.FailureCode = "press_window_timeout"
		l.Diagnostics.FailureReason = "The armed 20-second window expired without a verified matching press; ignored traffic did not extend it"
		a.finishLearningLocked("press_timeout", "No matching press was verified before this press window expired. Background captures were ignored; the intended remote's compatibility remains unverified. No mapping changed.")
	}
}
func (a *App) State() any {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.expireLocked()
	return clone(struct {
		Remotes               []Remote                  `json:"remotes"`
		Settings              Settings                  `json:"settings"`
		Receiver              receiver.Status           `json:"receiver"`
		Learning              *Learning                 `json:"learning"`
		Events                []Event                   `json:"events"`
		Version               string                    `json:"version"`
		Demo                  bool                      `json:"demo"`
		StorageError          bool                      `json:"storage_error"`
		LearningNotice        *Learning                 `json:"learning_notice"`
		ActionsPaused         bool                      `json:"actions_paused"`
		DecoderProfiles       []receiver.DecoderProfile `json:"decoder_profiles"`
		ReceiverMode          string                    `json:"receiver_mode"`
		ReceiverGeneration    uint64                    `json:"receiver_generation"`
		RFCaptureAvailable    bool                      `json:"rf_capture_available"`
		ReceiverLastDecodedAt string                    `json:"receiver_last_decoded_at"`
		Reception             ReceptionDiagnostics      `json:"reception"`
		Recording             *RecordingStatus          `json:"reception_recording"`
		RecordingSupported    bool                      `json:"reception_recording_supported"`
	}{a.config.Remotes, a.config.Settings, a.receiver, a.learning, a.events, Version, a.demo, a.storageErr, a.learningNotice, a.learning != nil || a.captureSuppressed, a.config.DecoderProfiles, a.receiverMode, a.receiverApplied, a.rfCapture != nil && a.rfCapture.Bytes > 0, a.lastDecodedAt, a.reception, a.recordingStatusLocked(), a.captureEnabled || a.demo})
}
func findButton(c *Config, id string) *Button {
	for ri := range c.Remotes {
		for bi := range c.Remotes[ri].Buttons {
			b := &c.Remotes[ri].Buttons[bi]
			if b.ID == id {
				return b
			}
		}
	}
	return nil
}
func (a *App) mutate(fn func(*Config) error) error {
	return a.mutateConfig(fn, false)
}
func (a *App) mutateConfig(fn func(*Config) error, restartReceiver bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.expireLocked()
	if a.learning != nil || a.captureSuppressed {
		return errors.New("finish or cancel learning before editing configuration")
	}
	next := clone(a.config)
	if e := fn(&next); e != nil {
		return e
	}
	if e := validateConfig(next); e != nil {
		return e
	}
	if e := a.store.Save(next); e != nil {
		a.storageErr = true
		return errors.New("configuration could not be saved")
	}
	a.pruneReceptionLocked(&next)
	a.stopRecordingLocked("configuration_changed")
	a.config = next
	a.epoch++
	if restartReceiver {
		a.receiverChangedLocked()
	}
	return nil
}
func (a *App) CreateRemote(name string, columns, count int) (Remote, error) {
	if count == 0 {
		count = 6
	}
	if columns == 0 {
		columns = 2
	}
	r := Remote{ID: newID(), Name: name, Columns: columns, Buttons: []Button{}}
	if count < 1 || count > 64 {
		return r, errors.New("button count must be 1–64")
	}
	labels := []string{"A ON", "A OFF", "B ON", "B OFF", "C ON", "C OFF"}
	for i := 0; i < count; i++ {
		label := fmt.Sprintf("Button %d", i+1)
		if count == 6 {
			label = labels[i]
		}
		r.Buttons = append(r.Buttons, newButton(label))
	}
	e := a.mutate(func(c *Config) error {
		for i := range r.Buttons {
			r.Buttons[i].IdentityFields = append([]string{}, c.Settings.IdentityFields...)
		}
		c.Remotes = append(c.Remotes, r)
		return nil
	})
	return r, e
}
func (a *App) EditRemote(id, name string, columns int) (Remote, error) {
	var out Remote
	e := a.mutate(func(c *Config) error {
		for i := range c.Remotes {
			if c.Remotes[i].ID == id {
				c.Remotes[i].Name = name
				c.Remotes[i].Columns = columns
				out = c.Remotes[i]
				return nil
			}
		}
		return errors.New("remote not found")
	})
	return out, e
}
func (a *App) DuplicateRemote(id string) (Remote, error) {
	var out Remote
	e := a.mutate(func(c *Config) error {
		for _, r := range c.Remotes {
			if r.ID == id {
				out = clone(r)
				out.ID = newID()
				out.Name = r.Name + " copy"
				for i := range out.Buttons {
					out.Buttons[i].ID = newID()
					out.Buttons[i].Signature = ""
					out.Buttons[i].DecoderProfileID = ""
					out.Buttons[i].Enabled = false
					out.Buttons[i].LastReceived = ""
				}
				c.Remotes = append(c.Remotes, out)
				return nil
			}
		}
		return errors.New("remote not found")
	})
	return out, e
}
func (a *App) DeleteRemote(id string) error {
	return a.mutate(func(c *Config) error {
		for i, r := range c.Remotes {
			if r.ID == id {
				c.Remotes = append(c.Remotes[:i], c.Remotes[i+1:]...)
				return nil
			}
		}
		return errors.New("remote not found")
	})
}
func (a *App) CreateButton(id, name string) (Button, error) {
	b := newButton(name)
	e := a.mutate(func(c *Config) error {
		for i := range c.Remotes {
			if c.Remotes[i].ID == id {
				b.IdentityFields = append([]string{}, c.Settings.IdentityFields...)
				c.Remotes[i].Buttons = append(c.Remotes[i].Buttons, b)
				return nil
			}
		}
		return errors.New("remote not found")
	})
	return b, e
}
func (a *App) EditButton(id string, next Button) (Button, error) {
	return a.editButton(id, next, "")
}

// A destination approved in the button editor is saved atomically with that
// button. Ordinary API edits still require an already allowed destination.
func (a *App) editButton(id string, next Button, allowOrigin string) (Button, error) {
	var out Button
	e := a.mutate(func(c *Config) error {
		b := findButton(c, id)
		if b == nil {
			return errors.New("button not found")
		}
		if allowOrigin != "" {
			destination, err := origin(next.Webhook.URL)
			if err != nil || destination != allowOrigin {
				return errors.New("approved destination must match this button's webhook origin")
			}
			found := false
			for _, allowed := range c.Settings.AllowedOrigins {
				found = found || allowed == destination
			}
			if !found {
				c.Settings.AllowedOrigins = append(c.Settings.AllowedOrigins, destination)
			}
		}
		next.ID = id
		next.Signature = b.Signature
		next.DecoderProfileID = b.DecoderProfileID
		next.LastReceived = b.LastReceived
		oldFields, _ := json.Marshal(b.IdentityFields)
		newFields, _ := json.Marshal(next.IdentityFields)
		if string(oldFields) != string(newFields) {
			next.Signature = ""
			next.DecoderProfileID = ""
			next.Enabled = false
		}
		*b = next
		out = next
		return nil
	})
	return out, e
}
func (a *App) ClearButton(id string) error {
	return a.mutate(func(c *Config) error {
		b := findButton(c, id)
		if b == nil {
			return errors.New("button not found")
		}
		fresh := newButton(b.Name)
		fresh.ID = id
		fresh.IdentityFields = append([]string{}, c.Settings.IdentityFields...)
		*b = fresh
		return nil
	})
}
func (a *App) DeleteButton(id string) error {
	return a.mutate(func(c *Config) error {
		for ri := range c.Remotes {
			for bi, b := range c.Remotes[ri].Buttons {
				if b.ID == id {
					c.Remotes[ri].Buttons = append(c.Remotes[ri].Buttons[:bi], c.Remotes[ri].Buttons[bi+1:]...)
					return nil
				}
			}
		}
		return errors.New("button not found")
	})
}
func (a *App) UpdateSettings(s Settings) error {
	return a.mutateConfig(func(c *Config) error {
		if c.Settings.ReceiverSerial != s.ReceiverSerial {
			devices, err := a.discoverReceivers()
			if err != nil {
				return err
			}
			if err := receiver.ValidateSelection(devices, s.ReceiverSerial); err != nil {
				return err
			}
		}
		c.Settings = s
		return nil
	}, true)
}
func (a *App) Backup() Config { a.mu.Lock(); defer a.mu.Unlock(); return clone(a.config) }
func (a *App) Restore(c Config) error {
	migrateConfig(&c)
	if e := validateConfig(c); e != nil {
		return e
	}
	for i := range c.Remotes {
		for j := range c.Remotes[i].Buttons {
			c.Remotes[i].Buttons[j].Enabled = false
			c.Remotes[i].Buttons[j].LastReceived = ""
		}
	}
	return a.mutateConfig(func(next *Config) error { *next = c; return nil }, true)
}

func (a *App) StartLearning(id string, fields []string) (*Learning, error) {
	return a.startLearning(id, fields, false)
}

func (a *App) StartInspection() (*Learning, error) {
	return a.startLearning("", nil, true)
}

func (a *App) startLearning(id string, fields []string, inspect bool) (*Learning, error) {
	a.actionGate.Lock()
	defer a.actionGate.Unlock()
	a.mu.Lock()
	defer a.mu.Unlock()
	a.expireLocked()
	if a.learning != nil || a.captureSuppressed {
		return nil, errors.New("another learning or inspection session is active")
	}
	b := findButton(&a.config, id)
	if b == nil && !inspect {
		return nil, errors.New("button not found")
	}
	if len(fields) == 0 {
		if inspect {
			fields = a.config.Settings.IdentityFields
		} else {
			fields = b.IdentityFields
		}
	}
	if e := validateFields(fields); e != nil {
		return nil, e
	}
	now := a.now()
	a.epoch++
	a.learningNotice = nil
	a.stopRecordingLocked("inspection_or_learning_started")
	a.learning = &Learning{SessionID: newID(), ButtonID: id, Stage: "waiting", Message: "Press the physical button once, then release it.", Required: 3, Fields: append([]string{}, fields...), expires: now.Add(90 * time.Second), armedAt: now, ExpiresAt: now.Add(90 * time.Second).UTC().Format(time.RFC3339)}
	a.learning.InspectOnly = inspect
	if a.captureEnabled && !a.demo {
		l := a.learning
		l.Automatic = true
		l.WindowAnalysisLimit = maxWindowAnalyses
		l.Stage = "preparing"
		l.Message = "Preparing the receiver for learning. All actions are paused."
		l.expires = now.Add(180 * time.Second)
		l.ExpiresAt = l.expires.UTC().Format(time.RFC3339)
		l.ctx, l.cancel = context.WithCancel(a.ctx)
		a.captureSuppressed = true
		a.receiverChangedLocked()
		l.CaptureGeneration = a.receiverGeneration
		a.rfCapture = &RFCapture{Format: "rf433-button-manager-capture-v1", Version: Version, StartedAt: now.UTC().Format(time.RFC3339Nano), FrequencyHz: 433920000, SampleRate: 250000, AdaptiveSensitivity: a.config.Settings.AdaptiveSensitivity, generation: l.CaptureGeneration, started: now}
		l.CaptureReceiver = receiver.Status{State: "recovering", Message: "Capture receiver plan has not started"}
		l.CaptureReceiverAt = now.UTC().Format(time.RFC3339Nano)
	}
	a.eventLocked("learning", "Learning started; all webhook actions paused", id, nil)
	return clone(a.learning), nil
}
func (a *App) ArmLearning() (*Learning, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.expireLocked()
	l := a.learning
	if l == nil || l.Stage != "captured" {
		return nil, errors.New("capture a candidate before arming the next press")
	}
	lastTransmission := l.lastRF
	if l.decodedSource {
		lastTransmission = l.lastDecoded
	}
	if l.Automatic && !lastTransmission.IsZero() && a.now().Sub(lastTransmission) < time.Second {
		return nil, errors.New("release the button and wait for one second of RF silence before the next press")
	}
	l.Stage = "waiting"
	l.Message = "Press the same physical button once, then release it."
	l.armedAt = a.now()
	if l.Automatic {
		l.WindowExpiresAt = l.armedAt.Add(learningPressWindow).UTC().Format(time.RFC3339Nano)
		l.WindowAnalyses = 0
	}
	l.firstPulse = time.Time{}
	l.packets = nil
	l.bytes = 0
	return clone(l), nil
}
func (a *App) CancelLearning() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	message := "Learning cancelled; previous mapping kept"
	if a.learning != nil && a.learning.InspectOnly {
		message = "Inspection stopped; no mapping or action changed"
	}
	a.finishLearningLocked("cancelled", message)
	a.learningNotice = nil
	return nil
}
func signaturesOverlap(one, two string) bool {
	var a, b map[string]json.RawMessage
	if json.Unmarshal([]byte(one), &a) != nil || json.Unmarshal([]byte(two), &b) != nil {
		return false
	}
	return signatureValuesOverlap(a, b)
}

func (a *App) ConfirmLearning() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.expireLocked()
	l := a.learning
	if l != nil && l.InspectOnly {
		return errors.New("inspection cannot save a mapping; use Learn on a button")
	}
	if l == nil || l.Stage != "ready" || l.Presses < 3 {
		return errors.New("three separate matching presses are required")
	}
	next := clone(a.config)
	b := findButton(&next, l.ButtonID)
	if b == nil {
		return errors.New("button not found")
	}
	b.Signature = l.Candidate
	b.DecoderProfileID = ""
	if l.Profile != nil {
		profile := *l.Profile
		profile.ValidatedPresses = 3
		profile.CreatedAt = a.now().UTC().Format(time.RFC3339)
		found := false
		for _, existing := range next.DecoderProfiles {
			if existing.ID == profile.ID {
				found = true
			}
		}
		if !found {
			next.DecoderProfiles = append(next.DecoderProfiles, profile)
		}
		b.DecoderProfileID = profile.ID
	}
	b.IdentityFields = append([]string{}, l.Fields...)
	b.Enabled = false
	for _, r := range next.Remotes {
		for _, other := range r.Buttons {
			if other.ID != b.ID && other.Signature != "" &&
				(signaturesOverlap(other.Signature, l.Candidate) ||
					decoderMappingsOverlap(other.Signature, l.Candidate, other.IdentityFields, l.Fields) ||
					profileMappingsOverlap(other.Signature, l.Candidate, other.IdentityFields, l.Fields, next.DecoderProfiles)) {
				return errors.New("signal overlaps an existing button mapping; clear that mapping first")
			}
		}
	}
	if e := validateConfig(next); e != nil {
		return e
	}
	if e := a.store.Save(next); e != nil {
		a.storageErr = true
		return errors.New("mapping could not be saved")
	}
	a.pruneReceptionLocked(&next)
	a.config = next
	a.finishLearningLocked("saved", "Signal and validated decoder saved; action remains disabled")
	a.learningNotice = nil
	return nil
}

func (a *App) Receive(raw []byte) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || len(raw) > 65536 || !json.Valid(raw) {
		return
	}
	a.expireLocked()
	var envelope struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal(raw, &envelope)
	a.recordCaptureEventLocked(raw)
	report := a.recordDecodedLocked(raw)
	if a.receiver.State == "connected" {
		a.lastDecodedAt = a.now().UTC().Format(time.RFC3339Nano)
	}
	original := append([]byte(nil), raw...)
	var normalizeErr error
	raw, normalizeErr = receiver.NormalizeConfiguredFlex(raw, a.config.Settings.FlexDecoder, a.learning != nil && a.learning.Automatic)
	if normalizeErr != nil {
		a.recordDecisionLocked(report, "configured_decoder_normalization_rejected", "", false)
		return
	}
	for _, profile := range a.config.DecoderProfiles {
		if envelope.Model == profile.Model {
			if a.learning != nil && a.learning.Automatic {
				if receiver.ValidateLearningFrame(original, profile.Bits) != nil {
					return
				}
			}
			normalized, e := receiver.NormalizeProfile(raw, profile)
			if e != nil {
				a.recordDecisionLocked(report, "profile_normalization_rejected", "", false)
				return
			}
			raw = normalized
			break
		}
	}
	now := a.now()
	levels := receiver.ParseSignalLevels(original)
	a.eventLocked("rf", "Decoded RF transmission", "", append(json.RawMessage{}, raw...), levels)
	if l := a.learning; l != nil {
		if l.Automatic {
			a.receiveDecodedLearningLocked(l, raw)
			return
		}
		sig, e := Signature(raw, l.Fields)
		if e != nil {
			a.eventLocked("learning", e.Error(), l.ButtonID, nil)
			return
		}
		if l.Candidate != "" && sig != l.Candidate {
			a.eventLocked("learning", "Different signal ignored; press the selected physical button", l.ButtonID, nil)
			return
		}
		quiet := l.lastRF.IsZero() || now.Sub(l.lastRF) >= 750*time.Millisecond
		l.lastRF = now
		if l.Stage != "waiting" || now.Before(l.armedAt) || !quiet {
			return
		}
		l.Candidate = sig
		l.Raw = append(json.RawMessage{}, raw...)
		l.Presses++
		l.Stage = "captured"
		l.Message = "Press verified. Release the button, then select Ready for next press."
		if l.Presses >= l.Required {
			l.Stage = "ready"
			l.Message = "Three separate presses matched. Confirm this is the intended button and save. Its action will remain disabled."
		}
		return
	}
	if a.captureSuppressed {
		a.recordDecisionLocked(report, "actions_paused", "", false)
		return
	}
	matches, ambiguous := a.matchReceivedButtonsLocked(raw)
	if ambiguous {
		a.recordDecisionLocked(report, "ambiguous_mapping", "", false)
		a.eventLocked("rf_ambiguous", "Signal matches more than one saved button; no action sent. Clear the duplicate mapping before using this signal.", "", nil)
		return
	}
	if len(matches) == 0 {
		a.recordDecisionLocked(report, "no_saved_mapping", "", false)
	}
	for _, match := range matches {
		b := match.button
		diagnostic := a.buttonReceptionLocked(b.ID)
		diagnostic.Reports++
		diagnostic.LastMatchedAt = now.UTC().Format(time.RFC3339Nano)
		diagnostic.Levels = levels
		prior := a.lastSeen[b.ID]
		a.lastSeen[b.ID] = now
		b.LastReceived = now.UTC().Format(time.RFC3339Nano)
		if !prior.IsZero() && now.Sub(prior) < time.Duration(b.DebounceMS)*time.Millisecond {
			diagnostic.Repeats++
			a.recordDecisionLocked(report, "debounce_repeat", b.ID, match.equivalent)
			continue
		}
		diagnostic.Presses++
		a.recordDecisionLocked(report, "recognized", b.ID, match.equivalent)
		if e := a.store.Save(a.config); e != nil {
			a.storageErr = true
		}
		message := "Physical button received"
		if match.equivalent {
			message += " (verified decoder equivalent)"
		}
		a.eventLocked("matched", message, b.ID, nil)
		if !b.Enabled || b.Webhook.URL == "" || now.Before(a.muteUntil) || a.storageErr {
			reason := "action_disabled"
			switch {
			case a.storageErr:
				reason = "storage_error"
			case now.Before(a.muteUntil):
				reason = "action_muted"
			case b.Webhook.URL == "":
				reason = "no_action_configured"
			}
			a.recordDecisionLocked(report, reason, b.ID, match.equivalent)
			continue
		}
		id, epoch := b.ID, a.epoch
		if e := a.scheduleLocked(id, epoch, false); e != nil {
			a.recordDecisionLocked(report, "action_busy", b.ID, match.equivalent)
			a.eventLocked("webhook_busy", e.Error(), id, nil)
		} else {
			a.recordDecisionLocked(report, "action_scheduled", b.ID, match.equivalent)
		}
	}
}
func (a *App) TestButton(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.expireLocked()
	if a.closed {
		return errors.New("service stopping")
	}
	if a.learning != nil || a.captureSuppressed {
		return errors.New("webhook tests are paused during learning")
	}
	if a.demo {
		return errors.New("webhook execution is disabled in demo mode")
	}
	if a.storageErr {
		return errors.New("actions paused because storage failed")
	}
	b := findButton(&a.config, id)
	if b == nil || b.Webhook.URL == "" {
		return errors.New("save a webhook URL before testing")
	}
	if a.now().Sub(a.lastTest[id]) < time.Second {
		return errors.New("wait one second before testing again")
	}
	a.lastTest[id] = a.now()
	epoch := a.epoch
	return a.scheduleLocked(id, epoch, true)
}
