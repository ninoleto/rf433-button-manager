//go:build linux

package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ninoleto/rf433-button-manager/internal/receiver"
)

// A private manifest contains file paths and expected counts, never committed
// identities. This exercises actual file replay, matching, debounce and isolated
// HTTP delivery. No USB, production destination or user configuration is used.
func TestPrivateFramingRecordingsToWebhook(t *testing.T) {
	manifest := os.Getenv("RFB_FRAMING_RECORDINGS")
	if manifest == "" {
		t.Skip("private normal-use recordings not supplied")
	}
	raw, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal("private manifest unavailable")
	}
	var cases []struct {
		File              string
		Legacy, Corrected int
	}
	if json.Unmarshal(raw, &cases) != nil || len(cases) == 0 {
		t.Fatal("invalid private manifest")
	}
	binary, err := exec.LookPath("rtl_433")
	if err != nil {
		t.Fatal("rtl_433 file replay unavailable")
	}
	for index, tc := range cases {
		raw, err := os.ReadFile(tc.File)
		var capture ReceptionRecording
		if err != nil || json.Unmarshal(raw, &capture) != nil || capture.Format != "rf433-button-manager-reception-v1" || capture.SampleRate != 250000 {
			t.Fatal("invalid private recording")
		}
		for _, corrected := range []bool{false, true} {
			a, clock := testApp(t)
			r := remote(t, a)
			id := r.Buttons[0].ID
			if err := a.mutate(func(c *Config) error { c.DecoderProfiles = clone(capture.Profiles); return nil }); err != nil {
				t.Fatal("cannot prepare saved profiles")
			}
			learn(t, a, clock, id, []byte(capture.Target.Signature))
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(http.StatusNoContent) }))
			configure(t, a, id, server.URL+"/synthetic-action")
			if err := a.mutate(func(c *Config) error { findButton(c, id).DebounceMS = capture.Target.DebounceMS; return nil }); err != nil {
				t.Fatal(err)
			}
			before := a.Backup()
			a.captureEnabled = true
			_, generation := a.ReceiverPlan(binary)
			a.ReceiverPlanApplied(generation)
			a.SetReceiverFor(generation, receiver.Status{State: "connected", Message: "synthetic replay"})
			clock.advance(2 * time.Second)
			if _, err := a.StartRecording(id); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(t.TempDir(), "private.ook")
			var previous int64
			for packetIndex, packet := range capture.Packets {
				if packet.Cleanup {
					continue
				}
				if err := os.WriteFile(file, []byte(packet.Data), 0600); err != nil {
					t.Fatal("cannot prepare private replay")
				}
				args := []string{"-c", "/dev/null", "-r", file, "-s", "250000", "-F", "json"}
				if len(capture.Protocols) > 0 || capture.FlexDecoder != "" {
					t.Fatal("private regression expects default builtin protocols")
				}
				for _, p := range capture.Profiles {
					args = append(args, "-X", p.FlexDecoder)
				}
				if corrected {
					for _, spec := range receiver.ProfileFramingFallbacks(capture.Profiles) {
						args = append(args, "-X", spec)
					}
				}
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				output, err := exec.CommandContext(ctx, binary, args...).Output()
				cancel()
				if err != nil {
					t.Fatalf("file replay failed, case %d packet %d", index, packetIndex)
				}
				clock.advance(time.Duration(packet.OffsetMS-previous) * time.Millisecond)
				previous = packet.OffsetMS
				for _, line := range strings.Split(string(output), "\n") {
					if !json.Valid([]byte(line)) {
						continue
					}
					a.Receive([]byte(line))
					a.wg.Wait()
				}
			}
			a.StopRecording()
			exported, err := a.ExportRecording()
			if err != nil || !reflect.DeepEqual(exported.FramingFallbacks, receiver.ProfileFramingFallbacks(capture.Profiles)) {
				t.Fatal("recording omits active framing specifications")
			}
			want := tc.Legacy
			if corrected {
				want = tc.Corrected
			}
			if int(calls.Load()) != want || exported.Presses != want {
				t.Fatalf("case %d corrected=%v: got %d isolated actions, want %d", index, corrected, calls.Load(), want)
			}
			after := a.Backup()
			findButton(&after, id).LastReceived = findButton(&before, id).LastReceived
			if !reflect.DeepEqual(before, after) {
				t.Fatal("reception rewrote configuration or mappings")
			}
			server.Close()
			t.Logf("case %d corrected=%v: %d packets -> %d isolated actions; stored configuration preserved", index, corrected, len(capture.Packets), calls.Load())
		}
	}
}
