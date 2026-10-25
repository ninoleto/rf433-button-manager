package receiver

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// measuredPWMTimings supplies sync-delimited PWM candidates when the upstream
// histogram heuristic cannot classify jittered/multiple-width marks. It never
// decodes bits: every candidate must pass rtl_433 replay and separate presses.
func measuredPWMTimings(capture []byte) []string {
	type bucket struct{ center, count int }
	cluster := func(values []int) []bucket {
		sort.Ints(values)
		var groups [][]int
		for _, v := range values {
			if v <= 0 || v > 1000000 {
				continue
			}
			if len(groups) == 0 || v > groups[len(groups)-1][len(groups[len(groups)-1])/2]*135/100+12 {
				groups = append(groups, []int{v})
			} else {
				groups[len(groups)-1] = append(groups[len(groups)-1], v)
			}
		}
		// Short marks can split into adjacent histogram bins as reception levels
		// change. Join only adjacent short bins; distinct data widths stay apart.
		for len(groups) > 1 && groups[1][len(groups[1])/2]*10 < groups[0][len(groups[0])/2]*18 {
			groups[0] = append(groups[0], groups[1]...)
			groups = append(groups[:1], groups[2:]...)
		}
		var out []bucket
		for _, g := range groups {
			if len(g) >= 2 {
				out = append(out, bucket{g[len(g)/2], len(g)})
			}
		}
		return out
	}
	var marks, gaps []int
	var out []string
	seen := map[string]bool{}
	ook := false
	inspect := func() {
		if len(marks) < 24 || len(out) >= 4 {
			return
		}
		bins := cluster(marks)
		if len(bins) < 3 || len(bins) > 6 || bins[0].center < 20 {
			return
		}
		short, long := bins[0].center, bins[1].center
		if long*10 < short*17 {
			return
		}
		// Require repeated frame separators, not merely a long mark inside an NRZ
		// data word. A final silence alone does not establish PWM framing.
		separators := 0
		for _, gap := range gaps[:len(gaps)-1] {
			if gap >= 4*long {
				separators++
			}
		}
		if separators < 2 {
			return
		}
		// Sync marks frame the rows. Put reset beyond the recorded gaps, including
		// inter-repeat gaps; rtl_433 also flushes at the end of every pulse packet.
		reset := 0
		for _, gap := range gaps {
			reset = max(reset, gap)
		}
		reset += 4
		if reset <= bins[len(bins)-1].center || reset > 1000000 {
			return
		}
		for i := len(bins) - 1; i >= 2 && len(out) < 4; i-- {
			sync := bins[i].center
			if sync*10 < long*15 {
				continue
			}
			timing := fmt.Sprintf("m=OOK_PWM,s=%d,l=%d,r=%d,g=0,t=0,y=%d", short, long, reset, sync)
			if !seen[timing] {
				seen[timing] = true
				out = append(out, timing)
			}
		}
	}
	for _, line := range strings.Split(string(capture), "\n") {
		if strings.HasPrefix(line, ";ook ") {
			marks, gaps, ook = nil, nil, true
		} else if strings.HasPrefix(line, ";fsk ") {
			marks, gaps, ook = nil, nil, false
		} else if line == ";end" {
			if ook {
				inspect()
			}
			marks, gaps, ook = nil, nil, false
		} else if ook && line != "" && !strings.HasPrefix(line, ";") {
			parts := strings.Fields(line)
			if len(parts) != 2 || len(marks) >= 4096 {
				return nil
			}
			mark, e1 := strconv.Atoi(parts[0])
			gap, e2 := strconv.Atoi(parts[1])
			if e1 != nil || e2 != nil || mark < 0 || gap < 0 || mark > 1000000 || gap > 1000000 {
				return nil
			}
			marks = append(marks, mark)
			gaps = append(gaps, gap)
		}
	}
	return out
}
