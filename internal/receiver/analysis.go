package receiver

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type LearningDiagnostics struct {
	Packets    int    `json:"packets"`
	Pulses     int    `json:"pulses"`
	Bytes      int    `json:"bytes"`
	Candidates int    `json:"candidates"`
	Replays    int    `json:"replays"`
	Source     string `json:"source"`
	Detail     string `json:"detail"`
}

type AnalysisRequest struct {
	Binary       string
	TempDir      string
	Capture      []byte
	Config       Config
	Selected     *DecoderProfile
	PreferredIDs []string
	Signature    func([]byte) (string, error)
}
type AnalysisResult struct {
	Raw         json.RawMessage
	Profile     *DecoderProfile
	Diagnostics LearningDiagnostics
}

type LearningError struct{ Code, Message string }

func (e *LearningError) Error() string { return e.Message }

func analysisFailure(code, message string) error { return &LearningError{Code: code, Message: message} }

var errMixedIdentity = errors.New("different identities decoded in one capture")

func AnalyzeCapture(parent context.Context, request AnalysisRequest) (result AnalysisResult, err error) {
	if len(request.Capture) == 0 || len(request.Capture) > MaxCaptureBytes || request.Signature == nil {
		return result, analysisFailure("unsupported", "Capture was empty or exceeded supported limits. Retry with one brief press.")
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	directory, err := os.MkdirTemp(request.TempDir, "rfb-learn-")
	if err != nil {
		return result, analysisFailure("capture_error", "Cannot create private learning scratch data. Check application storage.")
	}
	defer os.RemoveAll(directory)
	file := filepath.Join(directory, "capture_433.92M_250k.ook")
	if err = os.WriteFile(file, request.Capture, 0600); err != nil {
		return result, err
	}
	result.Diagnostics.Bytes = len(request.Capture)
	replay := func(extra []string) ([]json.RawMessage, string, error) {
		result.Diagnostics.Replays++
		args := []string{"-c", "/dev/null", "-r", file, "-s", "250000", "-F", "json"}
		args = append(args, extra...)
		output, diagnostic, e := offlineCommand(ctx, request.Binary, args)
		if e != nil {
			return nil, "", e
		}
		var frames []json.RawMessage
		for _, line := range strings.Split(output, "\n") {
			if classify([]byte(line)) == event {
				frames = append(frames, json.RawMessage(line))
			}
		}
		return frames, diagnostic, nil
	}
	consistent := func(frames []json.RawMessage, profile *DecoderProfile) (json.RawMessage, error) {
		var chosen json.RawMessage
		identity := ""
		for _, frame := range frames {
			normalized := []byte(frame)
			if profile != nil {
				var e error
				normalized, e = NormalizeProfile(frame, *profile)
				if e != nil {
					return nil, e
				}
			}
			signature := string(normalized) // Profile normalization already fixes model + exact code.
			if profile == nil {
				var e error
				signature, e = request.Signature(normalized)
				if e != nil {
					continue
				}
			}
			if identity != "" && signature != identity {
				return nil, errMixedIdentity
			}
			identity = signature
			chosen = append(json.RawMessage(nil), normalized...)
		}
		if chosen == nil {
			return nil, errors.New("no stable decoded identity")
		}
		return chosen, nil
	}
	tryProfile := func(profile DecoderProfile) (json.RawMessage, error) {
		if e := profile.Validate(false); e != nil {
			return nil, e
		}
		frames, _, e := replay([]string{"-R", "0", "-X", profile.FlexDecoder})
		if e != nil {
			return nil, e
		}
		return consistent(frames, &profile)
	}
	if request.Selected != nil {
		raw, e := tryProfile(*request.Selected)
		if e != nil {
			return result, analysisFailure("inconsistent", "This press did not decode consistently with the first capture. Use the same button, or retry learning. Changing/rolling codes are not supported.")
		}
		result.Raw = raw
		profile := *request.Selected
		result.Profile = &profile
		result.Diagnostics.Source = "validated profile replay"
		return result, nil
	}
	// Prefer a profile already used on this remote, then preserve legacy/default decoding.
	tried := map[string]bool{}
	for _, id := range request.PreferredIDs {
		if tried[id] {
			continue
		}
		tried[id] = true
		for _, profile := range request.Config.Profiles {
			if profile.ID == id {
				if raw, e := tryProfile(profile); e == nil {
					result.Raw = raw
					result.Profile = &profile
					result.Diagnostics.Source = "existing remote profile"
					return result, nil
				}
			}
		}
	}
	normal := []string{}
	if request.Config.FlexDecoder != "" || len(request.Config.Protocols) > 0 {
		normal = append(normal, "-R", "0")
	}
	for _, id := range request.Config.Protocols {
		normal = append(normal, "-R", intString(id))
	}
	if request.Config.FlexDecoder != "" {
		normal = append(normal, "-X", request.Config.FlexDecoder)
	}
	if frames, _, e := replay(normal); e == nil {
		if raw, e := consistent(frames, nil); e == nil {
			result.Raw = raw
			result.Diagnostics.Source = "existing or built-in decoder"
			return result, nil
		} else if errors.Is(e, errMixedIdentity) {
			return result, analysisFailure("inconsistent", "Different button identities were decoded in one capture. Keep other remotes idle and retry one brief press. No mapping changed.")
		}
	}
	for _, profile := range request.Config.Profiles {
		if tried[profile.ID] {
			continue
		}
		if raw, e := tryProfile(profile); e == nil {
			result.Raw = raw
			result.Profile = &profile
			result.Diagnostics.Source = "existing validated profile"
			return result, nil
		}
	}
	_, analyzer, e := replay([]string{"-R", "0", "-A"})
	if e != nil {
		return result, analysisFailure("analysis_error", "Capture analysis could not finish within its resource limits. Retry learning.")
	}
	timings := []string{}
	seen := map[string]bool{}
	for _, match := range suggestionPattern.FindAllStringSubmatch(analyzer, -1) {
		timing, e := canonicalTiming(match[1])
		if e != nil || seen[timing] {
			continue
		}
		seen[timing] = true
		timings = append(timings, timing)
	}
	result.Diagnostics.Candidates = len(timings)
	if len(timings) == 0 || len(timings) > 8 {
		return result, analysisFailure("unsupported", "Transmission detected, but no unambiguous supported fixed-code OOK/PWM candidate was found. Rolling/encrypted and other unsupported signals cannot be learned.")
	}
	var accepted *DecoderProfile
	var acceptedRaw json.RawMessage
	identity := ""
	for _, timing := range timings {
		frames, _, e := replay([]string{"-R", "0", "-X", "n=RFB-Probe," + timing + ",bits>=16,bits<=64,repeats>=2"})
		if e != nil {
			continue
		}
		code := ""
		bits := 0
		valid := len(frames) > 0
		for _, frame := range frames {
			next, n, e := repeatedCode(frame, 0)
			if e != nil || (code != "" && next != code) {
				valid = false
				break
			}
			code = next
			bits = n
		}
		if !valid || code == "" {
			continue
		}
		if identity != "" && identity != code {
			return result, analysisFailure("inconsistent", "Multiple different fixed-code interpretations were detected. Keep other remotes idle and retry; no profile was saved.")
		}
		profile, e := profileFromTiming(timing, bits)
		if e != nil {
			continue
		}
		raw, e := tryProfile(profile)
		if e != nil {
			continue
		}
		identity = code
		if accepted == nil {
			accepted = &profile
			acceptedRaw = raw
		}
	}
	if accepted == nil {
		return result, analysisFailure("unsupported", "Transmission detected, but analyzer candidates did not reproduce a consistent repeated fixed code. No decoder or mapping was saved.")
	}
	if ctx.Err() != nil {
		return result, analysisFailure("analysis_error", "Analysis was cancelled or exceeded its time limit. No profile was saved.")
	}
	result.Raw = acceptedRaw
	result.Profile = accepted
	result.Diagnostics.Source = "analyzer candidate validated by capture replay"
	return result, nil
}
