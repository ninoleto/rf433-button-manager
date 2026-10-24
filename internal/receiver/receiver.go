// Package receiver supervises a native rtl_433 process. The Linux implementation
// deliberately identifies a receiver afresh after every USB topology change.
package receiver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Binary      string
	Serial      string
	FlexDecoder string
	Protocols   []int
}

type Status struct {
	State    string `json:"state"`
	Message  string `json:"message"`
	Serial   string `json:"serial"`
	Restarts int    `json:"restarts"`
}

type device struct {
	serial string
	// incarnation is only an in-memory observation, never a selector or saved path.
	incarnation string
}

type record struct {
	data []byte
	err  error
}

type process interface {
	output() <-chan record
	exited() <-chan error
	stop()
}

type runtime struct {
	discover                                                                     func() ([]device, error)
	start                                                                        func(context.Context, string, []string, time.Duration) (process, error)
	poll, initialBackoff, maxBackoff, stableAfter, heartbeatTimeout, stopTimeout time.Duration
}

var errMissing = errors.New("receiver is disconnected")

// Run manages acquisition until ctx is canceled. Recoverable USB/process errors
// are reported as status and retried with capped exponential backoff. Callbacks
// run serially and must return promptly. They must not call Run recursively.
// A nil callback is allowed. No physical hardware validation is implied by this API.
func Run(ctx context.Context, cfg Config, onJSON func([]byte), onStatus func(Status)) error {
	if !platformSupported {
		return errors.New("native RTL-SDR supervision is supported only on Linux")
	}
	return run(ctx, cfg, onJSON, onStatus, defaultRuntime())
}

func validate(cfg Config) error {
	if len(cfg.Serial) > 255 || strings.ContainsAny(cfg.Serial, "\x00\r\n") {
		return errors.New("receiver serial must be at most 255 characters without control delimiters")
	}
	if len(cfg.FlexDecoder) > 8192 || strings.ContainsAny(cfg.FlexDecoder, "\x00\r\n") {
		return errors.New("flex decoder must be one line of at most 8192 characters")
	}
	if len(cfg.Protocols) > 128 {
		return errors.New("too many receiver protocols")
	}
	for _, id := range cfg.Protocols {
		if id < 1 || id > 10000 {
			return errors.New("protocol IDs must be positive integers")
		}
	}
	return nil
}

func arguments(cfg Config, dev device) []string {
	selector := "0"
	if dev.serial != "" {
		selector = ":" + dev.serial
	}
	// These options are supported by rtl_433 23.11. -D quit prevents competing
	// restart loops. Stats are emitted while IQ samples arrive, even in RF silence.
	args := []string{"-c", "/dev/null", "-d", selector, "-D", "quit", "-f", "433.92M", "-F", "json", "-F", "log:/dev/stderr", "-M", "stats:1:10"}
	if len(cfg.Protocols) > 0 || cfg.FlexDecoder != "" {
		args = append(args, "-R", "0")
	}
	for _, id := range cfg.Protocols {
		args = append(args, "-R", strconv.Itoa(id))
	}
	if cfg.FlexDecoder != "" {
		args = append(args, "-X", cfg.FlexDecoder)
	}
	return args
}

func selectDevice(devices []device, serial string) (device, error) {
	if serial == "" {
		if len(devices) == 0 {
			return device{}, errMissing
		}
		if len(devices) != 1 {
			return device{}, errors.New("multiple receivers found; configure a unique receiver serial")
		}
		return devices[0], nil
	}
	var matches []device
	for _, d := range devices {
		if d.serial == serial {
			matches = append(matches, d)
		}
	}
	if len(matches) == 0 {
		return device{}, errMissing
	}
	if len(matches) != 1 {
		return device{}, errors.New("receiver serial is not unique; disconnect duplicates or assign unique serials")
	}
	return matches[0], nil
}

func run(ctx context.Context, cfg Config, onJSON func([]byte), onStatus func(Status), rt runtime) error {
	if err := validate(cfg); err != nil {
		return err
	}
	if cfg.Binary == "" {
		cfg.Binary = "rtl_433"
	}
	if onJSON == nil {
		onJSON = func([]byte) {}
	}
	if onStatus == nil {
		onStatus = func(Status) {}
	}
	var previous Status
	attempts := 0
	emit := func(state, message, serial string) {
		s := Status{State: state, Message: message, Serial: serial, Restarts: max(0, attempts-1)}
		if s != previous {
			previous = s
			onStatus(s)
		}
	}
	var child process
	var current device
	var out <-chan record
	var exited <-chan error
	var started, heartbeat, nextAttempt time.Time
	backoff := rt.initialBackoff
	stop := func() {
		if child != nil {
			child.stop()
			child = nil
		}
		out, exited = nil, nil
	}
	defer stop()
	failure := func(message string) {
		stop()
		emit("error", message, current.serial)
		nextAttempt = time.Now().Add(backoff)
		backoff = min(backoff*2, rt.maxBackoff)
	}
	ticker := time.NewTicker(rt.poll)
	defer ticker.Stop()
	// Start discovery immediately; subsequent checks are bounded by poll.
	check := make(chan time.Time, 1)
	check <- time.Now()
	for {
		select {
		case <-ctx.Done():
			stop()
			emit("disconnected", "receiver service stopped", "")
			return nil
		case err := <-exited:
			message := "rtl_433 stopped unexpectedly; checking receiver access and retrying"
			if err != nil {
				message = "rtl_433 exited with an error; check permissions, decoder settings or another receiver process"
			}
			if diagnostics, ok := child.(interface{ lastDiagnostic() string }); ok {
				if detail := diagnostics.lastDiagnostic(); detail != "" {
					message += ": " + detail
				}
			}
			failure(message)
		case rec, ok := <-out:
			if !ok {
				out = nil
				continue
			}
			if rec.err != nil {
				failure(rec.err.Error())
				continue
			}
			kind := classify(rec.data)
			if kind == ignored {
				continue
			}
			heartbeat = time.Now()
			emit("connected", "receiver is acquiring samples", current.serial)
			if kind == event {
				onJSON(rec.data)
			}
		case <-ticker.C:
			select {
			case check <- time.Now():
			default:
			}
		case now := <-check:
			devices, err := rt.discover()
			if err != nil {
				stop()
				emit("error", "USB discovery failed; check sysfs access", "")
				continue
			}
			d, err := selectDevice(devices, cfg.Serial)
			if err != nil {
				stop()
				state := "error"
				if errors.Is(err, errMissing) {
					state = "disconnected"
				}
				emit(state, err.Error(), cfg.Serial)
				continue
			}
			if child != nil && current != d {
				stop()
				emit("recovering", "USB receiver changed; reacquiring it", d.serial)
				nextAttempt = now.Add(rt.initialBackoff)
			}
			if child != nil {
				if now.Sub(heartbeat) > rt.heartbeatTimeout {
					failure("rtl_433 sample heartbeat stopped; restarting decoder")
				} else if now.Sub(started) >= rt.stableAfter {
					backoff = rt.initialBackoff
				}
				continue
			}
			if now.Before(nextAttempt) {
				continue
			}
			current = d
			attempts++
			emit("recovering", "starting rtl_433; waiting for sample acquisition", d.serial)
			p, err := rt.start(ctx, cfg.Binary, arguments(cfg, d), rt.stopTimeout)
			if err != nil {
				failure("cannot start rtl_433; check executable installation and permissions")
				continue
			}
			child, out, exited = p, p.output(), p.exited()
			started, heartbeat = now, now
		}
	}
}

type recordKind int

const (
	ignored recordKind = iota
	event
	stats
)

// Recognize the rtl_433 23.11 stats shape without letting it enter learning.
// Unknown or malformed output is deliberately not an RF event.
func classify(raw []byte) recordKind {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil || obj == nil {
		return ignored
	}
	var model string
	if json.Unmarshal(obj["model"], &model) == nil && strings.TrimSpace(model) != "" {
		return event
	}
	var enabled int
	var frames map[string]json.RawMessage
	var list []json.RawMessage
	if json.Unmarshal(obj["enabled"], &enabled) == nil && json.Unmarshal(obj["frames"], &frames) == nil && frames != nil && json.Unmarshal(obj["stats"], &list) == nil {
		return stats
	}
	return ignored
}

func lineError(stream string) error {
	return fmt.Errorf("rtl_433 %s exceeded the safe line size or became unreadable", stream)
}
