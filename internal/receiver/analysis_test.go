//go:build linux

package receiver

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func syntheticCapture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/synthetic-pwm.ook")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func replaySignature(raw []byte) (string, error) {
	var frame struct {
		Model string   `json:"model"`
		Codes []string `json:"codes"`
	}
	if json.Unmarshal(raw, &frame) != nil || frame.Model == "" || len(frame.Codes) == 0 {
		return "", errors.New("no identity")
	}
	for _, code := range frame.Codes {
		if code != frame.Codes[0] {
			return "", errors.New("ambiguous code")
		}
	}
	return frame.Model + ":" + frame.Codes[0], nil
}

func installedRTL(t *testing.T) string {
	t.Helper()
	binary, err := exec.LookPath("rtl_433")
	if err != nil {
		t.Skip("offline integration requires rtl_433 (no USB is opened)")
	}
	return binary
}

func TestAutomaticPWMReplayIntegration(t *testing.T) {
	scratch := t.TempDir()
	request := AnalysisRequest{Binary: installedRTL(t), TempDir: scratch, Capture: syntheticCapture(t), Signature: replaySignature}
	result, err := AnalyzeCapture(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Profile == nil || result.Profile.Bits != 24 || result.Profile.Validate(false) != nil || result.Diagnostics.Candidates < 1 || result.Diagnostics.Replays < 3 {
		t.Fatalf("missing replay proof: %+v", result)
	}
	if result.Profile.Validate(true) == nil {
		t.Fatal("a single replay must not count as three physical presses")
	}
	if !strings.Contains(string(result.Raw), "{24}a9b2c5") {
		t.Fatal(string(result.Raw))
	}
	// Exercise the real combined JSON/pulse stdout format, still using file input.
	stream, _, err := offlineCommand(context.Background(), request.Binary, []string{"-c", "/dev/null", "-r", "testdata/synthetic-pwm.ook", "-R", "0", "-X", result.Profile.FlexDecoder, "-F", "json", "-w", "OOK:-"})
	if err != nil {
		t.Fatal(err)
	}
	var assembler pulseAssembler
	var packets []PulsePacket
	decoded := 0
	for _, line := range strings.Split(stream, "\n") {
		packet, consumed := assembler.feed([]byte(line))
		if packet != nil {
			packets = append(packets, *packet)
		}
		if !consumed && classify([]byte(line)) == event {
			decoded++
		}
	}
	if decoded < 1 || len(packets) != 1 {
		t.Fatal("live-format JSON/pulse separation failed", decoded, len(packets))
	}
	if _, err = ValidatePulseCapture(packets); err != nil {
		t.Fatal(err)
	}
	// A later press uses the same profile even with slightly different timings.
	request.Selected = result.Profile
	request.Capture = []byte(strings.ReplaceAll(strings.ReplaceAll(string(request.Capture), "400 ", "405 "), "1200 ", "1195 "))
	second, err := AnalyzeCapture(context.Background(), request)
	if err != nil || string(second.Raw) != string(result.Raw) || second.Diagnostics.Candidates != 0 {
		t.Fatalf("profile reuse failed: %+v %v", second, err)
	}
	files, err := os.ReadDir(scratch)
	if err != nil || len(files) != 0 {
		t.Fatal("private capture files not cleaned", files, err)
	}
}

func TestAnalyzerCandidateIsNotEvidence(t *testing.T) {
	// The fake analyzer recommends plausible timings but emits no decoded replay.
	binary := writeScript(t, "printf \"Use a flex decoder with -X 'n=name,m=OOK_PWM,s=400,l=1200,r=10004,g=1204,t=320,y=0'\\n\" >&2\n")
	result, err := AnalyzeCapture(context.Background(), AnalysisRequest{Binary: binary, Capture: syntheticCapture(t), Signature: replaySignature})
	var failure *LearningError
	if !errors.As(err, &failure) || failure.Code != "unsupported" || result.Profile != nil || result.Diagnostics.Candidates != 1 {
		t.Fatalf("unproven suggestion accepted: %+v %v", result, err)
	}
}

func TestMixedDecodedCaptureFailsBeforeCandidateDiscovery(t *testing.T) {
	binary := writeScript(t, "printf '%s\\n' '{\"model\":\"Existing\",\"codes\":[\"{24}a9b2c5\"]}' '{\"model\":\"Existing\",\"codes\":[\"{24}a9b2c6\"]}'\n")
	result, err := AnalyzeCapture(context.Background(), AnalysisRequest{Binary: binary, Capture: syntheticCapture(t), Signature: replaySignature})
	var failure *LearningError
	if !errors.As(err, &failure) || failure.Code != "inconsistent" || result.Diagnostics.Replays != 1 || result.Profile != nil {
		t.Fatalf("mixed identities hidden by fallback: %+v %v", result, err)
	}
}

func TestPWMRejectsMixedCodesAndUnsafeParameters(t *testing.T) {
	for _, spec := range []string{"n=name,m=OOK_PPM,s=400,l=1200,r=10004", "n=name,m=OOK_PWM,s=400,l=1200,r=10004,decode_uart", "n=name,m=OOK_PWM,s=400,l=1200,r=10004,s=500", "n=name,m=OOK_PWM,s=400,l=1200,r=99999999"} {
		if _, err := canonicalTiming(spec); err == nil {
			t.Fatal("unsafe/unsupported candidate", spec)
		}
	}
	for _, raw := range []string{`{"codes":["{24}a9b2c5"]}`, `{"codes":["{24}a9b2c5","{24}a9b2c6"]}`, `{"codes":["{24}a9b2c5","{24}a9b2c5","{24}a9b2c6","{24}a9b2c6"]}`, `{"codes":["{24}a9","{24}a9"]}`} {
		if _, _, err := repeatedCode([]byte(raw), 24); err == nil {
			t.Fatal("ambiguous or unrepeated code accepted", raw)
		}
	}
}

func TestOfflineCancelAndOutputBounds(t *testing.T) {
	for _, args := range [][]string{{"-F", "json"}, {"-r", "-"}, {"-r", "file.ook", "-d", "0"}} {
		if _, _, err := offlineCommand(context.Background(), "missing", args); err == nil {
			t.Fatal("allowed receiver access")
		}
	}
	binary := writeScript(t, "trap '' TERM\nwhile :; do sleep 1; done\n")
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, _, err := offlineCommand(ctx, binary, []string{"-r", "test.ook"}); err == nil {
		t.Fatal("timeout ignored")
	}
	if time.Since(start) > time.Second {
		t.Fatal("analysis child did not stop promptly")
	}
	binary = writeScript(t, "head -c 600000 /dev/zero | tr '\\000' x\n")
	if _, _, err := offlineCommand(context.Background(), binary, []string{"-r", "test.ook"}); err == nil {
		t.Fatal("unbounded output accepted")
	}
}

func TestPulseAssemblerAndCaptureBounds(t *testing.T) {
	var assembler pulseAssembler
	var packets []PulsePacket
	for _, line := range strings.Split(string(syntheticCapture(t)), "\n") {
		if strings.HasPrefix(line, ";ook") {
			if _, consumed := assembler.feed([]byte(`{"model":"interleaved"}`)); consumed {
				t.Fatal("JSON consumed as pulses")
			}
		}
		packet, _ := assembler.feed([]byte(line))
		if packet != nil {
			packets = append(packets, *packet)
		}
	}
	if len(packets) != 1 || packets[0].Pulses != 144 || packets[0].Error != "" {
		t.Fatalf("bad packet: %+v", packets)
	}
	if _, err := ValidatePulseCapture(packets); err != nil {
		t.Fatal(err)
	}
	packets[0].OOK = false
	if _, err := ValidatePulseCapture(packets); err == nil {
		t.Fatal("FSK should be unsupported")
	}
	for _, line := range []string{";ook 9000 pulses", "1 2", ";end"} {
		if packet, _ := assembler.feed([]byte(line)); packet != nil && packet.Error == "" {
			t.Fatal("malformed capture accepted")
		}
	}
	packet := PulsePacket{OOK: true, Data: make([]byte, MaxCaptureBytes+1)}
	if _, err := ValidatePulseCapture([]PulsePacket{packet}); err == nil {
		t.Fatal("capture exceeded memory budget")
	}
}
