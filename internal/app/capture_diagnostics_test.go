package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ninoleto/rf-button-manager/internal/receiver"
)

func syntheticFSK(t *testing.T) receiver.PulsePacket {
	t.Helper()
	raw, err := os.ReadFile("../receiver/testdata/synthetic-pwm.ook")
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.ReplaceAll(string(raw), ";ook", ";fsk"))
	return receiver.PulsePacket{OOK: false, Pulses: 144, Data: raw, ObservedBytes: len(raw)}
}

// rtl_433 23.11 emits FSK on -w OOK:- too. The old code immediately ended
// learning as "unsupported" with zero counters: this is the reported regression.
func TestLiveFSKIsCountedWithoutEndingOOKLearning(t *testing.T) {
	a, c, r := captureApp(t)
	generation := startCapture(t, a, r.Buttons[0].ID)
	packet := syntheticFSK(t)
	a.ReceivePulse(generation, packet)
	if a.learning == nil {
		t.Fatalf("valid FSK packet ended learning with hidden evidence: %+v", a.learningNotice)
	}
	d := a.learning.Diagnostics
	if d.ObservedPackets != 1 || d.Packets != 0 || d.RejectedPackets != 1 || d.FSKPackets != 1 || d.Pulses != 144 || d.Bytes != len(packet.Data) || d.Replays != 0 || d.RejectionCode != "unsupported_modulation" || a.learning.Stage != "waiting" {
		t.Fatalf("FSK evidence missing or accepted as a candidate: %+v", d)
	}
	capturePress(t, a, c, generation)
	if a.learning == nil || a.learning.Presses != 1 || a.learning.Diagnostics.Packets != 1 {
		t.Fatal("unrelated FSK prevented subsequent OOK verification")
	}
	if err := a.ConfirmLearning(); err == nil {
		t.Fatal("one OOK press bypassed three-press proof")
	}
	if a.Backup().Remotes[0].Buttons[0].Enabled {
		t.Fatal("action was enabled")
	}
}

func TestCaptureFailuresDistinguishEvidence(t *testing.T) {
	for _, kind := range []string{"empty_stream", "JSON_only", "FSK_only", "partial_packet", "malformed_packet", "empty_packet", "read_error"} {
		t.Run(kind, func(t *testing.T) {
			a, c, r := captureApp(t)
			before := a.Backup()
			generation := startCapture(t, a, r.Buttons[0].ID)
			stage, code := "press_timeout", "press_window_timeout"
			switch kind {
			case "JSON_only":
				a.ReceivePulse(generation, receiver.PulsePacket{Progress: true, Stream: receiver.PulseStream{Bytes: 120, Lines: 1, JSONRecords: 1}})
			case "FSK_only":
				a.ReceivePulse(generation, syntheticFSK(t))
			case "partial_packet":
				a.ReceivePulse(generation, receiver.PulsePacket{Progress: true, Stream: receiver.PulseStream{Bytes: 90, Lines: 6, Headers: 1, PulseRows: 1, Active: true, Expected: 2, Rows: 1}})
			case "malformed_packet":
				a.ReceivePulse(generation, receiver.PulsePacket{OOK: true, Pulses: 1, ObservedBytes: 53, ErrorCode: "invalid_timing", Error: "Pulse row 2 contains a non-integer or negative duration"})
				stage, code = "malformed_capture", "invalid_timing"
			case "empty_packet":
				a.ReceivePulse(generation, receiver.PulsePacket{OOK: true, ObservedBytes: 20, ErrorCode: "empty_packet", Error: "Pulse packet ended without any timing rows"})
				stage, code = "no_capture", "empty_packet"
			case "read_error":
				a.ReceivePulse(generation, receiver.PulsePacket{ErrorCode: "stream_read_error", Error: "rtl_433 stdout exceeded the safe line size or became unreadable"})
				stage, code = "capture_error", "stream_read_error"
			}
			c.advance(21 * time.Second)
			a.learningTick()
			l := a.learningNotice
			if a.learning != nil || l == nil || l.Stage != stage || l.Diagnostics.FailureCode != code || l.Diagnostics.FailureReason == "" || l.Diagnostics.Replays != 0 || l.Diagnostics.Candidates != 0 {
				t.Fatalf("incorrect failure classification: %+v", l)
			}
			if kind == "empty_stream" && l.Diagnostics.ObservedPackets != 0 {
				t.Fatal("invented RF evidence")
			}
			if kind == "JSON_only" && l.Diagnostics.Stream.JSONRecords != 1 {
				t.Fatal("JSON-only evidence lost")
			}
			if kind == "malformed_packet" && (l.Diagnostics.Pulses != 1 || l.Diagnostics.Bytes != 53 || l.Diagnostics.MalformedPackets != 1) {
				t.Fatal("rejected packet counters lost")
			}
			if !reflect.DeepEqual(before, a.Backup()) {
				t.Fatal("failed capture modified configuration")
			}
			plan, next := a.ReceiverPlan("rtl_433")
			if plan.CapturePulses || !a.captureSuppressed {
				t.Fatal("normal restoration not suppressed")
			}
			a.ReceiverPlanApplied(next)
			if a.captureSuppressed {
				t.Fatal("normal restoration did not complete")
			}
		})
	}
}

func TestCaptureCleanupPreservesCountersReasonAndReceiver(t *testing.T) {
	a, _, r := captureApp(t)
	generation := startCapture(t, a, r.Buttons[0].ID)
	a.ReceivePulse(generation, receiver.PulsePacket{OOK: true, Pulses: 1, ObservedBytes: 52, ErrorCode: "invalid_timing", Error: "Pulse row 2 contains a non-integer or negative duration", Stream: receiver.PulseStream{Bytes: 100, Lines: 8, Headers: 1, Completed: 1}})
	l := a.learningNotice
	if l == nil {
		t.Fatal("no failed attempt")
	}
	captured := l.CaptureReceiver
	// Buffered old-child records arrive while normal reception is being restored.
	a.ReceivePulse(generation, receiver.PulsePacket{Cleanup: true, OOK: true, Pulses: 1, ObservedBytes: 30, ErrorCode: "missing_end", Error: "Pulse stream stopped before the packet's ;end boundary", Stream: receiver.PulseStream{Bytes: 130, Lines: 10, Headers: 2, Completed: 1}})
	a.ReceivePulse(generation, receiver.PulsePacket{Progress: true, Final: true, Stream: receiver.PulseStream{Bytes: 130, Lines: 10, Headers: 2, Completed: 1, Active: true, Rows: 1, Expected: 2, Closed: true}})
	_, next := a.ReceiverPlan("rtl_433")
	a.ReceiverPlanApplied(next)
	a.SetReceiverFor(next, receiver.Status{State: "connected", Message: "restored normal samples"})
	a.SetReceiverFor(generation, receiver.Status{State: "disconnected", Message: "old capture stopped"})
	l = a.learningNotice
	d := l.Diagnostics
	if d.ObservedPackets != 2 || d.RejectedPackets != 2 || d.MalformedPackets != 2 || d.DiscardedPackets != 1 || d.Pulses != 2 || d.Bytes != 82 || d.Stream.Bytes != 130 || !d.Stream.Closed || !d.Stream.Active {
		t.Fatalf("cleanup discarded evidence: %+v", d)
	}
	if d.FailureCode != "invalid_timing" || !strings.Contains(d.FailureReason, "row 2") || d.RejectionCode != "missing_end" {
		t.Fatal("cleanup overwrote original rejection", d)
	}
	if l.CaptureReceiver != captured || a.receiver == captured || a.receiverMode != "normal" || a.receiverApplied != next || l.CaptureGeneration != generation {
		t.Fatal("restored receiver misrepresented as capture receiver")
	}
	if err := a.ConfirmLearning(); err == nil {
		t.Fatal("cleanup data became a candidate")
	}
	// Persistent event diagnostics contain neither raw RF nor webhook settings.
	events, err := a.store.Events(150)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.Kind != "learning_diagnostics" {
			continue
		}
		var saved struct {
			Diagnostics receiver.LearningDiagnostics `json:"diagnostics"`
		}
		if err = json.Unmarshal(event.Data, &saved); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(saved.Diagnostics, d) || strings.Contains(string(event.Data), "webhook") {
			t.Fatal("persisted diagnostics mismatch")
		}
		return
	}
	t.Fatal("cleanup evidence not persisted")
}

func TestCaptureCancelRetainsEvidenceAcrossStoreReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manager.db")
	s, config, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(s, config, false)
	if err != nil {
		t.Fatal(err)
	}
	r := remote(t, a)
	a.captureEnabled = true
	generation := startCapture(t, a, r.Buttons[0].ID)
	a.ReceivePulse(generation, syntheticFSK(t))
	if err = a.CancelLearning(); err != nil {
		t.Fatal(err)
	}
	a.ReceivePulse(generation, receiver.PulsePacket{Progress: true, Final: true, Stream: receiver.PulseStream{Bytes: 2000, Lines: 160, Headers: 1, Completed: 1, Closed: true}})
	if a.learningNotice != nil || a.terminalCapture.Stage != "cancelled" || a.terminalCapture.Diagnostics.FSKPackets != 1 {
		t.Fatal("cancel lost diagnostics")
	}
	a.Close()
	s.Close()
	s, config, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a, err = New(s, config, false)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if config.Remotes[0].Buttons[0].Signature != "" || config.Remotes[0].Buttons[0].Enabled {
		t.Fatal("cancel persisted a candidate or enabled action")
	}
	for _, event := range a.events {
		if event.Kind == "learning_diagnostics" && strings.Contains(string(event.Data), `"fsk_packets":1`) {
			return
		}
	}
	t.Fatal("capture diagnostics did not survive reopen")
}
