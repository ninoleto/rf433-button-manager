package receiver

import (
	"encoding/json"
	"math"
	"regexp"
	"strconv"
)

// Levels are observations only. They never participate in signal identity.
// rtl_433 reports relative dBFS at the current tuner gain, not calibrated dBm.
type SignalLevels struct {
	RSSI  *float64 `json:"rssi_dbfs,omitempty"`
	SNR   *float64 `json:"snr_db,omitempty"`
	Noise *float64 `json:"noise_dbfs,omitempty"`
}

type NoiseReading struct {
	DBFS float64 `json:"dbfs"`
	At   string  `json:"at"`
}

func finiteLevel(raw json.RawMessage, minimum, maximum float64) *float64 {
	var n *float64
	if json.Unmarshal(raw, &n) != nil || n == nil || math.IsNaN(*n) || math.IsInf(*n, 0) || *n < minimum || *n > maximum {
		return nil
	}
	return n
}

// ParseSignalLevels must run on the original event before identity normalization.
// Missing/invalid measurements remain absent instead of becoming a false zero.
func ParseSignalLevels(raw []byte) *SignalLevels {
	if len(raw) > 65536 {
		return nil
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	l := SignalLevels{RSSI: finiteLevel(m["rssi"], -200, 100), SNR: finiteLevel(m["snr"], 0, 200), Noise: finiteLevel(m["noise"], -200, 100)}
	if l.RSSI == nil && l.SNR == nil && l.Noise == nil {
		return nil
	}
	return &l
}

// Exact rtl_433 23.11 log format. Unknown/new formats are ignored, not guessed.
var noisePattern = regexp.MustCompile(`^Auto Level: Current (?:noise|signal) level (-?[0-9]+(?:\.[0-9]+)?) dB, estimated noise (-?[0-9]+(?:\.[0-9]+)?) dB$`)

func noiseLog(raw []byte) (float64, bool) {
	if len(raw) > 256 {
		return 0, false
	}
	m := noisePattern.FindSubmatch(raw)
	if m == nil {
		return 0, false
	}
	for _, part := range m[1:] {
		n, err := strconv.ParseFloat(string(part), 64)
		if err != nil || n < -200 || n > 100 {
			return 0, false
		}
	}
	n, _ := strconv.ParseFloat(string(m[2]), 64)
	return n, true
}
