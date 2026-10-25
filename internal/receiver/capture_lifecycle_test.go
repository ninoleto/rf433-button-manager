package receiver

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

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
