//go:build linux

package receiver

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func ppmFixture(separate bool) []byte {
	var b strings.Builder
	header := ";pulse data\n;version 1\n;timescale 1us\n"
	if !separate {
		fmt.Fprintf(&b, "%s;ook 150 pulses\n", header)
	}
	for frame := 0; frame < 6; frame++ {
		if separate {
			fmt.Fprintf(&b, "%s;ook 25 pulses\n", header)
		}
		for bit := 23; bit >= 0; bit-- {
			gap := 500
			if 0xa9b2c5&(1<<bit) != 0 {
				gap = 1500
			}
			fmt.Fprintf(&b, "300 %d\n", gap)
		}
		b.WriteString("300 10000\n")
		if separate {
			b.WriteString(";end\n")
		}
	}
	if !separate {
		b.WriteString(";end\n")
	}
	return []byte(b.String())
}

func TestAutomaticPPMAndSeparatePacketReplay(t *testing.T) {
	for _, separate := range []bool{false, true} {
		t.Run(fmt.Sprint(separate), func(t *testing.T) {
			req := AnalysisRequest{Binary: installedRTL(t), Capture: ppmFixture(separate), Config: Config{Protocols: []int{1}}, Signature: replaySignature}
			result, err := AnalyzeCapture(context.Background(), req)
			if err != nil {
				t.Fatalf("PPM discovery failed: %v (%+v)", err, result.Diagnostics)
			}
			if result.Profile == nil || !strings.Contains(result.Profile.FlexDecoder, "m=OOK_PPM,") || !strings.Contains(string(result.Raw), "a9b2c5") {
				t.Fatalf("wrong PPM profile: %+v %s", result.Profile, result.Raw)
			}
			req.Selected = result.Profile
			if _, err = AnalyzeCapture(context.Background(), req); err != nil {
				t.Fatal("profile did not replay", err)
			}
			// Future events can contain a single frame. Repetition was verified
			// during learning; it is not required within every runtime JSON row.
			raw := []byte(fmt.Sprintf(`{"model":%q,"codes":["{24}a9b2c5"]}`, result.Profile.Model))
			if _, err = NormalizeProfile(raw, *result.Profile); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLegacyPWMProfileStillValidates(t *testing.T) {
	p, err := profileWithRepeats("m=OOK_PWM,s=400,l=1200,r=10004,g=1204,t=320,y=0", 24, 2)
	if err != nil || p.Validate(false) != nil {
		t.Fatal("existing schema-2 profile rejected", err)
	}
	if !strings.Contains(p.FlexDecoder, ",repeats>=2") || !strings.HasPrefix(p.Model, "RFB-PWM-") {
		t.Fatal("legacy identity changed")
	}
}
