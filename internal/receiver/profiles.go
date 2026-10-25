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
// armed captures. Timings come from the recording, never a preset remote guess.
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

// canonicalTiming accepts only the bounded PWM/PPM/NRZ PCM flex vocabulary.
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
	widths := values["l"] > values["s"]
	if modulation == "OOK_PCM" {
		widths = values["l"] == values["s"] && values["g"] == values["r"] && values["y"] == 0 && values["t"] == 0
	}
	if (modulation != "OOK_PWM" && modulation != "OOK_PPM" && modulation != "OOK_PCM") || values["s"] < 1 || !widths || values["r"] <= values["l"] {
		return "", errors.New("only measured OOK PWM/PPM/NRZ PCM candidates are supported")
	}
	return fmt.Sprintf("m=%s,s=%d,l=%d,r=%d,g=%d,t=%d,y=%d", modulation, values["s"], values["l"], values["r"], values["g"], values["t"], values["y"]), nil
}

func profileFromTiming(timing string, bits int) (DecoderProfile, error) {
	return profileWithRepeats(timing, bits, 1)
}

func profileWithRepeats(timing string, bits, repeats int) (DecoderProfile, error) {
	canonical, err := canonicalTiming(timing)
	if err != nil {
		return DecoderProfile{}, err
	}
	if bits < 12 || bits > 64 {
		return DecoderProfile{}, errors.New("automatic fixed-code learning supports 12–64 bit codes")
	}
	key := fmt.Sprintf("%s,bits=%d,repeats>=%d", canonical, bits, repeats)
	hash := sha256.Sum256([]byte(key))
	id := hex.EncodeToString(hash[:16])
	kind := "PWM"
	if strings.HasPrefix(canonical, "m=OOK_PPM,") {
		kind = "PPM"
	} else if strings.HasPrefix(canonical, "m=OOK_PCM,") {
		kind = "PCM"
	}
	model := "RFB-" + kind + "-" + id
	return DecoderProfile{ID: id, Name: fmt.Sprintf("Fixed-code OOK %s (%d bits)", kind, bits), Model: model, FlexDecoder: "n=" + model + "," + key, Bits: bits}, nil
}

func (p DecoderProfile) Validate(persisted bool) error {
	if len(p.Name) == 0 || len(p.Name) > 100 || len(p.FlexDecoder) > 512 {
		return errors.New("invalid learned decoder profile")
	}
	repeats := 1
	if strings.HasSuffix(p.FlexDecoder, "repeats>=2") {
		repeats = 2
	} // Existing schema-2 PWM profiles.
	suffix := fmt.Sprintf(",bits=%d,repeats>=%d", p.Bits, repeats)
	if !strings.HasSuffix(p.FlexDecoder, suffix) {
		return errors.New("profile framing is not bounded")
	}
	candidate := strings.TrimSuffix(p.FlexDecoder, suffix)
	expected, err := profileWithRepeats(candidate, p.Bits, repeats)
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
func repeatedCode(raw []byte, bits int, minimum ...int) (string, int, error) {
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
		if n < 12 || n > 64 || (bits != 0 && n != bits) {
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
		required := 2
		if len(minimum) > 0 {
			required = minimum[0]
		}
		if count < required {
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
	code, _, err := repeatedCode(raw, profile.Bits, 1)
	if err != nil {
		return nil, err
	}
	// Keep only the exact validated identity. No analyzer output reaches this path.
	return json.Marshal(map[string]any{"model": model, "codes": []string{code}})
}

// NormalizeConfiguredFlex uses only an explicitly configured exact bit length.
// It never guesses a width from a single live event or changes another decoder.
func NormalizeConfiguredFlex(raw []byte, spec string, learning ...bool) ([]byte, error) {
	name, bits := "", 0
	seen := map[string]bool{}
	for _, part := range strings.Split(spec, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || (key != "n" && key != "bits") {
			continue
		}
		if seen[key] {
			return raw, nil // Preserve opaque/custom syntax; do not guess its bounds.
		}
		seen[key] = true
		if key == "n" {
			name = value
		} else {
			bits, _ = strconv.Atoi(value)
		}
	}
	if name == "" || bits < 12 || bits > 64 {
		return raw, nil
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return nil, errors.New("invalid RF JSON")
	}
	var model string
	_ = json.Unmarshal(obj["model"], &model)
	if model != name {
		return raw, nil
	}
	if len(learning) > 0 && learning[0] {
		if err := ValidateLearningFrame(raw, bits); err != nil {
			return nil, err
		}
	}
	code, _, err := repeatedCode(raw, bits, 1)
	if err != nil {
		return nil, err
	}
	obj["codes"], _ = json.Marshal([]string{code})
	return json.Marshal(obj)
}

// ValidateLearningFrame prevents a saved profile's one-off edge fragment from
// outranking a different, repeated frame in the same raw rtl_433 record. This
// applies to new learning/inspection, not dispatch of existing saved mappings.
func ValidateLearningFrame(raw []byte, bits int) error {
	var event struct {
		Codes []string `json:"codes"`
	}
	if json.Unmarshal(raw, &event) != nil {
		return errors.New("invalid RF JSON")
	}
	counts := map[string]int{}
	for _, code := range event.Codes {
		m := codePattern.FindStringSubmatch(strings.TrimSpace(code))
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		if n < 12 || n > 64 || len(m[2]) != (n+3)/4 {
			continue
		}
		canonical := fmt.Sprintf("{%d}%s", n, strings.ToLower(m[2]))
		counts[canonical]++
		if counts[canonical] >= 2 && n != bits {
			return errors.New("profile width conflicts with repeated complete frames; fragment excluded from learning")
		}
	}
	return nil
}

// rtl_433 can emit repeated frames in separate pulse packets/JSON records.
// Count across the recording, rather than demanding two repeats in each record.
func matchingFrameCount(raw []byte, bits int) int {
	var event struct {
		Codes []string `json:"codes"`
	}
	if json.Unmarshal(raw, &event) != nil {
		return 0
	}
	n := 0
	for _, code := range event.Codes {
		m := codePattern.FindStringSubmatch(strings.TrimSpace(code))
		if m == nil {
			continue
		}
		width, _ := strconv.Atoi(m[1])
		if width == bits {
			n++
		}
	}
	return n
}
