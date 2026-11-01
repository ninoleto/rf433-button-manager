//go:build linux

package receiver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNormalMonitoringIQDecodeParity(t *testing.T) {
	binary := installedRTL(t)
	for _, fsk := range []bool{false, true} {
		name := "OOK"
		if fsk {
			name = "FSK"
		}
		t.Run(name, func(t *testing.T) {
			input := filepath.Join(t.TempDir(), "synthetic_433.92M_250k.cu8")
			if err := os.WriteFile(input, syntheticIQ(fsk), 0600); err != nil {
				t.Fatal(err)
			}
			var baseline []string
			for _, monitor := range []bool{false, true} {
				cfg := Config{MonitorPulses: monitor, Profiles: []DecoderProfile{{FlexDecoder: "n=MonitorFixture,m=OOK_PWM,s=400,l=1200,r=20000,g=7000,t=160,bits=24,repeats>=2"}}}
				live := arguments(cfg, device{serial: "must-not-open"})
				var args []string
				for i := 0; i < len(live); i++ {
					if live[i] == "-d" || live[i] == "-D" {
						i++
						continue
					}
					args = append(args, live[i])
				}
				args = append(args, "-r", input)
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, binary, args...)
				var stdout, stderr bytes.Buffer
				cmd.Stdout, cmd.Stderr = &stdout, &stderr
				if err := cmd.Run(); err != nil {
					t.Fatalf("file-only receiver failed: %v %s", err, stderr.String())
				}
				var events []string
				var parser pulseAssembler
				packets, expectedModulation := 0, false
				for _, line := range strings.Split(stdout.String(), "\n") {
					if monitor {
						packet, _ := parser.feed([]byte(line))
						if packet != nil && packet.Error == "" {
							if packet.OOK != fsk {
								expectedModulation = true
							}
							packets++
						}
					}
					if classify([]byte(line)) == event {
						var obj map[string]any
						json.Unmarshal([]byte(line), &obj)
						delete(obj, "time")
						normalized, _ := json.Marshal(obj)
						events = append(events, string(normalized))
					}
				}
				if !monitor {
					baseline = events
					if !fsk && len(events) == 0 {
						t.Fatal("no baseline synthetic OOK decode")
					}
				} else if packets == 0 || !expectedModulation || !reflect.DeepEqual(events, baseline) {
					t.Fatalf("pulse monitoring changed decoded events: %v != %v, packets=%d", events, baseline, packets)
				}
			}
		})
	}
}

func TestAdaptiveIQRecoveryAndRealMeasurementOutput(t *testing.T) {
	binary := installedRTL(t)
	strong := syntheticIQ(false)
	weak := append([]byte(nil), strong...)
	for i, value := range weak {
		weak[i] = byte(128 + (int(value)-128)/8)
	}
	for _, tc := range []struct {
		name     string
		data     []byte
		adaptive bool
		want     bool
	}{{"strong_standard", strong, false, true}, {"faint_standard", weak, false, false}, {"faint_adaptive", weak, true, true}} {
		t.Run(tc.name, func(t *testing.T) {
			// Twelve seconds of quiet I/Q lets the noise estimate settle. Input is
			// entirely synthetic, including its noise floor; no range claim follows.
			data := append(bytes.Repeat([]byte{128}, 12*250000*2), tc.data...)
			path := filepath.Join(t.TempDir(), "synthetic_433.92M_250k.cu8")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			cfg := Config{AdaptiveSensitivity: tc.adaptive, FlexDecoder: "n=SensitivityFixture,m=OOK_PWM,s=400,l=1200,r=20000,g=7000,t=160,bits=24,repeats>=2"}
			// Exercise production options with file input and a shorter diagnostic
			// reporting interval for offline playback, which runs faster than RF.
			var args []string
			live := arguments(cfg, device{serial: "must-not-open"})
			for i := 0; i < len(live); i++ {
				if live[i] == "-d" || live[i] == "-D" {
					i++
					continue
				}
				args = append(args, live[i])
			}
			// File replay finishes faster than the live ten-second report interval.
			args = append(args, "-r", path, "-M", "noise:1")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, args...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("file replay failed: %v: %s", err, stderr.String())
			}
			found := false
			for _, line := range strings.Split(stdout.String(), "\n") {
				var e struct {
					Model string
					Codes []string
				}
				if json.Unmarshal([]byte(line), &e) == nil && e.Model == "SensitivityFixture" {
					for _, code := range e.Codes {
						if code == "{24}a9b2c5" {
							found = true
						}
					}
					if levels := ParseSignalLevels([]byte(line)); levels == nil || levels.RSSI == nil || levels.SNR == nil || levels.Noise == nil {
						t.Fatal("real level metadata not available")
					}
				}
			}
			if found != tc.want {
				t.Fatalf("target decoded=%v want %v", found, tc.want)
			}
			noise := false
			for _, line := range strings.Split(stderr.String(), "\n") {
				if _, ok := noiseLog([]byte(line)); ok {
					noise = true
				}
			}
			if !noise {
				t.Fatalf("upstream noise log format was not recognized: %s", stderr.String())
			}
		})
	}
}

// Entirely synthetic RF, unrelated to the operator's remote. IQ input exercises
// rtl_433's sample-processing callback; .ook replay takes a different code path.
func syntheticIQ(fsk bool) []byte {
	data := make([]byte, 0, 524288)
	phase := 0.0
	add := func(us int, on bool, hz float64) {
		for i := 0; i < us/4; i++ {
			phase += 2 * math.Pi * hz / 250000
			amplitude := 0.0
			if on {
				amplitude = 80
			}
			data = append(data, byte(math.Round(128+amplitude*math.Cos(phase))), byte(math.Round(128+amplitude*math.Sin(phase))))
		}
	}
	add(200000, false, 20000)
	bits := "101010011011001011000101"
	for repeat := 0; repeat < 6; repeat++ {
		for i, bit := range bits {
			mark := 1200
			if bit == '1' {
				mark = 400
			}
			gap := 1600 - mark
			if i == len(bits)-1 {
				gap = 10000
			}
			add(mark, true, 20000)
			add(gap, fsk, -20000)
		}
	}
	add(200000, false, 20000)
	// Full input blocks let rtl_433 process the trailing silence without EOF.
	for len(data) < 524288 {
		data = append(data, 128)
	}
	return data
}

func TestRTL433IQStreamBeforeEOF(t *testing.T) {
	binary := installedRTL(t)
	for _, fsk := range []bool{false, true} {
		name := "OOK_with_JSON"
		if fsk {
			name = "FSK_on_OOK_output"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			// Explicit file input never opens USB. Keep default protocols, as in the
			// initial live capture plan: -R 0 would disable FSK detection in 23.11.
			cmd := exec.CommandContext(ctx, binary, "-c", "/dev/null", "-r", "CU8:-", "-s", "250000", "-w", "OOK:-", "-F", "json", "-F", "log:/dev/stderr", "-M", "stats:1:10")
			stdin, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			if err = cmd.Start(); err != nil {
				t.Fatal(err)
			}
			waited := false
			defer func() {
				stdin.Close()
				if !waited {
					cmd.Process.Kill()
					cmd.Wait()
				}
			}()
			lines := make(chan []byte, 256)
			scanDone := make(chan error, 1)
			go func() {
				scanner := bufio.NewScanner(stdout)
				scanner.Buffer(make([]byte, 4096), MaxCaptureBytes)
				for scanner.Scan() {
					select {
					case lines <- append([]byte(nil), scanner.Bytes()...):
					case <-ctx.Done():
						scanDone <- ctx.Err()
						close(lines)
						return
					}
				}
				scanDone <- scanner.Err()
				close(lines)
			}()
			written := make(chan error, 1)
			go func() { _, err := stdin.Write(syntheticIQ(fsk)); written <- err }()
			var p pulseAssembler
			var packet *PulsePacket
			for packet == nil {
				select {
				case line, ok := <-lines:
					if !ok {
						t.Fatal("stdout closed before a complete pulse packet")
					}
					packet, _ = p.feed(line)
				case <-ctx.Done():
					t.Fatal("pulse output was absent or buffered until EOF")
				}
			}
			// This assertion happens BEFORE closing the input pipe: actual 23.11
			// pulse output is available while the sample source is still open.
			if packet.Error != "" || packet.OOK == fsk || packet.Pulses != 144 || packet.ObservedBytes == 0 || p.snapshot().Completed != 1 {
				t.Fatalf("actual IQ stream incompatible: %+v", packet)
			}
			if !fsk && p.snapshot().JSONRecords == 0 {
				t.Fatal("expected mixed decoded JSON and pulse output")
			}
			select {
			case err = <-written:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal("input write stalled")
			}
			stdin.Close()
			for range lines {
			} // Drain before Wait closes pipe descriptors.
			if err = <-scanDone; err != nil {
				t.Fatal(err)
			}
			err = cmd.Wait()
			waited = true
			if err != nil {
				t.Fatalf("offline rtl_433 failed: %v: %s", err, stderr.String())
			}
		})
	}
}
