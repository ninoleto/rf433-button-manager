package app

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/ninoleto/rf-button-manager/internal/receiver"
)

// One bounded, in-memory attempt. Never stored in SQLite or included in state
// polling. Export deliberately has no Config, Settings, token, or webhook fields.
type RFCapture struct {
	Format      string            `json:"format"`
	Version     string            `json:"version"`
	StartedAt   string            `json:"started_at"`
	FrequencyHz int               `json:"frequency_hz"`
	SampleRate  int               `json:"sample_rate"`
	Bytes       int               `json:"captured_bytes"`
	Limited     bool              `json:"limit_reached"`
	Packets     []RFPacket        `json:"packets"`
	Events      []json.RawMessage `json:"decoded_events"`
	generation  uint64
	started     time.Time
}

type RFPacket struct {
	OffsetMS   int64  `json:"offset_ms"`
	Modulation string `json:"modulation"`
	Pulses     int    `json:"pulses"`
	Cleanup    bool   `json:"cleanup"`
	Data       string `json:"pulse_data"`
}

func (a *App) recordCapturePacketLocked(generation uint64, packet receiver.PulsePacket) {
	c := a.rfCapture
	if c == nil || c.generation != generation || packet.Progress || packet.ErrorCode != "" || packet.Error != "" || len(packet.Data) == 0 {
		return
	}
	if len(c.Packets) >= 128 || c.Bytes+len(packet.Data) > receiver.MaxCaptureStreamBytes {
		c.Limited = true
		return
	}
	modulation := "FSK"
	if packet.OOK {
		modulation = "OOK"
	}
	c.Packets = append(c.Packets, RFPacket{OffsetMS: a.now().Sub(c.started).Milliseconds(), Modulation: modulation, Pulses: packet.Pulses, Cleanup: packet.Cleanup || a.learning == nil, Data: string(packet.Data)})
	c.Bytes += len(packet.Data)
}

func (a *App) recordCaptureEventLocked(raw []byte) {
	c := a.rfCapture
	if c == nil || a.learning == nil || !a.learning.Automatic || a.learning.CaptureGeneration != c.generation || !json.Valid(raw) {
		return
	}
	if len(c.Events) >= 128 || len(raw) > 8192 || c.Bytes+len(raw) > receiver.MaxCaptureStreamBytes {
		c.Limited = true
		return
	}
	c.Events = append(c.Events, append(json.RawMessage(nil), raw...))
	c.Bytes += len(raw)
}

func (a *App) ExportRFCapture() (*RFCapture, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.rfCapture == nil || a.rfCapture.Bytes == 0 {
		return nil, errors.New("no retained RF data; start Learn and press the intended button during the waiting window")
	}
	return clone(a.rfCapture), nil
}
