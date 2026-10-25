package receiver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// DecoderProfile is persisted only after the application verifies three distinct
// armed captures. Timings come from rtl_433 analysis, never a built-in guess.
type DecoderProfile struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Model            string `json:"model"`
	FlexDecoder      string `json:"flex_decoder"`
	Bits             int    `json:"bits"`
	ValidatedPresses int    `json:"validated_presses"`
	CreatedAt        string `json:"created_at"`
}

var suggestionPattern = regexp.MustCompile(`Use a flex decoder with -X '([^'\r\n]+)'`)
var codePattern = regexp.MustCompile(`^\{([0-9]{1,3})\}([0-9a-fA-F]+)$`)

// canonicalTiming accepts only the PWM numeric vocabulary printed by rtl_433.
func canonicalTiming(spec string) (string, error) {
	values := map[string]int{}
	modulation := ""
	for _, part := range strings.Split(spec, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			return "", errors.New("invalid analyzer candidate")
		}
		switch key {
		case "n":
		case "m":
			modulation = value
		case "s", "l", "r", "g", "t", "y":
			if _, exists := values[key]; exists {
				return "", errors.New("duplicate candidate parameter")
			}
			n, err := strconv.Atoi(value)
			if err != nil || n < 0 || n > 1000000 {
				return "", errors.New("candidate timing outside supported bounds")
			}
			values[key] = n
		default:
			return "", errors.New("unsupported analyzer candidate parameter")
		}
	}
	if modulation != "OOK_PWM" || values["s"] < 1 || values["l"] <= values["s"] || values["r"] <= values["l"] {
		return "", errors.New("only measured OOK PWM candidates are supported")
	}
	return fmt.Sprintf("m=OOK_PWM,s=%d,l=%d,r=%d,g=%d,t=%d,y=%d", values["s"], values["l"], values["r"], values["g"], values["t"], values["y"]), nil
}

func profileFromTiming(timing string, bits int) (DecoderProfile, error) {
	canonical, err := canonicalTiming(timing)
	if err != nil {
		return DecoderProfile{}, err
	}
	if bits < 16 || bits > 64 {
		return DecoderProfile{}, errors.New("automatic PWM learning supports 16–64 bit fixed codes")
	}
	key := fmt.Sprintf("%s,bits=%d,repeats>=2", canonical, bits)
	hash := sha256.Sum256([]byte(key))
	id := hex.EncodeToString(hash[:16])
	model := "RFB-PWM-" + id
	return DecoderProfile{ID: id, Name: fmt.Sprintf("Fixed-code OOK PWM (%d bits)", bits), Model: model, FlexDecoder: "n=" + model + "," + key, Bits: bits}, nil
}

func (p DecoderProfile) Validate(persisted bool) error {
	if len(p.Name) == 0 || len(p.Name) > 100 || len(p.FlexDecoder) > 512 {
		return errors.New("invalid learned decoder profile")
	}
	suffix := fmt.Sprintf(",bits=%d,repeats>=2", p.Bits)
	if !strings.HasSuffix(p.FlexDecoder, suffix) {
		return errors.New("profile framing is not bounded")
	}
	candidate := strings.TrimSuffix(p.FlexDecoder, suffix)
	expected, err := profileFromTiming(candidate, p.Bits)
	if err != nil || expected.ID != p.ID || expected.Model != p.Model || expected.FlexDecoder != p.FlexDecoder {
		return errors.New("profile does not match its measured timing/framing identity")
	}
	if persisted {
		if p.ValidatedPresses != 3 {
			return errors.New("decoder profile requires three verified captures")
		}
		if _, err := time.Parse(time.RFC3339, p.CreatedAt); err != nil {
			return errors.New("invalid decoder validation timestamp")
		}
	}
	return nil
}

// repeatedCode never chooses the first of multiple distinct same-length codes.
// Short delimiter rows are excluded only after the profile's bit length is fixed.
func repeatedCode(raw []byte, bits int) (string, int, error) {
	var event struct {
		Codes []string `json:"codes"`
	}
	if json.Unmarshal(raw, &event) != nil || len(event.Codes) > 4096 {
		return "", 0, errors.New("invalid decoder output")
	}
	counts := map[string]int{}
	for _, value := range event.Codes {
		match := codePattern.FindStringSubmatch(strings.TrimSpace(value))
		if match == nil {
			continue
		}
		n, _ := strconv.Atoi(match[1])
		if n < 16 || n > 64 || (bits != 0 && n != bits) {
			continue
		}
		if len(match[2]) != (n+3)/4 {
			return "", 0, errors.New("invalid fixed-code length")
		}
		code := fmt.Sprintf("{%d}%s", n, strings.ToLower(match[2]))
		counts[code]++
	}
	if len(counts) != 1 {
		return "", 0, errors.New("capture has no unambiguous repeated fixed code")
	}
	for code, count := range counts {
		if count < 2 {
			return "", 0, errors.New("capture needs at least two repeated RF frames")
		}
		match := codePattern.FindStringSubmatch(code)
		n, _ := strconv.Atoi(match[1])
		return code, n, nil
	}
	return "", 0, errors.New("no repeated code")
}

func NormalizeProfile(raw []byte, profile DecoderProfile) ([]byte, error) {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return nil, errors.New("invalid RF JSON")
	}
	var model string
	_ = json.Unmarshal(obj["model"], &model)
	if model != profile.Model {
		return nil, errors.New("different decoder model")
	}
	code, _, err := repeatedCode(raw, profile.Bits)
	if err != nil {
		return nil, err
	}
	// Keep only the exact validated identity. No analyzer output reaches this path.
	return json.Marshal(map[string]any{"model": model, "codes": []string{code}})
}
