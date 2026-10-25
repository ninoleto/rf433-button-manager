//go:build linux

package receiver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func syncPWMFixture(bits int, code uint64, jitter int) []byte {
	var pairs [][2]int
	short, long, sync, gap, pause := 60, 190, 450, 80, 1020
	if bits == 12 {
		short, long, sync, gap, pause = 72, 272, 1272, 128, 1528
	}
	pairs = append(pairs, [2]int{short, gap}) // incomplete leading row
	for frame := 0; frame < 3; frame++ {
		pairs = append(pairs, [2]int{sync, gap})
		for bit := bits - 1; bit >= 0; bit-- {
			mark := short
			if code&(1<<bit) == 0 {
				mark = long
				if bits == 12 && bit%3 == 0 {
					mark += 200
				}
			}
			delta := 0
			if jitter != 0 {
				delta = ((bit+frame)%3 - 1) * jitter
			}
			space := gap - delta
			if bit == 0 {
				space = pause
			}
			pairs = append(pairs, [2]int{mark + delta, space})
		}
	}
	pairs = append(pairs, [2]int{sync, gap}, [2]int{short, 13000})
	var b strings.Builder
	fmt.Fprintf(&b, ";pulse data\n;version 1\n;timescale 1us\n;ook %d pulses\n", len(pairs))
	for _, p := range pairs {
		fmt.Fprintf(&b, "%d %d\n", p[0], p[1])
	}
	b.WriteString(";end\n")
	return []byte(b.String())
}

func TestMeasuredSyncPWM12And16BitDiscovery(t *testing.T) {
	for _, tc := range []struct {
		bits int
		code uint64
	}{{12, 0xa69}, {16, 0x6a3d}} {
		t.Run(fmt.Sprint(tc.bits), func(t *testing.T) {
			raw := syncPWMFixture(tc.bits, tc.code, 8)
			if len(measuredPWMTimings(raw)) == 0 {
				t.Fatal("no measured PWM candidate")
			}
			req := AnalysisRequest{Binary: installedRTL(t), Capture: raw, Signature: replaySignature}
			result, err := AnalyzeCapture(context.Background(), req)
			if err != nil || result.Profile == nil || result.Profile.Bits != tc.bits {
				t.Fatalf("discovery: %+v %v", result, err)
			}
			want := fmt.Sprintf("{%d}%0*x", tc.bits, (tc.bits+3)/4, tc.code)
			if !strings.Contains(string(result.Raw), want) {
				t.Fatal("wrong decoded identity", string(result.Raw))
			}
			req.Selected = result.Profile
			req.Capture = syncPWMFixture(tc.bits, tc.code, 12)
			second, err := AnalyzeCapture(context.Background(), req)
			if err != nil || string(second.Raw) != string(result.Raw) {
				t.Fatal("jittered press did not retain identity", err)
			}
			req.Capture = syncPWMFixture(tc.bits, tc.code^1, 8)
			other, err := AnalyzeCapture(context.Background(), req)
			if err == nil && string(other.Raw) == string(result.Raw) {
				t.Fatal("different complete code collapsed to the same identity")
			}
			if len(measuredPWMTimings([]byte(strings.ReplaceAll(string(raw), ";ook ", ";fsk ")))) != 0 {
				t.Fatal("FSK accepted")
			}
		})
	}
}

func TestRepeatedWidthPrefersCompleteRowsAcrossPackets(t *testing.T) {
	frames := []json.RawMessage{
		json.RawMessage(`{"codes":["{12}a3d","{16}6a3d","{16}6a3d","{4}6"]}`),
		json.RawMessage(`{"codes":["{12}a3d","{16}6a3d","{16}6a3d","{5}68"]}`),
	}
	if repeatedFrameWidth(frames) != 16 {
		t.Fatal("repeated edge fragment hid the complete row")
	}
	frames = append(frames, json.RawMessage(`{"codes":["{16}6a3e"]}`))
	if repeatedFrameWidth(frames) != 0 {
		t.Fatal("conflicting full-length code accepted")
	}
	if repeatedFrameWidth([]json.RawMessage{json.RawMessage(`{"codes":["{12}a69","{12}a69","{16}6a3d","{16}6a3d"]}`)}) != 0 {
		t.Fatal("two repeated widths accepted")
	}
}

func TestConfiguredFlexNormalizesOnlyExplicitWidth(t *testing.T) {
	spec := "n=Fixture,m=OOK_PWM,s=72,l=272,r=11000,y=1272,bits=12"
	raw := []byte(`{"model":"Fixture","codes":["{1}8","{12}a69","{12}a69","{13}a690"],"rssi":-10}`)
	got, err := NormalizeConfiguredFlex(raw, spec)
	if err != nil {
		t.Fatal(err)
	}
	var event struct {
		Codes []string
		RSSI  int
	}
	_ = json.Unmarshal(got, &event)
	if len(event.Codes) != 1 || event.Codes[0] != "{12}a69" || event.RSSI != -10 {
		t.Fatal(string(got))
	}
	if _, err = NormalizeConfiguredFlex([]byte(`{"model":"Fixture","codes":["{12}a69","{12}a68"]}`), spec); err == nil {
		t.Fatal("same-width conflict accepted")
	}
	for _, unbounded := range []string{"", "n=Fixture,bits>=12", "n=Other,bits=12", "n=Fixture,bits=12,bits=16"} {
		got, err = NormalizeConfiguredFlex(raw, unbounded)
		if err != nil || string(got) != string(raw) {
			t.Fatal("guessed or changed another decoder")
		}
	}
}

// Private RF identifiers never enter the repository. An operator-supplied
// manifest pairs recordings with expected codes for optional offline regression.
func TestPrivateLearningRecordingsOptional(t *testing.T) {
	dir := os.Getenv("RFB_PRIVATE_CAPTURE_DIR")
	if dir == "" {
		t.Skip("private regression recordings not supplied")
	}
	data, err := os.ReadFile(filepath.Join(dir, "regression.json"))
	if err != nil {
		t.Fatal(err)
	}
	var groups []struct {
		Name, Code string
		Files      []string
	}
	if err = json.Unmarshal(data, &groups); err != nil {
		t.Fatal(err)
	}
	for _, group := range groups {
		t.Run(group.Name, func(t *testing.T) {
			var learned *DecoderProfile
			for _, file := range group.Files {
				raw, err := os.ReadFile(filepath.Join(dir, file))
				if err != nil {
					t.Fatal(err)
				}
				req := AnalysisRequest{Binary: installedRTL(t), Capture: raw, Signature: replaySignature}
				result, err := AnalyzeCapture(context.Background(), req)
				if err != nil || result.Profile == nil || !strings.Contains(string(result.Raw), group.Code) {
					t.Fatalf("recording %s discovery failed: %v", file, err)
				}
				if learned == nil {
					learned = result.Profile
				}
				req.Selected = learned
				result, err = AnalyzeCapture(context.Background(), req)
				if err != nil || !strings.Contains(string(result.Raw), group.Code) {
					t.Fatalf("recording %s profile reuse failed: %v", file, err)
				}
			}
			t.Logf("%d recordings: automatic discovery and saved-profile replay passed", len(group.Files))
		})
	}
}
