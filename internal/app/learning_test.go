package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/ninoleto/rf-button-manager/internal/receiver"
)

func captureApp(t *testing.T) (*App, *clock, Remote) {
	t.Helper()
	a, c := testApp(t)
	r := remote(t, a)
	a.captureEnabled = true
	a.analyzeCapture = func(ctx context.Context, request receiver.AnalysisRequest) (receiver.AnalysisResult, error) {
		return receiver.AnalysisResult{Raw: signalA}, nil
	}
	return a, c, r
}

func startCapture(t *testing.T, a *App, id string) uint64 {
	t.Helper()
	if _, err := a.StartLearning(id, nil); err != nil {
		t.Fatal(err)
	}
	plan, generation := a.ReceiverPlan("rtl_433")
	if !plan.CapturePulses {
		t.Fatal("learning did not request managed capture")
	}
	a.ReceiverPlanApplied(generation)
	a.SetReceiverFor(generation, receiver.Status{State: "connected", Message: "samples"})
	return generation
}

func capturePress(t *testing.T, a *App, c *clock, generation uint64) {
	t.Helper()
	c.advance(1100 * time.Millisecond)
	a.ReceivePulse(generation, receiver.PulsePacket{OOK: true, Pulses: 144, Data: captureFixture(t)})
	c.advance(1100 * time.Millisecond)
	a.learningTick()
}

func TestCaptureRequiresThreeSeparateConfirmedPresses(t *testing.T) {
	a, c, r := captureApp(t)
	generation := startCapture(t, a, r.Buttons[0].ID)
	a.Receive([]byte(`{"model":"Weather","id":1,"temperature_C":22}`))
	if a.learning.Presses != 0 {
		t.Fatal("sensor-only output became a button")
	}
	for i := 0; i < 3; i++ {
		capturePress(t, a, c, generation)
		if a.learning == nil || a.learning.Presses != i+1 {
			t.Fatalf("press %d: %+v", i, a.learningNotice)
		}
		for n := 0; n < 10; n++ {
			a.ReceivePulse(generation, receiver.PulsePacket{OOK: true, Data: []byte("repeat")})
		}
		if a.learning.Presses != i+1 {
			t.Fatal("held button counted as separate presses")
		}
		if i < 2 {
			if err := a.ConfirmLearning(); err == nil {
				t.Fatal("saved before three presses")
			}
			if _, err := a.ArmLearning(); err == nil {
				t.Fatal("armed while transmission was active")
			}
			c.advance(time.Second)
			if _, err := a.ArmLearning(); err != nil {
				t.Fatal(err)
			}
		}
	}
	before := a.Backup()
	if before.Remotes[0].Buttons[0].Signature != "" {
		t.Fatal("mapping saved before explicit confirmation")
	}
	if err := a.ConfirmLearning(); err != nil {
		t.Fatal(err)
	}
	after := a.Backup()
	if after.Remotes[0].Buttons[0].Signature == "" || after.Remotes[0].Buttons[0].Enabled {
		t.Fatal("save did not keep actions disabled")
	}
	if !a.captureSuppressed {
		t.Fatal("actions resumed before receiver restoration")
	}
	plan, normalGeneration := a.ReceiverPlan("rtl_433")
	if plan.CapturePulses {
		t.Fatal("capture plan left active")
	}
	a.ReceiverPlanApplied(generation)
	if !a.captureSuppressed {
		t.Fatal("stale acknowledgement resumed actions")
	}
	a.ReceiverPlanApplied(normalGeneration)
	if a.captureSuppressed {
		t.Fatal("normal receiver acknowledgement did not complete cleanup")
	}
}

func TestCaptureInconsistentSignalsPreserveExistingMapping(t *testing.T) {
	a, c := testApp(t)
	r := remote(t, a)
	learn(t, a, c, r.Buttons[0].ID, signalB)
	before := a.Backup()
	a.captureEnabled = true
	calls := 0
	a.analyzeCapture = func(context.Context, receiver.AnalysisRequest) (receiver.AnalysisResult, error) {
		calls++
		raw := signalA
		if calls == 2 {
			raw = signalB
		}
		return receiver.AnalysisResult{Raw: raw}, nil
	}
	generation := startCapture(t, a, r.Buttons[0].ID)
	capturePress(t, a, c, generation)
	if _, err := a.ArmLearning(); err != nil {
		t.Fatal(err)
	}
	capturePress(t, a, c, generation)
	if a.learning == nil || a.learning.Stage != "waiting" || a.learning.Presses != 1 || a.learning.Diagnostics.RejectedCaptures != 1 || a.learning.Diagnostics.RejectionCode != "inconsistent" {
		t.Fatal("mismatched background capture terminated learning or changed the candidate")
	}
	if !reflect.DeepEqual(before, a.Backup()) {
		t.Fatal("failed attempt changed existing data")
	}
	if err := a.ConfirmLearning(); err == nil {
		t.Fatal("failed attempt could be saved")
	}
	c.advance(21 * time.Second)
	a.learningTick()
	if a.learningNotice == nil || a.learningNotice.Stage != "press_timeout" || !reflect.DeepEqual(before, a.Backup()) {
		t.Fatal("mismatch timeout did not preserve the previous mapping")
	}
}

func TestCaptureTimeoutAndReceiverUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name  string
		ready bool
		delay time.Duration
		stage string
	}{
		{"waiting", true, 21 * time.Second, "press_timeout"},
		{"receiver", false, 31 * time.Second, "receiver_unavailable"},
		{"session", true, 181 * time.Second, "timeout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, c, r := captureApp(t)
			if tc.ready {
				startCapture(t, a, r.Buttons[0].ID)
			} else if _, err := a.StartLearning(r.Buttons[0].ID, nil); err != nil {
				t.Fatal(err)
			}
			c.advance(tc.delay)
			a.learningTick() // No browser polling required for cleanup.
			if a.learning != nil || a.learningNotice == nil || a.learningNotice.Stage != tc.stage {
				t.Fatalf("missing timeout: %+v", a.learningNotice)
			}
			plan, generation := a.ReceiverPlan("rtl_433")
			if plan.CapturePulses {
				t.Fatal("timeout left capture on")
			}
			a.ReceiverPlanApplied(generation)
			if a.captureSuppressed {
				t.Fatal("timeout did not restore normal reception")
			}
		})
	}
}

func TestCaptureCancellationWaitsForAnalysisCleanup(t *testing.T) {
	a, c, r := captureApp(t)
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	a.analyzeCapture = func(ctx context.Context, _ receiver.AnalysisRequest) (receiver.AnalysisResult, error) {
		close(started)
		<-ctx.Done()
		<-release
		return receiver.AnalysisResult{Raw: signalA}, nil // Deliberately late result.
	}
	generation := startCapture(t, a, r.Buttons[0].ID)
	c.advance(time.Second)
	a.ReceivePulse(generation, receiver.PulsePacket{OOK: true, Data: []byte("bounded capture")})
	c.advance(time.Second)
	go func() { a.learningTick(); close(done) }()
	<-started
	if err := a.CancelLearning(); err != nil {
		t.Fatal(err)
	}
	_, restoreGeneration := a.ReceiverPlan("rtl_433")
	a.ReceiverPlanApplied(restoreGeneration)
	if err := a.TestButton(r.Buttons[0].ID); err == nil || err.Error() != "webhook tests are paused during learning" {
		t.Fatal("test action not suppressed during cleanup", err)
	}
	if _, err := a.StartLearning(r.Buttons[0].ID, nil); err == nil {
		t.Fatal("overlapping analysis accepted")
	}
	close(release)
	<-done
	if a.learning != nil || a.captureSuppressed || a.Backup().Remotes[0].Buttons[0].Signature != "" {
		t.Fatal("late analysis changed state or failed to release suppression")
	}
}

func TestCaptureFailureBoundsAndDiagnosticNotice(t *testing.T) {
	a, c, r := captureApp(t)
	generation := startCapture(t, a, r.Buttons[0].ID)
	a.ReceivePulse(generation, receiver.PulsePacket{OOK: true, ErrorCode: "invalid_timing", Error: "Pulse row 1 contains a non-integer or negative duration", ObservedBytes: 30})
	if a.learningNotice == nil || a.learningNotice.Stage != "malformed_capture" {
		t.Fatal("malformed capture had no explanation")
	}
	_, generation = a.ReceiverPlan("rtl_433")
	a.ReceiverPlanApplied(generation)
	startCapture(t, a, r.Buttons[0].ID)
	generation = a.receiverGeneration
	a.ReceivePulse(generation, receiver.PulsePacket{OOK: true, Data: make([]byte, receiver.MaxCaptureBytes+1)})
	if a.learningNotice == nil || a.learningNotice.Stage != "capture_limit" {
		t.Fatal("unbounded capture accepted")
	}
	_, generation = a.ReceiverPlan("rtl_433")
	a.ReceiverPlanApplied(generation)
	startCapture(t, a, r.Buttons[0].ID)
	a.analyzeCapture = func(context.Context, receiver.AnalysisRequest) (receiver.AnalysisResult, error) {
		return receiver.AnalysisResult{}, errors.New("private internal failure")
	}
	capturePress(t, a, c, a.receiverGeneration)
	if a.learningNotice == nil || a.learningNotice.Stage != "analysis_error" || a.learningNotice.Message == "private internal failure" {
		t.Fatal("failure was silent or disclosed internals")
	}
}

func TestCaptureReceiverLossAndHeldTransmissionRestoreReception(t *testing.T) {
	for _, kind := range []string{"receiver_loss", "held_transmission"} {
		t.Run(kind, func(t *testing.T) {
			a, c, r := captureApp(t)
			generation := startCapture(t, a, r.Buttons[0].ID)
			if kind == "receiver_loss" {
				a.SetReceiverFor(generation, receiver.Status{State: "disconnected", Message: "receiver removed"})
				if a.learningNotice == nil || a.learningNotice.Stage != "receiver_unavailable" {
					t.Fatal("loss had no actionable notice")
				}
			} else {
				a.ReceivePulse(generation, receiver.PulsePacket{OOK: true, Data: []byte("pulse data")})
				c.advance(7 * time.Second)
				a.ReceivePulse(generation, receiver.PulsePacket{OOK: true, Data: []byte("still transmitting")})
				a.learningTick()
				if a.learningNotice == nil || a.learningNotice.Stage != "capture_limit" {
					t.Fatal("prolonged transmission not bounded")
				}
			}
			plan, next := a.ReceiverPlan("rtl_433")
			if plan.CapturePulses || !a.captureSuppressed {
				t.Fatal("failed attempt did not request suppressed restoration")
			}
			a.ReceivePulse(generation, receiver.PulsePacket{OOK: true, Data: []byte("stale pulse")})
			a.ReceiverPlanApplied(next)
			if a.captureSuppressed || a.learning != nil {
				t.Fatal("normal reception did not resume")
			}
		})
	}
}

func TestCaptureRealReplayPersistenceAndProfileReuse(t *testing.T) {
	binary, err := exec.LookPath("rtl_433")
	if err != nil {
		t.Skip("offline replay requires rtl_433")
	}
	a, c, r := captureApp(t)
	a.rtlBinary = binary
	a.captureTempDir = t.TempDir()
	a.analyzeCapture = receiver.AnalyzeCapture
	settingsBefore := a.Settings()
	settingsBefore.Protocols = []int{1}
	if err = a.UpdateSettings(settingsBefore); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../receiver/testdata/synthetic-pwm.ook")
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 2; index++ {
		generation := startCapture(t, a, r.Buttons[index].ID)
		for n := 0; n < 3; n++ {
			c.advance(time.Second)
			a.ReceivePulse(generation, receiver.PulsePacket{OOK: true, Pulses: 144, Data: raw})
			c.advance(time.Second)
			a.learningTick()
			if a.learning == nil {
				t.Fatalf("replay failed: %+v", a.learningNotice)
			}
			if index == 1 && n == 0 && a.learning.Diagnostics.Source != "existing remote profile" {
				t.Fatal("later button did not reuse profile", a.learning.Diagnostics)
			}
			if n < 2 {
				if _, err = a.ArmLearning(); err != nil {
					t.Fatal(err)
				}
			}
		}
		if index == 0 {
			if err = a.ConfirmLearning(); err != nil {
				t.Fatal(err)
			}
		} else {
			// The same fixture represents the SAME physical button: don't duplicate it.
			if err = a.ConfirmLearning(); err == nil {
				t.Fatal("existing mapping collision accepted")
			}
			a.CancelLearning()
		}
		_, generation = a.ReceiverPlan(binary)
		a.ReceiverPlanApplied(generation)
	}
	config := a.Backup()
	if len(config.DecoderProfiles) != 1 || config.DecoderProfiles[0].ValidatedPresses != 3 || config.Remotes[0].Buttons[0].DecoderProfileID != config.DecoderProfiles[0].ID {
		t.Fatal("validated profile not saved atomically with mapping")
	}
	path := filepath.Join(t.TempDir(), "manager.db")
	s, _, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Save(config); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, reopened, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if !reflect.DeepEqual(config, reopened) {
		t.Fatal("profile/mapping did not survive reopen")
	}
	// Runtime receives repeated rows and must normalize them to the same mapping.
	profile := config.DecoderProfiles[0]
	live, _ := json.Marshal(map[string]any{"model": profile.Model, "codes": []string{"{24}a9b2c5", "{24}a9b2c5"}})
	a.Receive(live)
	if a.Backup().Remotes[0].Buttons[0].LastReceived == "" {
		t.Fatal("saved profile did not match normal decoded reception")
	}
	settings := a.Settings()
	settings.ReceiverSerial = "test-serial"
	if err = a.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	if len(a.Backup().DecoderProfiles) != 1 {
		t.Fatal("legacy settings update dropped profiles")
	}
}

func TestSchemaOneMigrationPreservesConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manager.db")
	s, c, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	c.SchemaVersion = 1
	c.DecoderProfiles = nil
	c.Settings.FlexDecoder = "n=Existing-Verified,m=OOK_PWM,s=400,l=1200,r=10004"
	c.Remotes = []Remote{{ID: newID(), Name: "Existing", Columns: 2, Buttons: []Button{newButton("Existing button")}}}
	if err = s.Save(c); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, migrated, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c.SchemaVersion = 2
	c.DecoderProfiles = []receiver.DecoderProfile{}
	if !reflect.DeepEqual(c, migrated) {
		t.Fatal("migration changed existing settings or buttons")
	}
	a, err := New(s, migrated, false)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	c.SchemaVersion = 1
	if err = a.Restore(c); err != nil {
		t.Fatal("legacy logical backup rejected", err)
	}
}
