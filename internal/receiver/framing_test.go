//go:build linux

package receiver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func framingProfile(t *testing.T, timing string, bits int) DecoderProfile {
	t.Helper()
	p, err := profileFromTiming(timing, bits)
	if err != nil {
		t.Fatal(err)
	}
	p.ValidatedPresses = 3
	p.CreatedAt = "2026-01-01T00:00:00Z"
	return p
}

const framingTiming = "m=OOK_PWM,s=320,l=960,r=9000,g=950,t=120,y=0"

// Entirely synthetic; the bit word and timings do not come from an operator.
func framingPulse(word uint32, bits int, gap int) string {
	var b strings.Builder
	fmt.Fprintf(&b, ";pulse data\n;version 1\n;timescale 1us\n;ook %d pulses\n320 10000\n", bits+1)
	for bit := bits - 1; bit >= 0; bit-- {
		mark, space := 960, 340
		if word&(1<<bit) != 0 {
			mark, space = 320, gap
		}
		if bit == 0 {
			space = 10000
		}
		fmt.Fprintf(&b, "%d %d\n", mark, space)
	}
	b.WriteString(";end\n")
	return b.String()
}

func TestFramingFallbackScopeAndLearningIsolation(t *testing.T) {
	p := framingProfile(t, framingTiming, 25)
	before := p
	fallback := ProfileFramingFallbacks([]DecoderProfile{p})
	if len(fallback) != 1 || !strings.Contains(fallback[0], ",g=0,") || p != before || p.Validate(true) != nil {
		t.Fatal("missing bounded fallback or changed stored profile")
	}
	for _, capture := range []bool{false, true} {
		args := arguments(Config{Profiles: []DecoderProfile{p}, CapturePulses: capture}, device{})
		var specs []string
		for i, s := range args {
			if s == "-X" {
				specs = append(specs, args[i+1])
			}
		}
		want := []string{p.FlexDecoder}
		if !capture {
			want = append(want, fallback...)
		}
		if !reflect.DeepEqual(specs, want) {
			t.Fatal("normal/learning decoder selection changed")
		}
	}
	for _, tc := range []struct {
		name, timing string
		bits         int
	}{
		{"other width", framingTiming, 24},
		{"safe gap", strings.Replace(framingTiming, "g=950", "g=2000", 1), 25},
		{"no gap", strings.Replace(framingTiming, "g=950", "g=0", 1), 25},
		{"short gap", strings.Replace(framingTiming, "g=950", "g=400", 1), 25},
		{"sync", strings.Replace(framingTiming, "y=0", "y=1600", 1), 25},
		{"unbounded tolerance", strings.Replace(framingTiming, "t=120", "t=0", 1), 25},
		{"overlapping widths", strings.Replace(framingTiming, "t=120", "t=400", 1), 25},
		{"no separated delimiter", strings.Replace(framingTiming, "r=9000", "r=1400", 1), 25},
		{"PPM", strings.Replace(framingTiming, "OOK_PWM", "OOK_PPM", 1), 25},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if len(ProfileFramingFallbacks([]DecoderProfile{framingProfile(t, tc.timing, tc.bits)})) != 0 {
				t.Fatal("unrelated profile broadened")
			}
		})
	}
	p.ValidatedPresses = 2
	if len(ProfileFramingFallbacks([]DecoderProfile{p})) != 0 {
		t.Fatal("unvalidated profile accepted")
	}
	p = before
	p.FlexDecoder = strings.Replace(p.FlexDecoder, "g=950", "g=949", 1)
	if len(ProfileFramingFallbacks([]DecoderProfile{p})) != 0 {
		t.Fatal("tampered timing identity accepted")
	}
}

func TestFramingFallbackFileReplayCompleteWordsOnly(t *testing.T) {
	binary := installedRTL(t)
	p := framingProfile(t, framingTiming, 25)
	fallback := ProfileFramingFallbacks([]DecoderProfile{p})[0]
	file := filepath.Join(t.TempDir(), "synthetic.ook")
	replay := func(pulse, spec string) []string {
		t.Helper()
		if err := os.WriteFile(file, []byte(pulse), 0600); err != nil {
			t.Fatal(err)
		}
		output, _, err := offlineCommand(context.Background(), binary, []string{"-c", "/dev/null", "-r", file, "-s", "250000", "-R", "0", "-X", spec, "-F", "json"})
		if err != nil {
			t.Fatal(err)
		}
		var codes []string
		for _, line := range strings.Split(output, "\n") {
			if !json.Valid([]byte(line)) {
				continue
			}
			normalized, err := NormalizeProfile([]byte(line), p)
			if err != nil {
				continue
			}
			var event struct{ Codes []string }
			_ = json.Unmarshal(normalized, &event)
			codes = append(codes, event.Codes...)
		}
		return codes
	}
	const word uint32 = 0x1357981
	want := fmt.Sprintf("{25}%07x", word<<3)
	if got := replay(framingPulse(word, 25, 980), p.FlexDecoder); len(got) != 0 {
		t.Fatal("synthetic case did not reproduce fragmentation")
	}
	for _, gap := range []int{900, 948, 952, 980, 1000} {
		if got := replay(framingPulse(word, 25, gap), fallback); len(got) != 1 || got[0] != want {
			t.Fatal("complete word lost across data-gap boundary", gap, got)
		}
	}
	for bits := 1; bits < 25; bits++ {
		if got := replay(framingPulse(word>>(25-bits), bits, 980), fallback); len(got) != 0 {
			t.Fatal("incomplete word accepted", bits)
		}
	}
	if got := replay(framingPulse(word, 26, 980), fallback); len(got) != 0 {
		t.Fatal("overlong word accepted")
	}
	for bit := 0; bit < 25; bit++ {
		if got := replay(framingPulse(word^(1<<bit), 25, 980), fallback); len(got) != 1 || got[0] == want {
			t.Fatal("changed payload bit collapsed", bit)
		}
	}
	fragments := framingPulse(word>>13, 12, 980) + framingPulse(word&8191, 13, 980)
	if got := replay(fragments, fallback); len(got) != 0 {
		t.Fatal("separate truncated frames joined")
	}
}
