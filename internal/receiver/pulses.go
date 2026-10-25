package receiver

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const MaxCaptureBytes = 256 * 1024
const MaxCapturePackets = 32

// PulsePacket is an actual rtl_433 pulse dump, never an analyzer's guessed bits.
// It is kept in memory while learning and replayed from a private temporary file.
type PulsePacket struct {
	Data   []byte
	OOK    bool
	Pulses int
	Error  string
}

type pulseAssembler struct {
	buf              bytes.Buffer
	active, ook, bad bool
	expected, count  int
}

func (p *pulseAssembler) feed(line []byte) (*PulsePacket, bool) {
	text := strings.TrimSpace(string(line))
	if strings.HasPrefix(text, ";ook ") || strings.HasPrefix(text, ";fsk ") {
		*p = pulseAssembler{active: true, ook: strings.HasPrefix(text, ";ook ")}
		_, err := fmt.Sscanf(text[5:], "%d pulses", &p.expected)
		if err != nil || p.expected < 1 || p.expected > 4096 {
			p.bad = true
		}
		p.buf.WriteString(";pulse data\n;version 1\n;timescale 1us\n")
		p.buf.Write(line)
		p.buf.WriteByte('\n')
		return nil, true
	}
	if !p.active {
		return nil, strings.HasPrefix(text, ";")
	}
	if text == ";end" {
		p.buf.WriteString(";end\n")
		packet := &PulsePacket{Data: append([]byte(nil), p.buf.Bytes()...), OOK: p.ook, Pulses: p.count}
		if p.bad || p.count != p.expected {
			packet.Data = nil
			packet.Error = "Capture exceeded limits or contained invalid pulse data"
		}
		*p = pulseAssembler{}
		return packet, true
	}
	// Decoded JSON can share stdout with the dump; leave it to the JSON path.
	if strings.HasPrefix(text, "{") {
		return nil, false
	}
	if !strings.HasPrefix(text, ";") {
		parts := strings.Fields(text)
		if len(parts) != 2 {
			p.bad = true
		} else {
			for _, part := range parts {
				n, e := strconv.ParseUint(part, 10, 32)
				if e != nil || n > 10000000 {
					p.bad = true
				}
			}
		}
		p.count++
	}
	if p.buf.Len()+len(line)+1 > MaxCaptureBytes || p.count > 4096 {
		p.bad = true
	}
	if !p.bad {
		p.buf.Write(line)
		p.buf.WriteByte('\n')
	}
	return nil, true
}

func ValidatePulseCapture(packets []PulsePacket) ([]byte, error) {
	if len(packets) == 0 || len(packets) > MaxCapturePackets {
		return nil, errors.New("capture packet limit exceeded")
	}
	var out bytes.Buffer
	for _, packet := range packets {
		if packet.Error != "" || len(packet.Data) == 0 || !packet.OOK {
			return nil, errors.New("only bounded OOK pulse captures are supported")
		}
		if out.Len()+len(packet.Data) > MaxCaptureBytes {
			return nil, errors.New("capture size limit exceeded")
		}
		out.Write(packet.Data)
	}
	return out.Bytes(), nil
}
