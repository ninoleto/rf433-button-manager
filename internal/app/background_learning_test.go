package app

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ninoleto/rf-button-manager/internal/receiver"
)

// Reproduces the operator's 0/3 failure: three excluded FSK packets and one
// complete OOK capture that produced no decoder candidate after two replays.
func TestBackgroundCaptureDoesNotTerminateLearning(t *testing.T) {
	a, c, r := captureApp(t)
	a.analyzeCapture = func(context.Context, receiver.AnalysisRequest) (receiver.AnalysisResult, error) {
		return receiver.AnalysisResult{Diagnostics: receiver.LearningDiagnostics{Replays: 2}}, &receiver.LearningError{Code: "unsupported", Message: "No supported candidate was found in this recording"}
	}
	generation := startCapture(t, a, r.Buttons[0].ID)
	deadline, window := a.learning.ExpiresAt, a.learning.WindowExpiresAt
	for i := 0; i < 3; i++ {
		a.ReceivePulse(generation, syntheticFSK(t))
	}
	capturePress(t, a, c, generation)
	if a.learning == nil {
		t.Fatalf("background capture ended the session: %+v", a.learningNotice)
	}
	if a.learning.Stage != "waiting" || a.learning.Presses != 0 {
		t.Fatalf("background packet became a press: %+v", a.learning)
	}
	d := a.learning.Diagnostics
	if d.ObservedPackets != 4 || d.FSKPackets != 3 || d.Packets != 1 || d.Replays != 2 || d.Candidates != 0 || d.RejectedCaptures != 1 || len(d.RecentRejections) != 1 || d.FailureCode != "" {
		t.Fatalf("background evidence lost: %+v", d)
	}
	if a.learning.Message != "Unrecognized signal ignored; waiting for your button" || a.learning.ExpiresAt != deadline || a.learning.WindowExpiresAt != window || a.receiverGeneration != generation || !a.captureSuppressed {
		t.Fatal("rejected capture extended the deadline, restored the receiver, or unpaused actions")
	}
}

func TestBackgroundOnlyExpiresWithoutPressesAndWithBoundedWork(t *testing.T) {
	for _, kind := range []string{"FSK", "OOK", "analysis_budget"} {
		t.Run(kind, func(t *testing.T) {
			a, c, r := captureApp(t)
			before := a.Backup()
			calls := 0
			a.analyzeCapture = func(context.Context, receiver.AnalysisRequest) (receiver.AnalysisResult, error) {
				calls++
				return receiver.AnalysisResult{Diagnostics: receiver.LearningDiagnostics{Replays: 2}}, &receiver.LearningError{Code: "unsupported", Message: "No stable identity in this capture"}
			}
			generation := startCapture(t, a, r.Buttons[0].ID)
			armed, expires := a.learning.armedAt, a.learning.expires
			loops := 3
			if kind == "analysis_budget" {
				loops = 6
			}
			for i := 0; i < loops; i++ {
				a.ReceivePulse(generation, syntheticFSK(t))
				if kind != "FSK" {
					capturePress(t, a, c, generation)
				} else {
					c.advance(time.Second)
					a.learningTick()
				}
				if a.learning == nil || a.learning.Presses != 0 || !a.learning.armedAt.Equal(armed) || !a.learning.expires.Equal(expires) {
					t.Fatal("background changed a press/deadline or terminated early")
				}
			}
			if calls > maxWindowAnalyses {
				t.Fatal("analysis work exceeded the window budget", calls)
			}
			if kind == "analysis_budget" && (calls != 6 || strings.Contains(a.learning.Message, "analysis budget") || a.learning.WindowAnalyses != 6) {
				t.Fatal("background prematurely exhausted the window")
			}
			if _, err := a.ArmLearning(); err == nil {
				t.Fatal("background rejection permitted a new press window")
			}
			if err := a.ConfirmLearning(); err == nil {
				t.Fatal("background could be saved")
			}
			c.advance(armed.Add(learningPressWindow).Sub(c.now()) + time.Millisecond)
			a.learningTick()
			l := a.learningNotice
			if a.learning != nil || l == nil || l.Stage != "press_timeout" || l.Presses != 0 || l.Candidate != "" || l.Profile != nil || l.Diagnostics.RejectedCaptures != calls || !reflect.DeepEqual(before, a.Backup()) {
				t.Fatalf("background-only expiry: %+v", l)
			}
			events, err := a.store.Events(150)
			if err != nil {
				t.Fatal(err)
			}
			rejected := 0
			for _, event := range events {
				if event.Kind == "learning_capture" {
					rejected++
				}
			}
			if rejected != calls {
				t.Fatal("per-capture evidence not persisted", rejected, calls)
			}
			_, normal := a.ReceiverPlan("rtl_433")
			a.ReceiverPlanApplied(normal)
			if a.captureSuppressed {
				t.Fatal("timeout did not finish receiver restoration")
			}
		})
	}
}

func TestBackgroundCannotAutomaticallyArmMatchingPressWindows(t *testing.T) {
	a, c, r := captureApp(t)
	generation := startCapture(t, a, r.Buttons[0].ID)
	capturePress(t, a, c, generation)
	candidate := a.learning.Candidate
	for i := 0; i < 3; i++ {
		capturePress(t, a, c, generation)
	}
	if a.learning.Presses != 1 || a.learning.Stage != "captured" || a.learning.Diagnostics.AnalysisAttempts != 1 {
		t.Fatal("repeated matching background auto-armed another window")
	}
	if err := a.ConfirmLearning(); err == nil {
		t.Fatal("saved without three armed windows")
	}
	if _, err := a.ArmLearning(); err != nil {
		t.Fatal(err)
	}
	a.analyzeCapture = func(context.Context, receiver.AnalysisRequest) (receiver.AnalysisResult, error) {
		return receiver.AnalysisResult{Raw: signalB}, nil
	}
	capturePress(t, a, c, generation)
	if a.learning.Presses != 1 || a.learning.Candidate != candidate || a.learning.Stage != "waiting" || a.learning.Diagnostics.RejectionCode != "inconsistent" {
		t.Fatal("unrelated signal changed verified identity")
	}
	a.analyzeCapture = func(context.Context, receiver.AnalysisRequest) (receiver.AnalysisResult, error) {
		return receiver.AnalysisResult{Raw: signalA}, nil
	}
	capturePress(t, a, c, generation)
	if a.learning.Presses != 2 {
		t.Fatal("matching capture could not follow unrelated traffic in same window")
	}
	if _, err := a.ArmLearning(); err != nil {
		t.Fatal(err)
	}
	capturePress(t, a, c, generation)
	if a.learning.Presses != 3 || a.learning.Stage != "ready" {
		t.Fatal("three separately armed matches did not complete")
	}
	if a.Backup().Remotes[0].Buttons[0].Signature != "" {
		t.Fatal("mapping saved without confirmation")
	}
	if err := a.ConfirmLearning(); err != nil {
		t.Fatal(err)
	}
	if a.Backup().Remotes[0].Buttons[0].Enabled {
		t.Fatal("learned action enabled")
	}
}

func TestBackgroundThenLearningWithActualOfflineRTL433(t *testing.T) {
	binary, err := exec.LookPath("rtl_433")
	if err != nil {
		t.Skip("requires installed rtl_433; file replay only")
	}
	a, c, r := captureApp(t)
	a.rtlBinary = binary
	a.captureTempDir = t.TempDir()
	a.analyzeCapture = receiver.AnalyzeCapture
	// Restrict built-ins in this isolated test so the synthetic PWM fixture must
	// exercise measured profile discovery instead of an incidental built-in match.
	settings := a.Settings()
	settings.Protocols = []int{1}
	if err = a.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	generation := startCapture(t, a, r.Buttons[0].ID)
	for i := 0; i < 3; i++ {
		a.ReceivePulse(generation, syntheticFSK(t))
	}
	background := []byte(";pulse data\n;version 1\n;timescale 1us\n;ook 1 pulses\n500 10000\n;end\n")
	c.advance(time.Second)
	a.ReceivePulse(generation, receiver.PulsePacket{OOK: true, Pulses: 1, Data: background})
	c.advance(time.Second)
	a.learningTick()
	if a.learning == nil || a.learning.Presses != 0 || a.learning.Diagnostics.Replays != 0 || a.learning.Diagnostics.Candidates != 0 || a.learning.Diagnostics.RejectionCode != "short_burst" {
		t.Fatalf("actual unrecognized replay did not keep listening: active=%+v ended=%+v", a.learning, a.learningNotice)
	}
	raw, err := os.ReadFile("../receiver/testdata/synthetic-pwm.ook")
	if err != nil {
		t.Fatal(err)
	}
	for press := 0; press < 3; press++ {
		c.advance(time.Second)
		a.ReceivePulse(generation, receiver.PulsePacket{OOK: true, Pulses: 144, Data: raw})
		c.advance(time.Second)
		a.learningTick()
		if a.learning == nil || a.learning.Presses != press+1 {
			t.Fatalf("synthetic validated capture failed: %+v", a.learningNotice)
		}
		if press < 2 {
			if _, err = a.ArmLearning(); err != nil {
				t.Fatal(err)
			}
			if press == 0 {
				profileID := a.learning.Profile.ID
				c.advance(time.Second)
				a.ReceivePulse(generation, receiver.PulsePacket{OOK: true, Pulses: 1, Data: background})
				c.advance(time.Second)
				a.learningTick()
				if a.learning == nil || a.learning.Stage != "waiting" || a.learning.Presses != 1 || a.learning.Profile.ID != profileID || a.learning.Diagnostics.RejectionCode != "short_burst" {
					t.Fatal("background failed to decode with selected profile and ended or changed learning")
				}
			}
		}
	}
	if err = a.ConfirmLearning(); err != nil {
		t.Fatal(err)
	}
	after := a.Backup()
	if len(after.DecoderProfiles) != 1 || after.DecoderProfiles[0].ValidatedPresses != 3 || after.Remotes[0].Buttons[0].Signature == "" || after.Remotes[0].Buttons[0].Enabled {
		t.Fatal("validated profile and disabled mapping not saved")
	}
}

func TestPressDeadlineBoundsAnalysisAndRejectsLateSuccess(t *testing.T) {
	a, c, r := captureApp(t)
	started := make(chan time.Duration, 1)
	done := make(chan struct{})
	a.analyzeCapture = func(ctx context.Context, _ receiver.AnalysisRequest) (receiver.AnalysisResult, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			started <- time.Hour
		} else {
			started <- time.Until(deadline)
		}
		<-ctx.Done()
		return receiver.AnalysisResult{Raw: signalA, Diagnostics: receiver.LearningDiagnostics{Replays: 1}}, nil
	}
	generation := startCapture(t, a, r.Buttons[0].ID)
	c.advance(18 * time.Second)
	a.ReceivePulse(generation, receiver.PulsePacket{OOK: true, Data: []byte("bounded synthetic recording")})
	c.advance(1100 * time.Millisecond)
	go func() { a.learningTick(); close(done) }()
	budget := <-started
	if budget <= 0 || budget > time.Second {
		t.Fatal("analysis was not bounded by the original press deadline", budget)
	}
	c.advance(time.Second)
	a.State() // Expires while analysis is active and cancels its context.
	<-done
	if a.learning != nil || a.learningNotice.Stage != "press_timeout" || a.learningNotice.Presses != 0 || a.learningNotice.Diagnostics.Replays != 1 {
		t.Fatal("late analysis advanced an expired window")
	}
	if err := a.ConfirmLearning(); err == nil {
		t.Fatal("late success could be confirmed")
	}
}

func TestAnalysisProcessFailureStillEndsLearning(t *testing.T) {
	for _, typed := range []bool{false, true} {
		a, c, r := captureApp(t)
		a.analyzeCapture = func(context.Context, receiver.AnalysisRequest) (receiver.AnalysisResult, error) {
			if typed {
				return receiver.AnalysisResult{}, &receiver.LearningError{Code: "analysis_error", Message: "Offline decoder process failed"}
			}
			return receiver.AnalysisResult{}, errors.New("private process diagnostic")
		}
		generation := startCapture(t, a, r.Buttons[0].ID)
		capturePress(t, a, c, generation)
		if a.learning != nil || a.learningNotice.Stage != "analysis_error" || strings.Contains(a.learningNotice.Message, "private") {
			t.Fatal("process failure was ignored or disclosed private data")
		}
	}
}
