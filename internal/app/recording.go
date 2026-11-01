package app

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"time"

	"github.com/ninoleto/rf433-button-manager/internal/receiver"
)

const recordingDuration = 60 * time.Second
const recordingPacketLimit = 512
const recordingEventLimit = 2048

// RecordingStatus is the only recording data returned by ordinary state polling.
type RecordingStatus struct {
	ID               string `json:"id"`
	ButtonID         string `json:"button_id"`
	ButtonName       string `json:"button_name"`
	Active           bool   `json:"active"`
	StartedAt        string `json:"started_at"`
	ExpiresAt        string `json:"expires_at"`
	FinishedAt       string `json:"finished_at,omitempty"`
	StopReason       string `json:"stop_reason,omitempty"`
	SecondsRemaining int    `json:"seconds_remaining"`
	Bytes            int    `json:"captured_bytes"`
	PacketCount      int    `json:"packet_count"`
	DecodedCount     int    `json:"decoded_count"`
	Presses          int    `json:"recognized_presses"`
	Repeats          int    `json:"repeat_reports"`
}

type RecordingTarget struct {
	Signature        string   `json:"signature"`
	IdentityFields   []string `json:"identity_fields"`
	DebounceMS       int      `json:"debounce_ms"`
	Enabled          bool     `json:"enabled"`
	ActionConfigured bool     `json:"action_configured"`
}

// Decision report_id links to an unchanged decoded record. Action events are
// timestamped by button, not falsely attributed to a particular RF report: a
// request may already be in flight or may have been started by a manual test.
type RecordingEvent struct {
	OffsetMS   int64           `json:"offset_ms"`
	Kind       string          `json:"kind"`
	ReportID   uint64          `json:"report_id,omitempty"`
	ButtonID   string          `json:"button_id,omitempty"`
	Reason     string          `json:"reason,omitempty"`
	Equivalent bool            `json:"decoder_equivalent,omitempty"`
	HTTPStatus int             `json:"http_status,omitempty"`
	Data       json.RawMessage `json:"decoded_data,omitempty"`
}

// Explicit RF-only export. Never embed Config, Settings, Button, Event or
// Webhook: these can contain credentials, destinations, bodies or headers.
type ReceptionRecording struct {
	RecordingStatus
	Format              string                    `json:"format"`
	Version             string                    `json:"version"`
	Demo                bool                      `json:"demo"`
	FrequencyHz         int                       `json:"frequency_hz"`
	SampleRate          int                       `json:"sample_rate"`
	AdaptiveSensitivity bool                      `json:"adaptive_sensitivity"`
	Generation          uint64                    `json:"receiver_generation"`
	Target              RecordingTarget           `json:"target"`
	Protocols           []int                     `json:"protocols"`
	FlexDecoder         string                    `json:"flex_decoder"`
	Profiles            []receiver.DecoderProfile `json:"decoder_profiles"`
	ReceiverStart       receiver.Status           `json:"receiver_start"`
	ReceiverEnd         receiver.Status           `json:"receiver_end"`
	StreamStart         receiver.PulseStream      `json:"stream_start"`
	StreamEnd           receiver.PulseStream      `json:"stream_end"`
	Packets             []RFPacket                `json:"packets"`
	Events              []RecordingEvent          `json:"events"`
	Limitations         string                    `json:"limitations"`
	started, expires    time.Time
	nextReport          uint64
}

func (a *App) StartRecording(id string) (*RecordingStatus, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.expireLocked()
	if a.closed || a.learning != nil || a.captureSuppressed || (!a.demo && a.receiverApplied != a.receiverGeneration) {
		return nil, errors.New("wait for normal reception before recording")
	}
	if !a.demo && (!a.captureEnabled || a.receiver.State != "connected") {
		return nil, errors.New("a connected receiver is required for normal-use recording")
	}
	if a.recording != nil && a.recording.Active {
		return nil, errors.New("a normal-use recording is already running")
	}
	b := findButton(&a.config, id)
	if b == nil || b.Signature == "" {
		return nil, errors.New("choose a button with a saved signal before recording")
	}
	now := a.now()
	a.recording = &ReceptionRecording{
		RecordingStatus: RecordingStatus{ID: newID(), ButtonID: id, ButtonName: b.Name, Active: true,
			StartedAt: now.UTC().Format(time.RFC3339Nano), ExpiresAt: now.Add(recordingDuration).UTC().Format(time.RFC3339Nano)},
		Format: "rf433-button-manager-reception-v1", Version: Version, Demo: a.demo,
		FrequencyHz: 433920000, SampleRate: 250000, AdaptiveSensitivity: a.config.Settings.AdaptiveSensitivity,
		Generation: a.receiverApplied,
		Target:     RecordingTarget{Signature: b.Signature, IdentityFields: clone(b.IdentityFields), DebounceMS: b.DebounceMS, Enabled: b.Enabled, ActionConfigured: b.Webhook.URL != ""},
		Protocols:  clone(a.config.Settings.Protocols), FlexDecoder: a.config.Settings.FlexDecoder, Profiles: clone(a.config.DecoderProfiles),
		ReceiverStart: clone(a.receiver), StreamStart: a.pulseStream,
		Packets: []RFPacket{}, Events: []RecordingEvent{}, started: now, expires: now.Add(recordingDuration),
		Limitations: "Receive-only pulse timings, not raw IQ. Offsets are application receipt times. Packet and decoded-report counts are not physical press counts. A missing decode does not identify its cause. Recordings include surrounding RF traffic. Webhook events are timestamped by button and can include requests started before recording or by a manual test. Results arriving after recording stops are not included. No automatic retries or actions are introduced by recording. Credentials, webhook URLs, headers and bodies are excluded.",
	}
	metadata, _ := json.Marshal(a.recording)
	a.recording.Bytes = len(metadata)
	return a.recordingStatusLocked(), nil
}

func (a *App) recordingStatusLocked() *RecordingStatus {
	if a.recording == nil {
		return nil
	}
	s := a.recording.RecordingStatus
	if s.Active {
		s.SecondsRemaining = max(0, int((a.recording.expires.Sub(a.now())+time.Second-1)/time.Second))
	}
	return &s
}

func (a *App) stopRecordingLocked(reason string) {
	c := a.recording
	if c == nil || !c.Active {
		return
	}
	c.Active = false
	c.StopReason = reason
	c.FinishedAt = a.now().UTC().Format(time.RFC3339Nano)
	c.ReceiverEnd = clone(a.receiver)
	c.StreamEnd = a.pulseStream
}

func (a *App) StopRecording() (*RecordingStatus, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.expireLocked()
	if a.recording == nil {
		return nil, errors.New("no normal-use recording has been started")
	}
	a.stopRecordingLocked("stopped")
	return a.recordingStatusLocked(), nil
}

func (a *App) ExportRecording() (*ReceptionRecording, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.expireLocked()
	if a.recording == nil {
		return nil, errors.New("start a normal-use recording first")
	}
	out := clone(a.recording)
	out.RecordingStatus = *a.recordingStatusLocked()
	if out.Active {
		out.ReceiverEnd = clone(a.receiver)
		out.StreamEnd = a.pulseStream
	}
	return out, nil
}

func (a *App) recordDiagnosticLocked(e RecordingEvent) bool {
	c := a.recording
	if c == nil || !c.Active {
		return false
	}
	if !a.now().Before(c.expires) {
		a.stopRecordingLocked("time_limit")
		return false
	}
	e.OffsetMS = a.now().Sub(c.started).Milliseconds()
	encoded, err := json.Marshal(e)
	if err != nil || len(c.Events) >= recordingEventLimit || c.Bytes+len(encoded) > receiver.MaxCaptureStreamBytes {
		a.stopRecordingLocked("data_limit")
		return false
	}
	c.Events = append(c.Events, e)
	c.Bytes += len(encoded)
	return true
}

func (a *App) recordDecodedLocked(raw []byte) uint64 {
	c := a.recording
	if c == nil || !c.Active {
		return 0
	}
	if len(raw) > 8192 {
		a.stopRecordingLocked("record_too_large")
		return 0
	}
	c.nextReport++
	if !a.recordDiagnosticLocked(RecordingEvent{Kind: "decoded", ReportID: c.nextReport, Data: append(json.RawMessage(nil), raw...)}) {
		return 0
	}
	c.DecodedCount++
	return c.nextReport
}

func (a *App) recordDecisionLocked(report uint64, reason, button string, equivalent bool) {
	if report == 0 || !a.recordDiagnosticLocked(RecordingEvent{Kind: "decision", ReportID: report, Reason: reason, ButtonID: button, Equivalent: equivalent}) {
		return
	}
	if button == a.recording.ButtonID {
		switch reason {
		case "recognized":
			a.recording.Presses++
		case "debounce_repeat":
			a.recording.Repeats++
		}
	}
}

var recordedHTTPStatus = regexp.MustCompile(`\bHTTP ([0-9]{3})\b`)

func (a *App) recordActionEventLocked(kind, message, button string) {
	if a.recording == nil || !a.recording.Active {
		return
	}
	switch kind {
	case "webhook_start", "webhook_ok", "webhook_error", "webhook_unknown", "webhook_busy":
		code := 0
		if match := recordedHTTPStatus.FindStringSubmatch(message); len(match) == 2 {
			code, _ = strconv.Atoi(match[1])
		}
		// Never copy free-form messages, errors, response bodies or destinations.
		a.recordDiagnosticLocked(RecordingEvent{Kind: kind, ButtonID: button, HTTPStatus: code})
	}
}

func (a *App) recordNormalPulseLocked(generation uint64, p receiver.PulsePacket) {
	c := a.recording
	if c == nil || !c.Active || c.Generation != generation {
		return
	}
	if p.Final || p.Cleanup {
		a.stopRecordingLocked("receiver_stream_closed")
		return
	}
	if p.Progress {
		return
	}
	if p.ErrorCode != "" || p.Error != "" {
		a.recordDiagnosticLocked(RecordingEvent{Kind: "pulse_rejected", Reason: p.ErrorCode})
		return
	}
	// Exclude a packet whose header had already arrived before Start recording.
	if p.Stream.Headers <= c.StreamStart.Headers || len(p.Data) == 0 {
		return
	}
	packet := RFPacket{OffsetMS: a.now().Sub(c.started).Milliseconds(), Modulation: "FSK", Pulses: p.Pulses, Data: string(p.Data)}
	if p.OOK {
		packet.Modulation = "OOK"
	}
	encoded, _ := json.Marshal(packet)
	if len(c.Packets) >= recordingPacketLimit || c.Bytes+len(encoded) > receiver.MaxCaptureStreamBytes {
		a.stopRecordingLocked("data_limit")
		return
	}
	c.Packets = append(c.Packets, packet)
	c.PacketCount++
	c.Bytes += len(encoded)
}
