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
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Optional private evidence, file input only. Nothing from the recording enters
// source, test logs or real webhooks. Removing native messages in the second
// scenario is an explicit dispatch simulation, not a claim that the original
// recording contains a physically missed press or an alternate-only burst.
func TestPrivateEquivalentCaptureToWebhook(t *testing.T) {
	path := os.Getenv("RFB_EQUIVALENT_CAPTURE")
	if path == "" {
		t.Skip("private equivalence recording not supplied")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("private recording unavailable")
	}
	var capture struct {
		Format     string `json:"format"`
		SampleRate int    `json:"sample_rate"`
		Packets    []struct {
			Modulation string `json:"modulation"`
			Cleanup    bool   `json:"cleanup"`
			Data       string `json:"pulse_data"`
		} `json:"packets"`
		Decoded []json.RawMessage `json:"decoded_events"`
	}
	if json.Unmarshal(raw, &capture) != nil || capture.SampleRate != 250000 ||
		(capture.Format != "rf433-button-manager-capture-v1" && capture.Format != "rf-button-manager-capture-v1") {
		t.Fatal("unexpected capture format")
	}
	var target string
	var learningEvent []byte
	for _, event := range capture.Decoded {
		var envelope struct{ Model string }
		_ = json.Unmarshal(event, &envelope)
		if envelope.Model != "Akhan-100F14" {
			continue
		}
		signature, err := Signature(event, defaultFields)
		if err != nil || (target != "" && target != signature) {
			t.Fatal("capture does not identify one consistent target")
		}
		target, learningEvent = signature, event
	}
	if target == "" {
		t.Fatal("capture has no reference target")
	}
	binary, err := exec.LookPath("rtl_433")
	if err != nil {
		t.Fatal("rtl_433 file replay unavailable")
	}
	type burst struct {
		all, alternate []json.RawMessage
	}
	var bursts []burst
	for index, packet := range capture.Packets {
		if packet.Cleanup || packet.Modulation != "OOK" {
			continue
		}
		file := filepath.Join(t.TempDir(), "recording.ook")
		if err := os.WriteFile(file, []byte(packet.Data), 0600); err != nil {
			t.Fatal("cannot prepare private replay")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		output, err := exec.CommandContext(ctx, binary, "-c", "/dev/null", "-r", file, "-s", "250000", "-F", "json").Output()
		cancel()
		if err != nil {
			t.Fatalf("file replay failed for packet %d", index)
		}
		var received burst
		hasTarget := false
		for _, line := range strings.Split(string(output), "\n") {
			if !json.Valid([]byte(line)) {
				continue
			}
			event := json.RawMessage(line)
			received.all = append(received.all, event)
			if signature, _ := Signature(event, defaultFields); signature == target {
				hasTarget = true
			}
			native, _, alternate := decodedRemotePair(event)
			if alternate && string(native) == target {
				received.alternate = append(received.alternate, event)
				hasTarget = true
			}
		}
		if hasTarget {
			bursts = append(bursts, received)
		}
	}
	if len(bursts) < 3 {
		t.Fatal("fewer than three target bursts in private replay")
	}
	for _, onlyAlternate := range []bool{false, true} {
		name := "unaltered_replay"
		if onlyAlternate {
			name = "native_output_removed_simulation"
		}
		t.Run(name, func(t *testing.T) {
			a, clock := testApp(t)
			r := remote(t, a)
			learn(t, a, clock, r.Buttons[0].ID, learningEvent)
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()
			configure(t, a, r.Buttons[0].ID, server.URL+"/synthetic-action")
			expected, frames := 0, 0
			for _, burst := range bursts {
				events := burst.all
				if onlyAlternate {
					events = burst.alternate
				}
				if len(events) == 0 {
					continue
				}
				clock.advance(2 * time.Second)
				for _, event := range events {
					a.Receive(event)
					a.wg.Wait()
					clock.advance(10 * time.Millisecond)
					frames++
				}
				expected++
				if calls.Load() != int32(expected) {
					t.Fatalf("recorded burst %d produced a missed or duplicate local action", expected)
				}
			}
			if expected == 0 {
				t.Fatal("scenario contained no target events")
			}
			t.Logf("%d recorded bursts / %d replay messages -> %d isolated requests; no USB or real webhook", expected, frames, calls.Load())
		})
	}
}
