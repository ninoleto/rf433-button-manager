package receiver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const MaxCaptureBytes = 256 * 1024
const MaxCapturePackets = 32
const MaxCaptureStreamBytes = 2 * 1024 * 1024

// PulseStream describes stdout records actually read, independently of whether
// their pulse packets were accepted. It never contains raw RF or JSON payloads.
type PulseStream struct {
	Bytes       int  `json:"bytes"`
	Lines       int  `json:"lines"`
	JSONRecords int  `json:"json_records"`
	Headers     int  `json:"headers"`
	Completed   int  `json:"completed"`
	PulseRows   int  `json:"pulse_rows"`
	Active      bool `json:"packet_incomplete"`
	Expected    int  `json:"expected_pulses"`
	Rows        int  `json:"current_rows"`
	Closed      bool `json:"closed"`
}

// Progress records carry a stream snapshot only. Packet records retain counts
// and an exact rejection reason even when invalid raw data must be discarded.
type PulsePacket struct {
	Data           []byte
	OOK            bool
	Pulses         int
	ExpectedPulses int
	ObservedBytes  int
	Error          string
	ErrorCode      string
	Progress       bool
	Final          bool
	Cleanup        bool // Evidence drained after stopping; never a new learning candidate.
	Stream         PulseStream
}

type pulseAssembler struct {
	buf                                     bytes.Buffer
	active, ook                             bool
	expected, count, validRows, packetBytes int
	errorCode, reason, timescale, version   string
	stream                                  PulseStream
}

func (p *pulseAssembler) snapshot() PulseStream {
	s := p.stream
	s.Active = p.active
	if p.active {
		s.Expected = p.expected
		s.Rows = p.count
	}
	return s
}

func (p *pulseAssembler) reject(code, reason string) {
	if p.errorCode == "" {
		p.errorCode = code
		p.reason = reason
	}
}

func (p *pulseAssembler) finish(code, reason string) *PulsePacket {
	if !p.active {
		return nil
	}
	if code != "" {
		p.reject(code, reason)
	}
	if p.count != p.expected {
		p.reject("pulse_count_mismatch", fmt.Sprintf("Pulse header declared %d rows; %d timing rows were read", p.expected, p.count))
	}
	if p.count == 0 {
		p.reject("empty_packet", "Pulse packet ended without any timing rows")
	}
	packet := &PulsePacket{OOK: p.ook, Pulses: p.validRows, ExpectedPulses: p.expected, ObservedBytes: p.packetBytes, ErrorCode: p.errorCode, Error: p.reason}
	if p.errorCode == "" {
		packet.Data = append([]byte(nil), p.buf.Bytes()...)
	}
	p.active = false
	p.buf.Reset()
	p.expected = 0
	p.count = 0
	p.validRows = 0
	p.packetBytes = 0
	p.errorCode = ""
	p.reason = ""
	packet.Stream = p.snapshot()
	return packet
}

func (p *pulseAssembler) flush() *PulsePacket {
	return p.finish("missing_end", "Pulse stream stopped before the packet's ;end boundary")
}

func (p *pulseAssembler) append(line []byte) {
	p.packetBytes += len(line) + 1
	if p.packetBytes > MaxCaptureBytes || p.buf.Len()+len(line)+1 > MaxCaptureBytes {
		p.reject("packet_byte_limit", "Pulse packet exceeded the 256 KiB limit")
	}
	if p.errorCode == "" {
		p.buf.Write(line)
		p.buf.WriteByte('\n')
	}
}

func (p *pulseAssembler) feed(line []byte) (*PulsePacket, bool) {
	p.stream.Bytes += len(line) + 1
	p.stream.Lines++
	text := strings.TrimSpace(string(line))
	// rtl_433 JSON output is one complete object per line, independent of pulse
	// output. Do not copy interleaved JSON into a recorded pulse file.
	if strings.HasPrefix(text, "{") && json.Valid([]byte(text)) {
		p.stream.JSONRecords++
		return nil, false
	}
	if text == "" {
		return nil, true
	}
	if strings.HasPrefix(text, ";ook") || strings.HasPrefix(text, ";fsk") {
		prior := p.finish("missing_end", "A new pulse header arrived before the previous packet's ;end boundary")
		p.stream.Headers++
		p.active = true
		p.ook = strings.HasPrefix(text, ";ook")
		parts := strings.Fields(text)
		if len(parts) != 3 || (parts[0] != ";ook" && parts[0] != ";fsk") || parts[2] != "pulses" {
			p.reject("invalid_header", "Expected a pulse header of the form ;ook N pulses or ;fsk N pulses")
		} else {
			n, err := strconv.Atoi(parts[1])
			p.expected = n
			if err != nil || n < 0 {
				p.reject("invalid_header", "Pulse header has an invalid pulse count")
			}
			if n > 4096 {
				p.reject("pulse_count_limit", "Pulse header exceeds the 4096-row limit")
			}
		}
		if p.version != "1" {
			p.reject("invalid_version", "Expected rtl_433 pulse stream version 1")
		}
		if p.timescale != "1us" {
			p.reject("invalid_timescale", "Expected an explicit pulse timescale of 1us; units were not guessed")
		}
		p.buf.WriteString(";pulse data\n;version 1\n;timescale 1us\n")
		p.append(line)
		if prior != nil {
			prior.Stream = p.snapshot()
		}
		return prior, true
	}
	if strings.HasPrefix(text, ";version ") || strings.HasPrefix(text, ";timescale ") || text == ";pulse data" || strings.HasPrefix(text, ";received ") {
		var prior *PulsePacket
		if p.active {
			prior = p.finish("missing_end", "A stream/packet preamble arrived before the packet's ;end boundary")
		}
		if text == ";pulse data" {
			p.version = ""
			p.timescale = ""
		}
		if strings.HasPrefix(text, ";version ") {
			p.version = strings.TrimSpace(strings.TrimPrefix(text, ";version "))
		}
		if strings.HasPrefix(text, ";timescale ") {
			p.timescale = strings.ReplaceAll(strings.TrimSpace(strings.TrimPrefix(text, ";timescale ")), " ", "")
		}
		return prior, true
	}
	if text == ";end" {
		if !p.active {
			return &PulsePacket{ErrorCode: "orphan_end", Error: "Pulse ;end boundary arrived without a packet header", ObservedBytes: len(line) + 1, Stream: p.snapshot()}, true
		}
		p.append(line)
		p.stream.Completed++
		return p.finish("", ""), true
	}
	if !p.active {
		if strings.HasPrefix(text, ";") {
			return nil, true
		}
		// Never silently interpret unframed timings as an accepted recording.
		return &PulsePacket{ErrorCode: "unframed_data", Error: "Non-JSON stdout data arrived outside a framed pulse packet", ObservedBytes: len(line) + 1, Stream: p.snapshot()}, true
	}
	p.append(line)
	if !strings.HasPrefix(text, ";") {
		p.count++
		p.stream.PulseRows++
		valid := true
		parts := strings.Fields(text)
		if len(parts) != 2 {
			valid = false
			p.reject("invalid_timing", fmt.Sprintf("Pulse row %d must contain exactly two integer durations", p.count))
		} else {
			for _, part := range parts {
				n, err := strconv.ParseUint(part, 10, 32)
				if err != nil {
					valid = false
					p.reject("invalid_timing", fmt.Sprintf("Pulse row %d contains a non-integer or negative duration", p.count))
				} else if n > 10000000 {
					valid = false
					p.reject("timing_limit", fmt.Sprintf("Pulse row %d exceeds the duration limit", p.count))
				}
			}
		}
		if valid {
			p.validRows++
		}
		if p.count > 4096 {
			p.reject("pulse_count_limit", "Pulse packet exceeded the 4096-row limit")
		}
	}
	return nil, true
}

func ValidatePulseCapture(packets []PulsePacket) ([]byte, error) {
	if len(packets) == 0 || len(packets) > MaxCapturePackets {
		return nil, errors.New("capture packet limit exceeded")
	}
	var out bytes.Buffer
	for _, packet := range packets {
		if packet.Error != "" || packet.ErrorCode != "" || len(packet.Data) == 0 || !packet.OOK || packet.Progress || packet.Cleanup {
			return nil, errors.New("only bounded, complete OOK pulse captures are supported")
		}
		if out.Len()+len(packet.Data) > MaxCaptureBytes {
			return nil, errors.New("capture size limit exceeded")
		}
		out.Write(packet.Data)
	}
	return out.Bytes(), nil
}
