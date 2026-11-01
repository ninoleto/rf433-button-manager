package app

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/bits"
	"strconv"
	"strings"

	"github.com/ninoleto/rf433-button-manager/internal/receiver"
)

// smokePayload validates the complete live decoder output, including redundant
// identity fields and pairing status. Unlike the Akhan decoder, this format can
// carry all four-bit command values. It does not identify the transmitter model.
func smokePayload(event map[string]json.RawMessage) (uint32, bool) {
	for key := range event {
		if key != "id" && key != "unit" && key != "learn" && key != "code" && !volatile[key] {
			return 0, false
		}
	}
	var code string
	id, validID := decoderUint(event["id"])
	unit, validUnit := decoderUint(event["unit"])
	learn, validLearn := decoderUint(event["learn"])
	if json.Unmarshal(event["code"], &code) != nil || len(code) != 6 ||
		!validID || !validUnit || !validLearn || learn != 0 {
		return 0, false
	}
	data, err := hex.DecodeString(code)
	if err != nil {
		return 0, false
	}
	encoded := uint32(data[0])<<16 | uint32(data[1])<<8 | uint32(data[2])
	if id == 0 || id == 0x7fff || id != (encoded>>5)&0x7fff || unit != encoded&31 {
		return 0, false
	}
	return bits.Reverse32(encoded) >> 8, true
}

func boundedPWM25(profile receiver.DecoderProfile) bool {
	return profile.Bits == 25 && profile.Validate(true) == nil &&
		strings.HasPrefix(profile.FlexDecoder, "n="+profile.Model+",m=OOK_PWM,")
}

// profileEquivalentEvents connects only registered, three-press-validated PWM
// profiles with the exact 24-bit payload represented by GS558/Akhan output.
// Upstream rtl_433 23.11 pulse_slicer_pwm emits short=1 and long=0. For these
// bounded reports the complete 25-bit word is inverted payload + trailing 1;
// its three hex padding bits must be zero. GS558 reverses the inverted 24 bits.
// No bit errors, truncated codes, guessed timings or arbitrary RFB-* names pass.
// GS558 JSON loses the original RF row length: this is equality of reported
// payloads, not transmitter authentication or proof of the full air protocol.
func profileEquivalentEvents(raw []byte, profiles []receiver.DecoderProfile) [][]byte {
	var event map[string]json.RawMessage
	if json.Unmarshal(raw, &event) != nil {
		return nil
	}
	var model string
	if json.Unmarshal(event["model"], &model) != nil {
		return nil
	}
	validProfiles := make([]receiver.DecoderProfile, 0, len(profiles))
	for _, profile := range profiles {
		if boundedPWM25(profile) {
			validProfiles = append(validProfiles, profile)
		}
	}
	if len(validProfiles) == 0 {
		return nil
	}
	var payload uint32
	var valid bool
	fromProfile := false
	switch model {
	case "Smoke-GS558":
		payload, valid = smokePayload(event)
	case "Akhan-100F14":
		_, alternate, _ := decodedRemotePair(raw)
		var smoke map[string]json.RawMessage
		if json.Unmarshal(alternate, &smoke) == nil {
			payload, valid = smokePayload(smoke)
		}
	default:
		for _, profile := range validProfiles {
			if profile.Model == model {
				fromProfile = true
				break
			}
		}
		// Called after NormalizeProfile (or on a saved canonical signature).
		// Do not strip additional identity fields to create an equivalence.
		if !fromProfile || len(event) != 2 {
			return nil
		}
		var codes []string
		if json.Unmarshal(event["codes"], &codes) != nil || len(codes) != 1 ||
			len(codes[0]) != 11 || !strings.HasPrefix(codes[0], "{25}") {
			return nil
		}
		frame, err := strconv.ParseUint(codes[0][4:], 16, 28)
		if err != nil || frame&15 != 8 {
			return nil
		}
		payload, valid = uint32(frame>>4)^0xffffff, true
	}
	if !valid {
		return nil
	}
	var out [][]byte
	if fromProfile {
		encoded := bits.Reverse32(payload) >> 8
		id, unit := (encoded>>5)&0x7fff, encoded&31
		if id != 0 && id != 0x7fff {
			smoke, _ := json.Marshal(map[string]any{"model": "Smoke-GS558", "id": id, "unit": unit,
				"learn": 0, "code": fmt.Sprintf("%06x", encoded)})
			out = append(out, smoke)
			if native, _, _ := decodedRemotePair(smoke); native != nil {
				out = append(out, native)
			}
		}
	}
	for _, profile := range validProfiles {
		frame := ((payload ^ 0xffffff) << 4) | 8
		equivalent, _ := json.Marshal(map[string]any{"model": profile.Model, "codes": []string{fmt.Sprintf("{25}%07x", frame)}})
		out = append(out, equivalent)
	}
	return out
}

func profileMappingsOverlap(one, two string, oneFields, twoFields []string, profiles []receiver.DecoderProfile) bool {
	for _, pair := range []struct {
		source, target string
		fields         []string
	}{
		{one, two, twoFields}, {two, one, oneFields},
	} {
		b := Button{Signature: pair.target, IdentityFields: pair.fields}
		for _, event := range profileEquivalentEvents([]byte(pair.source), profiles) {
			if matchesButton(event, &b) {
				return true
			}
		}
	}
	return false
}
