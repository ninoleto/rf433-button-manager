package receiver

import (
	"strings"
	"testing"
)

const pulsePreamble = ";pulse data\n;version 1\n;timescale 1us\n"

func assembleLines(input string) (*pulseAssembler, []PulsePacket) {
	p := &pulseAssembler{}
	var packets []PulsePacket
	for _, line := range strings.Split(strings.TrimSuffix(input, "\n"), "\n") {
		if packet, _ := p.feed([]byte(line)); packet != nil {
			packets = append(packets, *packet)
		}
	}
	return p, packets
}

func TestPulseMixedJSONMetadataAndPacketBoundaries(t *testing.T) {
	input := pulsePreamble + ";created synthetic\n" + signalJSON + "\n;received synthetic\n;ook 2 pulses\n;freq1 433920000\n400 1200\n" + heartbeatJSON + "\n\n1200 10000\n;end\n;received synthetic\n;fsk 2 pulses\n400 1200\n1200 10000\n;end\n"
	p, packets := assembleLines(strings.ReplaceAll(input, "\n", "\r\n"))
	if len(packets) != 2 || !packets[0].OOK || packets[1].OOK || packets[0].Error != "" || packets[1].Error != "" {
		t.Fatalf("mixed stream rejected: %+v", packets)
	}
	if strings.Contains(string(packets[0].Data), "{") {
		t.Fatal("JSON leaked into offline pulse replay")
	}
	s := p.snapshot()
	if s.JSONRecords != 2 || s.Headers != 2 || s.Completed != 2 || s.PulseRows != 4 || s.Active || s.Bytes == 0 {
		t.Fatalf("stream evidence lost: %+v", s)
	}
	if _, err := ValidatePulseCapture(packets); err == nil {
		t.Fatal("FSK accepted as supported OOK")
	}
	if _, err := ValidatePulseCapture(packets[:1]); err != nil {
		t.Fatal(err)
	}
}

func TestPulseMalformedCaptureRetainsEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, input, code string
		rows              int
	}{
		{"negative", pulsePreamble + ";ook 2 pulses\n400 1200\n-1 10000\n;end\n", "invalid_timing", 1},
		{"fraction", pulsePreamble + ";ook 1 pulses\n4.5 1200\n;end\n", "invalid_timing", 0},
		{"columns", pulsePreamble + ";ook 1 pulses\n400 1200 1\n;end\n", "invalid_timing", 0},
		{"count", pulsePreamble + ";ook 2 pulses\n400 1200\n;end\n", "pulse_count_mismatch", 1},
		{"empty", pulsePreamble + ";ook 0 pulses\n;end\n", "empty_packet", 0},
		{"units", ";pulse data\n;version 1\n;timescale 4us\n;ook 1 pulses\n400 1200\n;end\n", "invalid_timescale", 1},
		{"version", ";pulse data\n;version 2\n;timescale 1us\n;ook 1 pulses\n400 1200\n;end\n", "invalid_version", 1},
		{"no_units", ";ook 1 pulses\n400 1200\n;end\n", "invalid_version", 1},
		{"header", pulsePreamble + ";ook nope pulses\n;end\n", "invalid_header", 0},
		{"timing_limit", pulsePreamble + ";ook 1 pulses\n10000001 2\n;end\n", "timing_limit", 0},
		{"count_limit", pulsePreamble + ";ook 4097 pulses\n;end\n", "pulse_count_limit", 0},
		{"unframed", pulsePreamble + "400 1200\n", "unframed_data", 0},
		{"orphan", pulsePreamble + ";end\n", "orphan_end", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, packets := assembleLines(tc.input)
			if len(packets) != 1 || packets[0].ErrorCode != tc.code || packets[0].Error == "" || packets[0].Pulses != tc.rows || packets[0].ObservedBytes == 0 || p.snapshot().Bytes == 0 {
				t.Fatalf("missing rejection evidence: %+v", packets)
			}
			if len(packets[0].Data) != 0 {
				t.Fatal("invalid raw data retained for replay")
			}
			if _, err := ValidatePulseCapture(packets); err == nil {
				t.Fatal("malformed input accepted")
			}
		})
	}
}

func TestPulsePartialBoundariesAndFlush(t *testing.T) {
	for _, boundary := range []string{"", ";ook 1 pulses\n", ";received synthetic\n", pulsePreamble} {
		p, packets := assembleLines(pulsePreamble + ";ook 2 pulses\n400 1200\n" + boundary)
		if boundary == "" {
			if s := p.snapshot(); !s.Active || s.Rows != 1 || s.Expected != 2 {
				t.Fatalf("partial capture invisible: %+v", s)
			}
			packets = append(packets, *p.flush())
		}
		if len(packets) != 1 || packets[0].ErrorCode != "missing_end" || packets[0].Pulses != 1 || packets[0].ObservedBytes == 0 {
			t.Fatalf("partial capture silently discarded at %q: %+v", boundary, packets)
		}
	}
	_, packets := assembleLines(pulsePreamble + ";ook 1 pulses\n;" + strings.Repeat("x", MaxCaptureBytes) + "\n400 1200\n;end\n")
	if len(packets) != 1 || packets[0].ErrorCode != "packet_byte_limit" || len(packets[0].Data) != 0 {
		t.Fatal("oversized packet not rejected boundedly")
	}
}
