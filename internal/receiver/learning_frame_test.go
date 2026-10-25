//go:build linux

package receiver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLearningRejectsSavedFragmentWithoutChangingRuntimeNormalization(t *testing.T) {
	profile, err := profileFromTiming("m=OOK_PWM,s=100,l=300,r=10000,g=0,t=0,y=900", 12)
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"model":"` + profile.Model + `","codes":["{12}a69","{16}ba69","{16}ba69","{5}28"]}`)
	if ValidateLearningFrame(raw, 12) == nil || ValidateLearningFrame(raw, 16) != nil {
		t.Fatal("preferred fragment beat complete repeated frame")
	}
	if _, err := NormalizeProfile(raw, profile); err != nil {
		t.Fatal("existing mapping semantics changed", err)
	}
	spec := "n=" + profile.Model + ",m=OOK_PWM,s=100,l=300,r=10000,bits=12"
	if _, err := NormalizeConfiguredFlex(raw, spec, true); err == nil {
		t.Fatal("custom flex bypassed learning guard")
	}
	if _, err := NormalizeConfiguredFlex(raw, spec); err != nil {
		t.Fatal("existing custom mapping changed")
	}
	for _, codes := range []string{`["{3}e","{12}a69","{12}a69","{13}a690"]`, `["{12}a69"]`} {
		if ValidateLearningFrame([]byte(`{"codes":`+codes+`}`), 12) != nil {
			t.Fatal("genuine 12-bit record rejected")
		}
	}
	if ValidateLearningFrame([]byte(`{"codes":["{12}a69","{12}a69","{16}ba69","{16}ba69"]}`), 12) == nil {
		t.Fatal("ambiguous repeated widths accepted")
	}
}

// Operator captures stay outside source control and release packages.
func TestPrivateSavedFragmentReplayOptional(t *testing.T) {
	dir := os.Getenv("RFB_PRIVATE_CAPTURE_DIR")
	if dir == "" {
		t.Skip("private regression recordings not supplied")
	}
	manifest, err := os.ReadFile(filepath.Join(dir, "fragment-regression.json"))
	if os.IsNotExist(err) {
		t.Skip("fragment regression not supplied")
	}
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Profile DecoderProfile
		Files   []string
		Bits    int
		Code    string
	}
	if err = json.Unmarshal(manifest, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, file := range fixture.Files {
		t.Run(file, func(t *testing.T) {
			capture, err := os.ReadFile(filepath.Join(dir, file))
			if err != nil {
				t.Fatal(err)
			}
			req := AnalysisRequest{Binary: installedRTL(t), Capture: capture, Config: Config{Profiles: []DecoderProfile{fixture.Profile}}, PreferredIDs: []string{fixture.Profile.ID}, Signature: replaySignature}
			result, err := AnalyzeCapture(context.Background(), req)
			if err != nil || result.Profile == nil || result.Profile.Bits != fixture.Bits || !strings.Contains(string(result.Raw), fixture.Code) {
				t.Fatalf("saved fragment took priority: %v", err)
			}
			req.Selected = &fixture.Profile
			if _, err := AnalyzeCapture(context.Background(), req); err == nil {
				t.Fatal("selected partial profile incorrectly validated")
			}
		})
	}
}
