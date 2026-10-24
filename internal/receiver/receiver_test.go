package receiver

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestDeviceSelectionFailClosed(t *testing.T) {
	a := device{serial: "one", incarnation: "a"}
	b := device{serial: "two", incarnation: "b"}
	for _, tc := range []struct {
		name   string
		ds     []device
		serial string
		want   device
		fail   bool
	}{
		{"absent", nil, "", device{}, true},
		{"singleton", []device{a}, "", a, false},
		{"ambiguous", []device{a, b}, "", device{}, true},
		{"unique serial", []device{a, b}, "two", b, false},
		{"missing configured", []device{a, b}, "three", device{}, true},
		{"duplicate serial", []device{a, {serial: "one", incarnation: "c"}}, "one", device{}, true},
		{"no serial singleton", []device{{incarnation: "a"}}, "", device{incarnation: "a"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := selectDevice(tc.ds, tc.serial)
			if (err != nil) != tc.fail || got != tc.want {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
}

func TestSafeArguments(t *testing.T) {
	spec := "n=Switch433,m=OOK_PWM,s=1,l=2,r=3; $(touch /tmp/do-not-run)"
	args := arguments(Config{Protocols: []int{30}, FlexDecoder: spec}, device{serial: "abc"})
	want := []string{"-c", "/dev/null", "-d", ":abc", "-D", "quit", "-f", "433.92M", "-F", "json", "-F", "log:/dev/stderr", "-M", "stats:1:10", "-R", "0", "-R", "30", "-X", spec}
	if !reflect.DeepEqual(args, want) {
		t.Fatal(args)
	}
	if args := arguments(Config{}, device{}); args[3] != "0" {
		t.Fatal(args)
	}
	for _, cfg := range []Config{{Serial: "x\n"}, {FlexDecoder: "x\x00"}, {Protocols: []int{-1}}} {
		if validate(cfg) == nil {
			t.Fatal("accepted bad config")
		}
	}
}

const heartbeatJSON = `{"enabled":1,"since":"2026-10-25T01:00:00","frames":{"count":0,"events":0},"stats":[]}`
const signalJSON = `{"model":"Switch433","codes":["{24}abcd12"]}`

func TestOutputClassification(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		kind recordKind
	}{
		{heartbeatJSON, stats}, {signalJSON, event}, {`{"model":""}`, ignored}, {`{`, ignored}, {`null`, ignored}, {`[1]`, ignored}, {`{"enabled":1,"stats":[],"frames":null}`, ignored}, {`{"time":"now"}`, ignored},
	} {
		if got := classify([]byte(tc.raw)); got != tc.kind {
			t.Fatalf("%s: %v", tc.raw, got)
		}
	}
}

type fakeProcess struct {
	out     chan record
	exit    chan error
	stopped chan struct{}
	once    sync.Once
}

func newFake() *fakeProcess {
	return &fakeProcess{make(chan record, 64), make(chan error, 1), make(chan struct{}), sync.Once{}}
}
func (f *fakeProcess) output() <-chan record { return f.out }
func (f *fakeProcess) exited() <-chan error  { return f.exit }
func (f *fakeProcess) stop()                 { f.once.Do(func() { close(f.stopped) }) }

type fixture struct {
	mu        sync.Mutex
	devices   []device
	processes chan *fakeProcess
	statuses  chan Status
	events    chan []byte
	done      chan error
	cancel    context.CancelFunc
}

func startFixture(t *testing.T, heartbeat time.Duration) *fixture {
	t.Helper()
	f := &fixture{devices: []device{{serial: "serial", incarnation: "port1:1"}}, processes: make(chan *fakeProcess, 100), statuses: make(chan Status, 100), events: make(chan []byte, 100), done: make(chan error, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel
	rt := runtime{discover: func() ([]device, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		return append([]device(nil), f.devices...), nil
	}, start: func(context.Context, string, []string, time.Duration) (process, error) {
		p := newFake()
		f.processes <- p
		return p, nil
	}, poll: 2 * time.Millisecond, initialBackoff: 10 * time.Millisecond, maxBackoff: 80 * time.Millisecond, stableAfter: time.Second, heartbeatTimeout: heartbeat, stopTimeout: 20 * time.Millisecond}
	go func() {
		f.done <- run(ctx, Config{}, func(b []byte) { f.events <- b }, func(s Status) { f.statuses <- s }, rt)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-f.done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(time.Second):
			t.Error("supervisor did not stop")
		}
	})
	return f
}
func (f *fixture) setDevices(ds ...device) { f.mu.Lock(); f.devices = ds; f.mu.Unlock() }
func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
		var z T
		return z
	}
}
func waitState(t *testing.T, ch <-chan Status, state string) Status {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case s := <-ch:
			if s.State == state {
				return s
			}
		case <-deadline:
			t.Fatalf("missing %s state", state)
			return Status{}
		}
	}
}

func TestSupervisorUSBMoveAndReconnect(t *testing.T) {
	f := startFixture(t, time.Second)
	first := receive(t, f.processes)
	first.out <- record{data: []byte(heartbeatJSON)}
	waitState(t, f.statuses, "connected")
	f.setDevices()
	receive(t, first.stopped)
	waitState(t, f.statuses, "disconnected")
	f.setDevices(device{serial: "serial", incarnation: "different-hub:7"})
	second := receive(t, f.processes)
	second.out <- record{data: []byte(signalJSON)}
	receive(t, f.events)
	waitState(t, f.statuses, "connected")
	// Unplug/replug into the same sysfs port still changes its USB device number.
	f.setDevices(device{serial: "serial", incarnation: "different-hub:8"})
	receive(t, second.stopped)
	third := receive(t, f.processes)
	if third == second {
		t.Fatal("reused stale child")
	}
}

func TestSupervisorNewAmbiguityStopsChild(t *testing.T) {
	f := startFixture(t, time.Second)
	p := receive(t, f.processes)
	f.setDevices(device{serial: "one", incarnation: "a"}, device{serial: "two", incarnation: "b"})
	receive(t, p.stopped)
	waitState(t, f.statuses, "error")
	select {
	case <-f.processes:
		t.Fatal("started with ambiguous receivers")
	case <-time.After(30 * time.Millisecond):
	}
}

func TestSupervisorSilenceWithSampleHeartbeatDoesNotRestart(t *testing.T) {
	f := startFixture(t, 50*time.Millisecond)
	p := receive(t, f.processes)
	for i := 0; i < 12; i++ {
		p.out <- record{data: []byte(heartbeatJSON)}
		time.Sleep(10 * time.Millisecond)
	}
	select {
	case <-f.processes:
		t.Fatal("restarted on RF silence")
	default:
	}
	select {
	case <-f.events:
		t.Fatal("statistics reached learning")
	default:
	}
}

func TestSupervisorStalledSampleHeartbeatRestarts(t *testing.T) {
	f := startFixture(t, 35*time.Millisecond)
	p := receive(t, f.processes)
	receive(t, p.stopped)
	s := waitState(t, f.statuses, "error")
	if s.Message != "rtl_433 sample heartbeat stopped; restarting decoder" {
		t.Fatal(s)
	}
	receive(t, f.processes)
}

func TestSupervisorCrashBackoff(t *testing.T) {
	f := startFixture(t, time.Second)
	for i := 0; i < 4; i++ {
		p := receive(t, f.processes)
		start := time.Now()
		p.exit <- errors.New("crashed")
		receive(t, p.stopped)
		// Next retry must not busy-loop. Use a lower bound with timer scheduling margin.
		minimum := time.Duration(1<<i) * 10 * time.Millisecond
		next := receive(t, f.processes)
		if elapsed := time.Since(start); elapsed < minimum-2*time.Millisecond {
			t.Fatalf("backoff %s < %s", elapsed, minimum)
		}
		// Keep the new process available for the next iteration.
		f.processes <- next
	}
}

func TestSupervisorCancellationStopsProcess(t *testing.T) {
	f := startFixture(t, time.Second)
	p := receive(t, f.processes)
	f.cancel()
	receive(t, p.stopped)
}

func TestSupervisorStartFailureIsRetriedWithBackoff(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 65*time.Millisecond)
	defer cancel()
	count := 0
	rt := runtime{discover: func() ([]device, error) { return []device{{serial: "a"}}, nil }, start: func(context.Context, string, []string, time.Duration) (process, error) {
		count++
		return nil, errors.New("not installed")
	}, poll: time.Millisecond, initialBackoff: 10 * time.Millisecond, maxBackoff: 40 * time.Millisecond, stableAfter: time.Second, heartbeatTimeout: time.Second}
	if err := run(ctx, Config{}, nil, nil, rt); err != nil {
		t.Fatal(err)
	}
	if count < 2 || count > 4 {
		t.Fatalf("unexpected attempts %d", count)
	}
}
