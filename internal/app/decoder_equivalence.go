package app

import (
	"encoding/hex"
	"encoding/json"
	"math/bits"
)

// rtl_433 23.11 can report the same 24 payload bits through these two decoders.
// Akhan reports a 20-bit ID plus a four-bit command; GS558 reverses the 24 bits.
// This is an exact payload transformation, never a timing or bit-error guess.
// Reference: upstream src/devices/{akhan_100F14,smoke_gs558}.c at tag 23.11.
// GS558 JSON does not retain its original row length, so this is equivalence of
// the reported payload, not proof of transmitter model or complete RF framing.
// Keep the original event and saved signature. Only an existing complete Akhan
// mapping can use the GS558 fallback; other decoder families remain exact-only.
var akhanCommands = map[uint32]string{
	1: "0x1 (Lock)", 2: "0x2 (Unlock)", 4: "0x4 (Mute)", 8: "0x8 (Alarm)",
}

func decoderUint(raw json.RawMessage) (uint32, bool) {
	var value *uint32
	if json.Unmarshal(raw, &value) != nil || value == nil {
		return 0, false
	}
	return *value, true
}

func decodedRemotePair(raw []byte) (native, alternate []byte, fromAlternate bool) {
	var event map[string]json.RawMessage
	if json.Unmarshal(raw, &event) != nil {
		return nil, nil, false
	}
	var model string
	if json.Unmarshal(event["model"], &model) != nil {
		return nil, nil, false
	}
	var payload uint32
	switch model {
	case "Akhan-100F14":
		for key := range event {
			if key != "id" && key != "data" && key != "command" && key != "cmd" && key != "action" && !volatile[key] {
				return nil, nil, false
			}
		}
		id, valid := decoderUint(event["id"])
		if !valid || id > 0xfffff {
			return nil, nil, false
		}
		var command string
		value := event["command"]
		if len(value) == 0 {
			value = event["data"]
		}
		if json.Unmarshal(value, &command) != nil {
			return nil, nil, false
		}
		// Do not choose between conflicting aliases in malformed/changed output.
		for _, key := range []string{"data", "cmd", "action"} {
			if value, exists := event[key]; exists {
				var alias string
				if json.Unmarshal(value, &alias) != nil || alias != command {
					return nil, nil, false
				}
			}
		}
		for code, name := range akhanCommands {
			if name == command {
				payload = id<<4 | code
			}
		}
	case "Smoke-GS558":
		for key := range event {
			if key != "id" && key != "unit" && key != "learn" && key != "code" && !volatile[key] {
				return nil, nil, false
			}
		}
		var code string
		id, validID := decoderUint(event["id"])
		unit, validUnit := decoderUint(event["unit"])
		learn, validLearn := decoderUint(event["learn"])
		if json.Unmarshal(event["code"], &code) != nil || len(code) != 6 ||
			!validID || !validUnit || !validLearn || learn != 0 {
			return nil, nil, false
		}
		data, err := hex.DecodeString(code)
		if err != nil {
			return nil, nil, false
		}
		encoded := uint32(data[0])<<16 | uint32(data[1])<<8 | uint32(data[2])
		if id == 0 || id == 0x7fff || id != (encoded>>5)&0x7fff || unit != encoded&31 {
			return nil, nil, false
		}
		payload = bits.Reverse32(encoded) >> 8
		fromAlternate = true
	default:
		return nil, nil, false
	}
	command, supported := akhanCommands[payload&15]
	if !supported {
		return nil, nil, false
	}
	encoded := bits.Reverse32(payload) >> 8
	code := hex.EncodeToString([]byte{byte(encoded >> 16), byte(encoded >> 8), byte(encoded)})
	native, _ = json.Marshal(map[string]any{"model": "Akhan-100F14", "id": payload >> 4, "command": command})
	alternate, _ = json.Marshal(map[string]any{"model": "Smoke-GS558", "id": (encoded >> 5) & 0x7fff, "unit": encoded & 31, "learn": 0, "code": code})
	return native, alternate, fromAlternate
}

func matchesButton(raw []byte, b *Button) bool {
	if len(raw) == 0 || b.Signature == "" {
		return false
	}
	signature, err := Signature(raw, b.IdentityFields)
	return err == nil && signature == b.Signature
}

type receivedButton struct {
	button     *Button
	equivalent bool
}

// Called under a.mu, after the learning/suppression gates. Both representations
// share the existing button's sliding debounce and dispatch gates. A legacy
// config may contain the two representations on different buttons; leave that
// config readable but block this ambiguous payload, even if one action is off.
func (a *App) matchReceivedButtonsLocked(raw []byte) ([]receivedButton, bool) {
	native, alternate, fromAlternate := decodedRemotePair(raw)
	var matches []receivedButton
	related := 0
	for ri := range a.config.Remotes {
		for bi := range a.config.Remotes[ri].Buttons {
			b := &a.config.Remotes[ri].Buttons[bi]
			direct := matchesButton(raw, b)
			nativeMatch := matchesButton(native, b)
			alternateMatch := matchesButton(alternate, b)
			if nativeMatch || alternateMatch {
				related++
			}
			if direct || (fromAlternate && nativeMatch) {
				matches = append(matches, receivedButton{b, !direct})
			}
		}
	}
	return matches, related > 1 || len(matches) > 1
}

// Prevent newly learned collisions without rejecting or rewriting previously
// valid schema-2 configurations at startup. Runtime matching also guards old or
// restored collisions. Model-specific identities with extra required fields do
// not lose those fields to obtain an equivalent match.
func decoderMappingsOverlap(one, two string, oneFields, twoFields []string) bool {
	for _, pair := range []struct {
		native, other string
		fields        []string
	}{{one, two, twoFields}, {two, one, oneFields}} {
		native, alternate, fromAlternate := decodedRemotePair([]byte(pair.native))
		if native == nil || fromAlternate {
			continue
		}
		b := Button{Signature: pair.other, IdentityFields: pair.fields}
		if matchesButton(alternate, &b) {
			return true
		}
	}
	return false
}
