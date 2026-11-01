package receiver

import (
	"reflect"
	"testing"
	"time"
)

func TestAdaptiveArgumentsPreserveStandardAndCapture(t *testing.T) {
	for _, capture := range []bool{false, true} {
		cfg := Config{CapturePulses: capture}
		standard := arguments(cfg, device{serial: "fixture"})
		cfg.AdaptiveSensitivity = true
		adaptive := arguments(cfg, device{serial: "fixture"})
		var restored []string
		found := 0
		for i := 0; i < len(adaptive); i++ {
			if adaptive[i] == "-Y" && i+1 < len(adaptive) && adaptive[i+1] == "autolevel" {
				found++
				i++
				continue
			}
			restored = append(restored, adaptive[i])
		}
		if found != 1 || !reflect.DeepEqual(restored, standard) {
			t.Fatal("adaptive mode changed more than the detection threshold")
		}
	}
}

func TestSignalLevelsValidateMissingZeroAndInvalid(t *testing.T) {
	l := ParseSignalLevels([]byte(`{"model":"Fixture","rssi":0,"snr":12.5,"noise":-32.1}`))
	if l == nil || l.RSSI == nil || *l.RSSI != 0 || *l.SNR != 12.5 || *l.Noise != -32.1 {
		t.Fatal(l)
	}
	for _, raw := range []string{`{}`, `null`, `[]`, `{`, `{"rssi":null,"noise":"-5","snr":-1}`, `{"rssi":1e300,"noise":-300,"snr":true}`} {
		if ParseSignalLevels([]byte(raw)) != nil {
			t.Fatalf("invalid measurement accepted: %s", raw)
		}
	}
}

func TestNoiseDiagnosticStrictFormat(t *testing.T) {
	for _, kind := range []string{"noise", "signal"} {
		if value, ok := noiseLog([]byte("Auto Level: Current " + kind + " level -23.4 dB, estimated noise -25.6 dB")); !ok || value != -25.6 {
			t.Fatal(value, ok)
		}
	}
	for _, line := range []string{signalJSON, "prefix Auto Level: Current noise level -2 dB, estimated noise -3 dB", "Auto Level: Current noise level -2 dB, estimated noise NaN dB", "Auto Level: Current noise level -2 dB, estimated noise -300 dB", "Auto Level: Current noise level -2 dB, estimated noise -3 dB trailing"} {
		if _, ok := noiseLog([]byte(line)); ok {
			t.Fatal("accepted malformed noise log")
		}
	}
}

func TestNoiseStatusCannotBecomeRFOrKeepStalledReceiverAlive(t *testing.T) {
	f := startConfiguredFixture(t, 80*time.Millisecond, Config{AdaptiveSensitivity: true})
	p := receive(t, f.processes)
	p.out <- record{data: []byte(heartbeatJSON)}
	waitState(t, f.statuses, "connected")
	p.out <- record{data: []byte("Auto Level: Current noise level -23.4 dB, estimated noise -25.6 dB"), stderr: true}
	s := waitState(t, f.statuses, "connected")
	if s.Noise == nil || s.Noise.DBFS != -25.6 || s.Noise.At == "" || !s.AdaptiveSensitivity {
		t.Fatal(s)
	}
	// Continued diagnostics cannot hide a missing sample heartbeat.
	for i := 0; i < 8; i++ {
		p.out <- record{data: []byte("Auto Level: Current noise level -23.4 dB, estimated noise -25.6 dB"), stderr: true}
		time.Sleep(15 * time.Millisecond)
	}
	receive(t, p.stopped)
	errState := waitState(t, f.statuses, "error")
	if errState.Noise != nil {
		t.Fatal("stale noise survived stopped receiver")
	}
	select {
	case <-f.events:
		t.Fatal("noise report became an RF event")
	default:
	}
}
