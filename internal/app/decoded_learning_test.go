package app

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ninoleto/rf-button-manager/internal/receiver"
)

func TestDecodedButtonInterruptsBackgroundAnalysis(t *testing.T) {
	a, c, r := captureApp(t)
	gen := startCapture(t, a, r.Buttons[0].ID)
	started, finished := make(chan struct{}), make(chan struct{})
	a.analyzeCapture = func(ctx context.Context, _ receiver.AnalysisRequest) (receiver.AnalysisResult, error) {
		close(started)
		<-ctx.Done()
		return receiver.AnalysisResult{}, ctx.Err()
	}
	a.ReceivePulse(gen, receiver.PulsePacket{OOK: true, Pulses: 144, Data: captureFixture(t)})
	c.advance(time.Second)
	go func() { a.learningTick(); close(finished) }()
	<-started
	a.Receive([]byte(`{"model":"Generic-Remote","id":30037,"cmd":85}`))
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("background replay did not stop")
	}
	if a.learning == nil || a.learning.Presses != 1 || a.learning.Stage != "captured" || a.analysisActive {
		t.Fatalf("background analysis overrode decoded press: %+v", a.learning)
	}
}

func captureFixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("../receiver/testdata/synthetic-pwm.ook")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Field shapes observed by replaying public upstream IQ recordings through 23.11.
// These are decoder events, not a claim of physical testing of the user's remote.
func TestKnownRemoteDecodersLearnWithoutPulseRediscovery(t *testing.T) {
	for name, raw := range map[string]string{
		"EV1527": `{"model":"Akhan-100F14","id":455627,"data":"0x1 (Lock)"}`,
		"PT2262": `{"model":"Generic-Remote","id":30037,"cmd":85,"tristate":"Z1ZZZZZZZZZZ"}`,
		"HT680":  `{"model":"HT680-Remote","id":961082,"button1":"PRESSED","button2":"","button3":"PRESSED","button4":""}`,
	} {
		t.Run(name, func(t *testing.T) {
			a, c, r := captureApp(t)
			gen := startCapture(t, a, r.Buttons[0].ID)
			for press := 0; press < 3; press++ {
				if press > 0 {
					c.advance(1100 * time.Millisecond)
					if _, err := a.ArmLearning(); err != nil {
						t.Fatal(err)
					}
				}
				a.Receive([]byte(raw))
				if a.learning == nil || a.learning.Presses != press+1 {
					t.Fatalf("decoder event did not learn: %+v", a.learning)
				}
				for n := 0; n < 5; n++ {
					a.Receive([]byte(raw))
				}
				if a.learning.Presses != press+1 {
					t.Fatal("repeated frame counted as a separate press")
				}
				// Unrelated raw activity must not block rearming a known identity.
				a.ReceivePulse(gen, receiver.PulsePacket{OOK: true, Pulses: 144, Data: captureFixture(t)})
			}
			if a.learning.Diagnostics.AnalysisAttempts != 0 {
				t.Fatal("already-decoded event was needlessly reanalyzed")
			}
			if err := a.ConfirmLearning(); err != nil {
				t.Fatal(err)
			}
			b := a.Backup().Remotes[0].Buttons[0]
			if b.Enabled || b.Signature == "" {
				t.Fatal("mapping missing or action enabled")
			}
			_, normal := a.ReceiverPlan("rtl_433")
			a.ReceiverPlanApplied(normal)
			c.advance(2 * time.Second)
			a.Receive([]byte(raw))
			if a.Backup().Remotes[0].Buttons[0].LastReceived == "" {
				t.Fatal("learned identity did not match normal reception")
			}
		})
	}
}

func TestCanonicalButtonAliasesSeparateButtons(t *testing.T) {
	for _, pair := range [][2]string{
		{`{"model":"Remote","id":3,"cmd":1}`, `{"model":"Remote","id":3,"cmd":2}`},
		{`{"model":"Remote","id":3,"data":"0x1"}`, `{"model":"Remote","id":3,"data":"0x2"}`},
		{`{"model":"Remote","id":3,"button1":"PRESSED","button2":""}`, `{"model":"Remote","id":3,"button1":"","button2":"PRESSED"}`},
	} {
		a, e := Signature([]byte(pair[0]), defaultFields)
		if e != nil {
			t.Fatal(e)
		}
		b, e := Signature([]byte(pair[1]), defaultFields)
		if e != nil {
			t.Fatal(e)
		}
		if a == b || signaturesOverlap(a, b) {
			t.Fatal("different buttons overlap")
		}
	}
	if _, err := Signature([]byte(`{"model":"Weather","id":1,"temperature_C":22,"battery_ok":1}`), defaultFields); err == nil {
		t.Fatal("sensor measurement became a button")
	}
}

func TestShortNoiseDoesNotSpendLearningBudgetOrRequireSilence(t *testing.T) {
	a, c, r := captureApp(t)
	gen := startCapture(t, a, r.Buttons[0].ID)
	for i := 0; i < 30; i++ {
		a.ReceivePulse(gen, receiver.PulsePacket{OOK: true, Pulses: 10, Data: []byte("noise")})
		c.advance(50 * time.Millisecond)
		a.learningTick()
	}
	if a.learning == nil || a.learning.WindowAnalyses != 0 || a.learning.Presses != 0 {
		t.Fatal("short noise changed learning")
	}
	a.ReceivePulse(gen, receiver.PulsePacket{OOK: true, Pulses: 144, Data: captureFixture(t)})
	c.advance(time.Second)
	a.learningTick()
	if a.learning == nil || a.learning.Presses != 1 {
		t.Fatal("noise prevented subsequent capture")
	}
}
