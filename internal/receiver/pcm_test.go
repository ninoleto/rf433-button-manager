//go:build linux

package receiver

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestPCMReplayWithPartialFrames(t *testing.T) {
	raw, err := os.ReadFile("testdata/synthetic-pcm.ook")
	if err != nil {
		t.Fatal(err)
	}
	req := AnalysisRequest{Binary: installedRTL(t), Capture: raw, Signature: replaySignature}
	result, err := AnalyzeCapture(context.Background(), req)
	if err != nil || result.Profile == nil || !strings.Contains(result.Profile.FlexDecoder, "m=OOK_PCM,") {
		t.Fatalf("measured PCM replay failed: %v (%+v)", err, result.Diagnostics)
	}
	if result.Profile.Validate(false) != nil || result.Profile.Validate(true) == nil {
		t.Fatal("single recording incorrectly counts as three presses")
	}
	req.Selected = result.Profile
	second, err := AnalyzeCapture(context.Background(), req)
	if err != nil || string(second.Raw) != string(result.Raw) {
		t.Fatal("profile reuse failed", err)
	}
	// Unsupported timing grids must not be turned into a plausible decoder.
	if got := measuredPCMTimings([]byte(strings.ReplaceAll(string(raw), "308 92", "350 155"))); len(got) != 0 {
		t.Fatal("non-integer pulse timing accepted", got)
	}
	if got := measuredPCMTimings([]byte(";ook 2 pulses\n100 800\n100 800\n;end\n")); len(got) != 0 {
		t.Fatal("short traffic generated a candidate")
	}
	if got := measuredPCMTimings([]byte(strings.Replace(string(raw), "308 92\n", "", 1))); len(got) != 0 {
		t.Fatal("different complete frame lengths accepted")
	}
	if got := measuredPCMTimings([]byte(strings.ReplaceAll(string(raw), ";ook ", ";fsk "))); len(got) != 0 {
		t.Fatal("FSK data relabeled as OOK")
	}
}

func TestPCMRejectsAmbiguousRepeats(t *testing.T) {
	for _, codes := range []string{
		`["{24}aabbcc"]`,
		`["{24}aabbcc","{24}aabbcc","{24}aabbdd"]`,
		`["{24}aabbcc","{24}aabbcc","{16}1234","{16}1234"]`,
	} {
		if repeatedFrameWidth([]json.RawMessage{json.RawMessage(`{"codes":` + codes + `}`)}) != 0 {
			t.Fatal("ambiguous code accepted")
		}
	}
	if repeatedFrameWidth([]json.RawMessage{json.RawMessage(`{"codes":["{16}1234","{24}aabbcc","{24}aabbcc","{20}12345"]}`)}) != 24 {
		t.Fatal("incomplete prefix/suffix hid an unambiguous repeated frame")
	}
}

// The operator's recording stays outside the repository and release. This is
// offline replay evidence, never evidence of three separate physical presses.
func TestOperatorCaptureReplayOptional(t *testing.T) {
	path := os.Getenv("RFB_OPERATOR_CAPTURE")
	if path == "" {
		t.Skip("optional private operator RF recording")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var recording struct {
		Packets []struct {
			Modulation string `json:"modulation"`
			Data       string `json:"pulse_data"`
		} `json:"packets"`
	}
	if err = json.Unmarshal(raw, &recording); err != nil {
		t.Fatal(err)
	}
	accepted := 0
	for _, packet := range recording.Packets {
		if !strings.EqualFold(packet.Modulation, "OOK") {
			continue
		}
		req := AnalysisRequest{Binary: installedRTL(t), Capture: []byte(packet.Data), Signature: replaySignature}
		result, err := AnalyzeCapture(context.Background(), req)
		if err != nil {
			continue
		}
		if result.Profile == nil || !strings.Contains(result.Profile.FlexDecoder, "m=OOK_PCM,") {
			t.Fatal("unexpected recorded candidate")
		}
		req.Selected = result.Profile
		second, err := AnalyzeCapture(context.Background(), req)
		if err != nil || string(second.Raw) != string(result.Raw) {
			t.Fatal("recorded candidate did not replay consistently")
		}
		accepted++
	}
	if accepted != 1 {
		t.Fatalf("expected one repeatable PCM packet, got %d", accepted)
	}
}
