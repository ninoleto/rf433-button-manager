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

// Optional operator evidence stays outside Git. Replay opens files only, and
// webhook delivery targets a disposable loopback server, never Home Assistant.
func TestPrivateWeakCaptureBuiltinToWebhook(t *testing.T) {
	path := os.Getenv("RFB_WEAK_CAPTURE")
	if path == "" {
		t.Skip("private other-room recording not supplied")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("private recording unavailable")
	}
	var capture struct {
		SampleRate int `json:"sample_rate"`
		Packets    []struct {
			Modulation string `json:"modulation"`
			Cleanup    bool   `json:"cleanup"`
			Data       string `json:"pulse_data"`
		} `json:"packets"`
		Decoded []json.RawMessage `json:"decoded_events"`
	}
	if json.Unmarshal(raw, &capture) != nil || capture.SampleRate != 250000 {
		t.Fatal("unexpected capture format")
	}
	binary, err := exec.LookPath("rtl_433")
	if err != nil {
		t.Fatal(err)
	}
	var bursts [][]json.RawMessage
	for i, packet := range capture.Packets {
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
			t.Fatalf("file replay failed for packet %d", i)
		}
		var events []json.RawMessage
		for _, line := range strings.Split(string(output), "\n") {
			var envelope struct {
				Model string `json:"model"`
			}
			if json.Unmarshal([]byte(line), &envelope) == nil && envelope.Model == "Akhan-100F14" {
				events = append(events, json.RawMessage(line))
			}
		}
		if len(events) > 0 {
			bursts = append(bursts, events)
		}
	}
	if len(bursts) != 9 {
		t.Fatalf("expected nine recorded remote bursts, decoded %d", len(bursts))
	}
	a, clock, remote := captureApp(t)
	startCapture(t, a, remote.Buttons[0].ID)
	// Reproduce the old first-frame choice with a real custom-profile event,
	// followed by the native decoder's output for that same intended remote.
	for _, event := range capture.Decoded {
		var envelope struct {
			Model string `json:"model"`
		}
		_ = json.Unmarshal(event, &envelope)
		if strings.HasPrefix(envelope.Model, "RFB-") {
			a.Receive(event)
			break
		}
	}
	for i := 0; i < 3; i++ {
		if i > 0 {
			clock.advance(2 * time.Second)
			if _, err := a.ArmLearning(); err != nil {
				t.Fatal(err)
			}
		}
		a.Receive(bursts[i][0])
	}
	if a.learning == nil || a.learning.Presses != 3 || a.learning.Profile != nil {
		t.Fatal("native decoder did not complete three-arm learning")
	}
	if err := a.ConfirmLearning(); err != nil {
		t.Fatal(err)
	}
	_, normalGeneration := a.ReceiverPlan(binary)
	a.ReceiverPlanApplied(normalGeneration)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(http.StatusNoContent) }))
	defer server.Close()
	configure(t, a, remote.Buttons[0].ID, server.URL+"/synthetic-action")
	_, generation := a.ReceiverPlan(binary)
	a.ReceiverPlanApplied(generation)
	frames := 0
	for index, events := range bursts {
		clock.advance(2 * time.Second)
		for _, event := range events {
			a.Receive(event)
			frames++
		}
		a.wg.Wait()
		if calls.Load() != int32(index+1) {
			t.Fatal("missed or duplicate local action for a recorded burst")
		}
	}
	t.Logf("%d recorded bursts / %d native frames produced %d local actions after explicit three-arm learning; no USB or real webhook used", len(bursts), frames, calls.Load())
}
