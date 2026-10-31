package app

import (
	"strings"
	"testing"
	"time"
)

// Synthetic identities only. No private RF identifiers are embedded in Git.
func TestBuiltInDecoderWinsOverFirstCustomFrame(t *testing.T) {
	a, c, r := captureApp(t)
	a.config.Settings.FlexDecoder = "n=CustomExample,m=OOK_PWM,s=300,l=900,r=9000,bits=24"
	startCapture(t, a, r.Buttons[0].ID)
	a.Receive([]byte(`{"model":"CustomExample","codes":["{24}abcdef"]}`))
	if a.learning.Presses != 0 {
		t.Fatal("first flex result selected before comparison window")
	}
	c.advance(50 * time.Millisecond)
	a.Receive([]byte(`{"model":"ExampleBuiltin","id":100,"cmd":1}`))
	if a.learning.Presses != 1 || !strings.Contains(a.learning.Candidate, "ExampleBuiltin") || a.learning.Profile != nil {
		t.Fatal("built-in decoder did not replace early custom candidate")
	}
	for i := 0; i < 2; i++ {
		c.advance(2 * time.Second)
		if _, e := a.ArmLearning(); e != nil {
			t.Fatal(e)
		}
		a.Receive([]byte(`{"model":"ExampleBuiltin","id":100,"cmd":1}`))
	}
	if e := a.ConfirmLearning(); e != nil {
		t.Fatal(e)
	}
	if b := a.Backup().Remotes[0].Buttons[0]; b.Enabled || !strings.Contains(b.Signature, "ExampleBuiltin") {
		t.Fatal("wrong mapping saved or action enabled")
	}
}

func TestCustomOnlyDecoderStillLearnsAfterBoundedWindow(t *testing.T) {
	a, c, r := captureApp(t)
	a.config.Settings.FlexDecoder = "n=CustomExample,m=OOK_PWM,s=300,l=900,r=9000,bits=24"
	startCapture(t, a, r.Buttons[0].ID)
	for i := 0; i < 30; i++ {
		a.Receive([]byte(`{"model":"CustomExample","codes":["{24}abcdef"]}`))
		c.advance(15 * time.Millisecond)
	}
	a.learningTick()
	if a.learning.Presses != 1 || !strings.Contains(a.learning.Candidate, "CustomExample") {
		t.Fatal("custom-only decoding lost or duplicate press counted")
	}
	if e := a.ConfirmLearning(); e == nil {
		t.Fatal("single press confirmed")
	}
}

func TestPendingDecoderCancelKeepsMapping(t *testing.T) {
	a, c, r := captureApp(t)
	a.config.Settings.FlexDecoder = "n=CustomExample,m=OOK_PWM,s=300,l=900,r=9000,bits=24"
	before := a.Backup()
	startCapture(t, a, r.Buttons[0].ID)
	a.Receive([]byte(`{"model":"CustomExample","codes":["{24}abcdef"]}`))
	if e := a.CancelLearning(); e != nil {
		t.Fatal(e)
	}
	c.advance(time.Second)
	a.learningTick()
	if a.Backup().Remotes[0].Buttons[0].Signature != before.Remotes[0].Buttons[0].Signature {
		t.Fatal("cancelled candidate changed mapping")
	}
}
