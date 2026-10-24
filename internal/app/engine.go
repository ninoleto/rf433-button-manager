package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ninoleto/rf-button-manager/internal/receiver"
	"sync"
	"time"
)

const Version = "0.1.0-dev"

type App struct {
	mu              sync.Mutex
	actionGate      sync.RWMutex
	store           *Store
	config          Config
	receiver        receiver.Status
	learning        *Learning
	events          []Event
	inFlight        map[string]bool
	actionSlots     chan struct{}
	lastSeen        map[string]time.Time
	lastTest        map[string]time.Time
	muteUntil       time.Time
	epoch           uint64
	storageErr      bool
	demo            bool
	closed          bool
	now             func() time.Time
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	settingsChanged chan struct{}
}

func New(store *Store, c Config, demo bool) (*App, error) {
	events, e := store.Events(150)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &App{store: store, config: c, inFlight: map[string]bool{}, actionSlots: make(chan struct{}, 8), events: events, lastSeen: map[string]time.Time{}, lastTest: map[string]time.Time{}, muteUntil: time.Now().Add(2 * time.Second), demo: demo, now: time.Now, ctx: ctx, cancel: cancel, settingsChanged: make(chan struct{}, 1), receiver: receiver.Status{State: "disconnected", Message: "Receiver has not started"}}, nil
}
func (a *App) Close()                           { a.mu.Lock(); a.closed = true; a.cancel(); a.mu.Unlock(); a.wg.Wait() }
func (a *App) SettingsChanges() <-chan struct{} { return a.settingsChanged }
func (a *App) Settings() Settings               { a.mu.Lock(); defer a.mu.Unlock(); return clone(a.config.Settings) }
func (a *App) eventLocked(kind, message, button string, data json.RawMessage) bool {
	if len(data) > 8192 {
		data = nil
		message += " (large raw payload omitted from history)"
	}
	ev := Event{Time: a.now().UTC().Format(time.RFC3339Nano), Kind: kind, Message: message, ButtonID: button, Data: data}
	saved, e := a.store.Record(ev)
	if e != nil {
		a.storageErr = true
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
	if a.closed {
		return
	}
	if a.receiver.State != s.State || a.receiver.Message != s.Message {
		a.eventLocked("receiver", s.State+": "+s.Message, "", nil)
	}
	a.receiver = s
}
func (a *App) expireLocked() {
	if a.learning != nil && !a.now().Before(a.learning.expires) {
		id := a.learning.ButtonID
		a.learning = nil
		a.epoch++
		a.muteUntil = a.now().Add(time.Second)
		a.eventLocked("learning", "Learning expired; no mapping changed", id, nil)
	}
}
func (a *App) State() any {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.expireLocked()
	return clone(struct {
		Remotes      []Remote        `json:"remotes"`
		Settings     Settings        `json:"settings"`
		Receiver     receiver.Status `json:"receiver"`
		Learning     *Learning       `json:"learning"`
		Events       []Event         `json:"events"`
		Version      string          `json:"version"`
		Demo         bool            `json:"demo"`
		StorageError bool            `json:"storage_error"`
	}{a.config.Remotes, a.config.Settings, a.receiver, a.learning, a.events, Version, a.demo, a.storageErr})
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
	a.mu.Lock()
	defer a.mu.Unlock()
	a.expireLocked()
	if a.learning != nil {
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
	a.config = next
	a.epoch++
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
	var out Button
	e := a.mutate(func(c *Config) error {
		b := findButton(c, id)
		if b == nil {
			return errors.New("button not found")
		}
		next.ID = id
		next.Signature = b.Signature
		next.LastReceived = b.LastReceived
		oldFields, _ := json.Marshal(b.IdentityFields)
		newFields, _ := json.Marshal(next.IdentityFields)
		if string(oldFields) != string(newFields) {
			next.Signature = ""
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
	e := a.mutate(func(c *Config) error { c.Settings = s; return nil })
	if e == nil {
		select {
		case a.settingsChanged <- struct{}{}:
		default:
		}
	}
	return e
}
func (a *App) Backup() Config { a.mu.Lock(); defer a.mu.Unlock(); return clone(a.config) }
func (a *App) Restore(c Config) error {
	if e := validateConfig(c); e != nil {
		return e
	}
	for i := range c.Remotes {
		for j := range c.Remotes[i].Buttons {
			c.Remotes[i].Buttons[j].Enabled = false
			c.Remotes[i].Buttons[j].LastReceived = ""
		}
	}
	e := a.mutate(func(next *Config) error { *next = c; return nil })
	if e == nil {
		select {
		case a.settingsChanged <- struct{}{}:
		default:
		}
	}
	return e
}

func (a *App) StartLearning(id string, fields []string) (*Learning, error) {
	a.actionGate.Lock()
	defer a.actionGate.Unlock()
	a.mu.Lock()
	defer a.mu.Unlock()
	a.expireLocked()
	if a.learning != nil {
		return nil, errors.New("another button is already learning")
	}
	b := findButton(&a.config, id)
	if b == nil {
		return nil, errors.New("button not found")
	}
	if len(fields) == 0 {
		fields = b.IdentityFields
	}
	if e := validateFields(fields); e != nil {
		return nil, e
	}
	now := a.now()
	a.epoch++
	a.learning = &Learning{ButtonID: id, Stage: "waiting", Required: 3, Fields: append([]string{}, fields...), expires: now.Add(90 * time.Second), armedAt: now, ExpiresAt: now.Add(90 * time.Second).UTC().Format(time.RFC3339)}
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
	l.Stage = "waiting"
	l.armedAt = a.now()
	return clone(l), nil
}
func (a *App) CancelLearning() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.learning != nil {
		a.eventLocked("learning", "Learning cancelled; previous mapping kept", a.learning.ButtonID, nil)
	}
	a.learning = nil
	a.epoch++
	a.muteUntil = a.now().Add(time.Second)
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
	if l == nil || l.Stage != "ready" || l.Presses < 3 {
		return errors.New("three separate matching presses are required")
	}
	next := clone(a.config)
	b := findButton(&next, l.ButtonID)
	if b == nil {
		return errors.New("button not found")
	}
	for _, r := range next.Remotes {
		for _, other := range r.Buttons {
			if other.ID != b.ID && other.Signature != "" && signaturesOverlap(other.Signature, l.Candidate) {
				return errors.New("signal overlaps an existing button mapping; clear that mapping first")
			}
		}
	}
	b.Signature = l.Candidate
	b.IdentityFields = append([]string{}, l.Fields...)
	b.Enabled = false
	if e := validateConfig(next); e != nil {
		return e
	}
	if e := a.store.Save(next); e != nil {
		a.storageErr = true
		return errors.New("mapping could not be saved")
	}
	a.config = next
	a.learning = nil
	a.epoch++
	a.muteUntil = a.now().Add(time.Second)
	a.eventLocked("learning", "Signal saved; review webhook and enable button", b.ID, nil)
	return nil
}

func (a *App) Receive(raw []byte) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || len(raw) > 65536 || !json.Valid(raw) {
		return
	}
	a.expireLocked()
	now := a.now()
	a.eventLocked("rf", "Decoded RF transmission", "", append(json.RawMessage{}, raw...))
	if l := a.learning; l != nil {
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
		if l.Presses >= l.Required {
			l.Stage = "ready"
		}
		return
	}
	for ri := range a.config.Remotes {
		for bi := range a.config.Remotes[ri].Buttons {
			b := &a.config.Remotes[ri].Buttons[bi]
			if b.Signature == "" {
				continue
			}
			sig, e := Signature(raw, b.IdentityFields)
			if e != nil || sig != b.Signature {
				continue
			}
			prior := a.lastSeen[b.ID]
			a.lastSeen[b.ID] = now
			b.LastReceived = now.UTC().Format(time.RFC3339Nano)
			if !prior.IsZero() && now.Sub(prior) < time.Duration(b.DebounceMS)*time.Millisecond {
				continue
			}
			if e = a.store.Save(a.config); e != nil {
				a.storageErr = true
			}
			a.eventLocked("matched", "Physical button received", b.ID, nil)
			if !b.Enabled || b.Webhook.URL == "" || now.Before(a.muteUntil) || a.storageErr {
				continue
			}
			id, epoch := b.ID, a.epoch
			if e := a.scheduleLocked(id, epoch, false); e != nil {
				a.eventLocked("webhook_busy", e.Error(), id, nil)
			}
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
	if a.learning != nil {
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
