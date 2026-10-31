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
	// A learned flex profile must not win merely because rtl_433 emits it
	// before a built-in decoder for the same burst. Keep a bounded first-press
	// comparison window. Existing mappings and normal reception are untouched.
	var envelope struct {
		Model string `json:"model"`
	}
	_ = json.Unmarshal(raw, &envelope)
	custom := strings.HasPrefix(envelope.Model, "RFB-")
	for _, part := range strings.Split(a.config.Settings.FlexDecoder, ",") {
		if part == "n="+envelope.Model {
			custom = true
		}
	}
	if l.Candidate == "" && custom && !now.Before(l.armedAt) && (l.Stage == "waiting" || l.Stage == "receiving" || l.Stage == "analyzing") {
		if len(l.pendingDecoded) == 0 {
			l.pendingDecoded = append([]byte(nil), raw...)
			l.pendingDecodedAt = now
		} else if previous, _ := Signature(l.pendingDecoded, l.Fields); previous != sig {
			a.finishLearningLocked("inconsistent", "Multiple learned signal identities appeared in one press. Keep other remotes idle and retry; no mapping changed.")
			return
		}
		l.lastDecoded = now
		l.decodedSource = true
		l.Stage = "receiving"
		l.Message = "Comparing the learned profile with rtl_433 decoders. Release the button."
		if l.analysisCancel != nil {
			l.analysisCancel()
		}
		return
	}
	quiet := l.lastDecoded.IsZero() || now.Sub(l.lastDecoded) >= time.Second
	if len(l.pendingDecoded) > 0 && !custom {
		quiet = true
	}
	l.lastDecoded = now
	if (l.Stage != "waiting" && l.Stage != "receiving" && l.Stage != "analyzing") || now.Before(l.armedAt) || !quiet {
		return
	}
	a.acceptDecodedLocked(l, raw, sig)
}

func (a *App) acceptDecodedLocked(l *Learning, raw []byte, sig string) {
	if l.analysisCancel != nil {
		l.analysisCancel() // A decoded button takes priority over background replay.
	}
	l.decodedSource = true
	l.pendingDecoded = nil
	l.Candidate = sig
	l.Raw = append(json.RawMessage(nil), raw...)
	l.packets = nil
	l.bytes = 0
	l.firstPulse = time.Time{}
	l.Diagnostics.Source = "rtl_433 decoded button"
	l.Profile = nil
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
