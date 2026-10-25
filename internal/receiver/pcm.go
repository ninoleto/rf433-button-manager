package receiver

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// measuredPCMTimings supplies bounded NRZ candidates when rtl_433's histogram
// analyzer cannot classify a packet. It does not decode bits. rtl_433 must still
// reproduce an unambiguous repeated code, then match three separately armed
// presses. Only complete, gap-delimited frames with a measured integer timing
// grid are eligible; incomplete leading/trailing fragments are not evidence.
func measuredPCMTimings(capture []byte) []string {
	type pair struct{ mark, space int }
	median := func(v []int) int {
		sort.Ints(v)
		return v[len(v)/2]
	}
	var rows []pair
	seen := map[string]bool{}
	var out []string
	ook := false
	inspect := func() {
		if len(rows) < 20 || len(out) >= 4 {
			return
		}
		gaps := make([]int, 0, len(rows))
		for _, p := range rows {
			gaps = append(gaps, p.space)
		}
		base := median(gaps)
		if base < 20 || base > 2000 {
			return
		}
		var frames [][]pair
		start := -1
		for i, p := range rows {
			if p.space < 6*base {
				continue
			}
			if start >= 0 && i-start+1 >= 8 {
				frames = append(frames, rows[start:i+1])
			}
			start = i + 1
		}
		if len(frames) < 2 {
			return
		}
		var cells []int
		for _, frame := range frames {
			for _, p := range frame[:len(frame)-1] {
				if p.mark < 2*base && p.space < 2*base {
					cells = append(cells, (p.mark+p.space+1)/2)
				}
			}
		}
		if len(cells) < 8 {
			return
		}
		unit := median(cells)
		if unit < 20 || unit > 2000 {
			return
		}
		maxGap, minBreak := 0, 1000001
		frameWidth := 0
		variedRuns := false
		fit := func(n int) bool {
			run := math.Round(float64(n) / float64(unit))
			return run >= 1 && run <= 16 && math.Abs(float64(n)-run*float64(unit)) <= 0.4*float64(unit)
		}
		for _, frame := range frames {
			width := 0
			for i, p := range frame {
				if !fit(p.mark) {
					return
				}
				width += int(math.Round(float64(p.mark) / float64(unit)))
				variedRuns = variedRuns || p.mark > 2*unit
				if i == len(frame)-1 {
					minBreak = min(minBreak, p.space)
					continue
				}
				period := float64(p.mark+p.space) / float64(unit)
				if !fit(p.space) || math.Abs(period-math.Round(period)) > 0.25 {
					return
				}
				maxGap = max(maxGap, p.space)
				width += int(math.Round(float64(p.space) / float64(unit)))
				variedRuns = variedRuns || p.space > 2*unit
			}
			if frameWidth != 0 && width != frameWidth {
				return // A repeating shorter fragment cannot hide different full frames.
			}
			frameWidth = width
		}
		if !variedRuns || minBreak < 4*maxGap || minBreak > 1000000 {
			return
		}
		reset := (maxGap + minBreak) / 2
		timing := fmt.Sprintf("m=OOK_PCM,s=%d,l=%d,r=%d,g=%d,t=0,y=0", unit, unit, reset, reset)
		if !seen[timing] {
			seen[timing] = true
			out = append(out, timing)
		}
	}
	for _, line := range strings.Split(string(capture), "\n") {
		if strings.HasPrefix(line, ";ook ") {
			rows = nil
			ook = true
		} else if strings.HasPrefix(line, ";fsk ") {
			rows = nil
			ook = false
		} else if line == ";end" {
			if ook {
				inspect()
			}
			rows = nil
			ook = false
		} else if ook && line != "" && !strings.HasPrefix(line, ";") {
			fields := strings.Fields(line)
			if len(fields) != 2 {
				return nil
			}
			mark, e1 := strconv.Atoi(fields[0])
			space, e2 := strconv.Atoi(fields[1])
			if e1 != nil || e2 != nil || mark < 0 || space < 0 {
				return nil
			}
			rows = append(rows, pair{mark, space})
		}
	}
	return out
}

// repeatedFrameWidth identifies a repeated complete row before discarding edge
// fragments. In-record repetition outranks fragments repeated once per packet.
// Different codes at the selected width, or two in-record repeated widths,
// remain ambiguous and are rejected.
func repeatedFrameWidth(frames []json.RawMessage) int {
	counts := map[int]map[string]int{}
	strong := map[int]bool{}
	for _, raw := range frames {
		var event struct {
			Codes []string `json:"codes"`
		}
		if json.Unmarshal(raw, &event) != nil {
			return 0
		}
		local := map[string]int{}
		for _, code := range event.Codes {
			m := codePattern.FindStringSubmatch(code)
			if m == nil {
				return 0
			}
			bits, _ := strconv.Atoi(m[1])
			if bits < 12 || bits > 64 || len(m[2]) != (bits+3)/4 {
				continue
			}
			code = strings.ToLower(code)
			if counts[bits] == nil {
				counts[bits] = map[string]int{}
			}
			counts[bits][code]++
			local[code]++
			if local[code] >= 2 {
				strong[bits] = true
			}
		}
	}
	chosen := 0
	for bits, codes := range counts {
		if len(strong) > 0 && !strong[bits] {
			continue
		}
		for _, count := range codes {
			if count < 2 {
				continue
			}
			if len(codes) != 1 || chosen != 0 {
				return 0
			}
			chosen = bits
		}
	}
	return chosen
}
