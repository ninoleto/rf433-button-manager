package receiver

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNormalMonitoringKeepsOneReceiverAndUnchangedDecodingArguments(t *testing.T) {
	base := Config{Protocols: []int{30}, AdaptiveSensitivity: true}
	baseArgs := arguments(base, device{serial: "synthetic"})
	base.MonitorPulses = true
	got := arguments(base, device{serial: "synthetic"})
	var stripped []string
	for i := 0; i < len(got); i++ {
		if got[i] == "-w" && i+1 < len(got) && got[i+1] == "OOK:-" {
			i++
			continue
		}
		stripped = append(stripped, got[i])
	}
	if !reflect.DeepEqual(stripped, baseArgs) || len(got) != len(baseArgs)+2 {
		t.Fatal("normal monitoring changed decoder or sensitivity arguments", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := newFake()
	started := make(chan struct{}, 2)
	packets := make(chan PulsePacket, 64)
	events := make(chan string, 8)
	statuses := make(chan Status, 64)
	rt := runtime{discover: func() ([]device, error) { return []device{{serial: "synthetic", incarnation: "fake:1"}}, nil }, start: func(context.Context, string, []string, time.Duration) (process, error) {
		started <- struct{}{}
		return p, nil
	}, poll: time.Millisecond, heartbeatTimeout: time.Second, stopTimeout: time.Millisecond, initialBackoff: time.Millisecond, maxBackoff: time.Second, stableAfter: time.Second}
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, base, func(raw []byte) { events <- string(raw) }, func(s Status) { statuses <- s }, rt, func(packet PulsePacket) { packets <- packet })
	}()
	receive(t, started)
	for _, line := range strings.Split(strings.TrimSuffix(pulsePreamble, "\n"), "\n") {
		p.out <- record{data: []byte(line)}
	}
	for _, line := range []string{";ook 2 pulses", "400 1200", signalJSON, "1200 10000", ";end", heartbeatJSON, signalJSON} {
		p.out <- record{data: []byte(line)}
	}
	if receive(t, events) != signalJSON || receive(t, events) != signalJSON {
		t.Fatal("normal monitoring altered decoded events")
	}
	for {
		packet := receive(t, packets)
		if !packet.Progress && !packet.Final {
			if packet.Error != "" || !packet.OOK || packet.Pulses != 2 || packet.Stream.JSONRecords != 1 {
				t.Fatal("interleaved pulse/JSON records corrupted", packet)
			}
			break
		}
	}
	cancel()
	if err := receive(t, done); err != nil {
		t.Fatal(err)
	}
	if len(started) != 0 {
		t.Fatal("monitoring restarted the receiver")
	}
	close(statuses)
	for s := range statuses {
		if s.State == "connected" && s.Message != "receiver is acquiring samples" {
			t.Fatal("pulse traffic would alternate receiver status and flood event storage", s)
		}
	}
}

func TestNormalPulseParserRemainsBoundedBeyondRecordingByteLimit(t *testing.T) {
	var parser pulseAssembler
	for _, line := range strings.Split(strings.TrimSuffix(pulsePreamble, "\n"), "\n") {
		parser.feed([]byte(line))
	}
	packets := 0
	for parser.stream.Bytes < 2*MaxCaptureStreamBytes {
		for _, line := range []string{";ook 2 pulses", "400 1200", "1200 10000", ";end"} {
			packet, _ := parser.feed([]byte(line))
			if packet != nil {
				if packet.Error != "" || packet.Pulses != 2 {
					t.Fatal("cumulative monitoring traffic disabled future packets")
				}
				packets++
			}
		}
		if parser.buf.Len() != 0 || parser.buf.Cap() > MaxCaptureBytes {
			t.Fatal("normal monitoring retained historical pulse data")
		}
	}
	if parser.stream.Completed != packets || parser.stream.Headers != packets {
		t.Fatal("cumulative stream accounting lost records")
	}
}

// Models records already queued by the reader when stop joins it. A complete
// packet and a partial next packet must remain diagnostics, never candidates.
type queuedStopProcess struct {
	*fakeProcess
	stopOnce sync.Once
}

func (p *queuedStopProcess) stop() {
	p.stopOnce.Do(func() {
		for _, line := range []string{"1200 10000", ";end", ";ook 2 pulses", "400 1200"} {
			p.out <- record{data: []byte(line)}
		}
		close(p.out)
		p.fakeProcess.stop()
	})
}

func TestCaptureStopDrainsQueuedRecordsAndFlushesPartialPacket(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := &queuedStopProcess{fakeProcess: newFake()}
	started := make(chan struct{}, 1)
	packets := make(chan PulsePacket, 64)
	rt := runtime{discover: func() ([]device, error) { return []device{{serial: "synthetic", incarnation: "fake:1"}}, nil }, start: func(context.Context, string, []string, time.Duration) (process, error) {
		started <- struct{}{}
		return p, nil
	}, poll: time.Millisecond, heartbeatTimeout: time.Second, stopTimeout: time.Millisecond, initialBackoff: time.Millisecond, maxBackoff: time.Second, stableAfter: time.Second}
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, Config{CapturePulses: true}, nil, nil, rt, func(packet PulsePacket) { packets <- packet })
	}()
	receive(t, started)
	for _, line := range strings.Split(strings.TrimSuffix(pulsePreamble, "\n"), "\n") {
		p.out <- record{data: []byte(line)}
	}
	p.out <- record{data: []byte(";ook 2 pulses")}
	p.out <- record{data: []byte("400 1200")}
	p.out <- record{data: []byte(heartbeatJSON)}
	for {
		progress := receive(t, packets)
		if progress.Progress && progress.Stream.Rows == 1 && progress.Stream.JSONRecords == 1 {
			break
		}
	}
	cancel()
	if err := receive(t, done); err != nil {
		t.Fatal(err)
	}
	close(packets)
	var records []PulsePacket
	var final PulseStream
	for packet := range packets {
		if packet.Final {
			final = packet.Stream
		} else if !packet.Progress {
			records = append(records, packet)
		}
	}
	if len(records) != 2 || !records[0].Cleanup || records[0].Pulses != 2 || records[0].Error != "" || !records[1].Cleanup || records[1].ErrorCode != "missing_end" || records[1].Pulses != 1 {
		t.Fatalf("queued/partial evidence discarded: %+v", records)
	}
	if !final.Closed || !final.Active || final.Rows != 1 || final.Expected != 2 || final.Headers != 2 || final.Completed != 1 || final.PulseRows != 3 || final.JSONRecords != 1 || final.Bytes == 0 {
		t.Fatalf("incorrect final stream evidence: %+v", final)
	}
	if _, err := ValidatePulseCapture(records[:1]); err == nil {
		t.Fatal("shutdown data eligible as a new candidate")
	}
	select {
	case <-p.stopped:
	default:
		t.Fatal("receiver child survived cleanup")
	}
}
