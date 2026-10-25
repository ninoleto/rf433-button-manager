//go:build linux

package receiver

import (
	"bufio"
	"bytes"
	"context"
	"math"
	"os/exec"
	"testing"
	"time"
)

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
