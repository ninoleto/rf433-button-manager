package app

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// rtl_433's existing remote decoders use several names for the button field.
// Canonicalize aliases in the identity calculation, not in the saved RF event.
// Never use RSSI, time, battery, or a sensor reading as a button identity.
func normalizeButtonFields(m map[string]any) {
	if _, exists := m["command"]; !exists {
		for _, alias := range []string{"cmd", "action", "data"} {
			if value, ok := m[alias]; ok && value != nil {
				m["command"] = value
				break
			}
		}
	}
	if _, exists := m["button"]; exists {
		return
	}
	var pressed []string
	for key, value := range m {
		if !strings.HasPrefix(key, "button") || len(key) <= 6 {
			continue
		}
		suffix := key[6:]
		valid := true
		for _, ch := range suffix {
			if ch < '0' || ch > '9' {
				valid = false
			}
		}
		if !valid {
			continue
		}
		switch v := value.(type) {
		case string:
			if v != "" {
				pressed = append(pressed, key+"="+v)
			}
		case bool:
			if v {
				pressed = append(pressed, key+"=true")
			}
		}
	}
	if len(pressed) > 0 {
		sort.Strings(pressed)
		m["button"] = strings.Join(pressed, ";")
	}
}

// Known decoders already produce button identities. Three separately armed
// presses provide the same learning gate as pulse replay, without rediscovering
// a decoder that rtl_433 already has. Invoked under a.mu by Receive.
func (a *App) receiveDecodedLearningLocked(l *Learning, raw []byte) {
	if l.Stage == "preparing" || (l.Candidate != "" && !l.decodedSource) {
		return
	}
	sig, err := Signature(raw, l.Fields)
	if err != nil || (l.Candidate != "" && sig != l.Candidate) {
		return
	}
	now := a.now()
	quiet := l.lastDecoded.IsZero() || now.Sub(l.lastDecoded) >= time.Second
	l.lastDecoded = now
	if (l.Stage != "waiting" && l.Stage != "receiving" && l.Stage != "analyzing") || now.Before(l.armedAt) || !quiet {
		return
	}
	if l.analysisCancel != nil {
		l.analysisCancel() // A decoded button takes priority over background replay.
	}
	l.decodedSource = true
	l.Candidate = sig
	l.Raw = append(json.RawMessage(nil), raw...)
	l.packets = nil
	l.bytes = 0
	l.firstPulse = time.Time{}
	l.Diagnostics.Source = "rtl_433 decoded button"
	var envelope struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal(raw, &envelope)
	for _, profile := range a.config.DecoderProfiles {
		if profile.Model == envelope.Model {
			p := profile
			l.Profile = &p
			break
		}
	}
	l.Presses++
	l.Stage = "captured"
	l.Message = "Press received. Release the button, then select Ready for next press."
	if l.Presses == l.Required {
		l.Stage = "ready"
		l.Message = "Three separate presses matched. Confirm and save; the action stays disabled."
	}
	a.eventLocked("learning", "Button identity received from rtl_433", l.ButtonID, nil)
}
