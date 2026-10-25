package main

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ninoleto/rf-button-manager/internal/app"
	"github.com/ninoleto/rf-button-manager/internal/receiver"
)

func TestPrivateListenOnly(t *testing.T) {
	for _, s := range []string{"0.0.0.0:8787", ":8787", "[::]:8787", "8.8.8.8:8787", "example.com:8787"} {
		if validListen(s) == nil {
			t.Error("allowed", s)
		}
	}
	for _, s := range []string{"127.0.0.1:8787", "[::1]:8787", "192.168.1.5:8787", "100.100.1.1:8787"} {
		if e := validListen(s); e != nil {
			t.Error(e)
		}
	}
}

func TestLearningReceiverTransitionsWaitForCleanup(t *testing.T) {
	store, config, err := app.OpenStore(filepath.Join(t.TempDir(), "manager.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	a, err := app.New(store, config, false)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.EnableCaptureLearning("unused", t.TempDir())
	r, err := a.CreateRemote("Test", 2, 6)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan receiver.Config, 8)
	stopping := make(chan struct{}, 8)
	release := make(chan struct{}, 8)
	var active, peak atomic.Int32
	run := func(ctx context.Context, c receiver.Config, _ func([]byte), status func(receiver.Status), _ ...func(receiver.PulsePacket)) error {
		n := active.Add(1)
		for {
			old := peak.Load()
			if n <= old || peak.CompareAndSwap(old, n) {
				break
			}
		}
		defer active.Add(-1)
		started <- c
		status(receiver.Status{State: "connected", Message: "fake sample acquisition"})
		<-ctx.Done()
		stopping <- struct{}{}
		<-release
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); runReceiverLoop(ctx, a, "unused", run) }()
	t.Cleanup(func() {
		cancel()
		for i := 0; i < 4; i++ {
			select {
			case release <- struct{}{}:
			default:
			}
		}
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("receiver loop did not stop")
		}
	})
	waitStart := func() receiver.Config {
		t.Helper()
		select {
		case c := <-started:
			return c
		case <-time.After(time.Second):
			t.Fatal("receiver failed to start")
			return receiver.Config{}
		}
	}
	waitStop := func() {
		t.Helper()
		select {
		case <-stopping:
		case <-time.After(time.Second):
			t.Fatal("receiver did not stop")
		}
	}
	if waitStart().CapturePulses {
		t.Fatal("capture enabled outside learning")
	}
	if _, err = a.StartLearning(r.Buttons[0].ID, nil); err != nil {
		t.Fatal(err)
	}
	waitStop()
	select {
	case <-started:
		t.Fatal("capture started before old receiver cleanup")
	case <-time.After(20 * time.Millisecond):
	}
	release <- struct{}{}
	if !waitStart().CapturePulses {
		t.Fatal("new receiver did not capture")
	}
	if err = a.CancelLearning(); err != nil {
		t.Fatal(err)
	}
	waitStop()
	if _, err = a.StartLearning(r.Buttons[1].ID, nil); err == nil {
		t.Fatal("new learning started while old receiver still owned USB")
	}
	select {
	case <-started:
		t.Fatal("normal receiver started before capture cleanup")
	case <-time.After(20 * time.Millisecond):
	}
	release <- struct{}{}
	if waitStart().CapturePulses {
		t.Fatal("cancel failed to restore normal reception")
	}
	if peak.Load() != 1 {
		t.Fatal("multiple simultaneous live receivers", peak.Load())
	}
}
func TestManagementTokenPersistsPrivately(t *testing.T) {
	dir := t.TempDir()
	one, e := tokenFile(dir)
	if e != nil {
		t.Fatal(e)
	}
	two, e := tokenFile(dir)
	if e != nil || one != two || len(one) != 64 {
		t.Fatal("token not stable")
	}
	st, _ := os.Stat(filepath.Join(dir, "management.token"))
	if st.Mode().Perm() != 0600 {
		t.Fatal("unsafe token mode")
	}
	os.WriteFile(filepath.Join(dir, "management.token"), []byte("invalid"), 0600)
	if _, e = tokenFile(dir); e == nil {
		t.Fatal("corrupt token silently replaced")
	}
}
