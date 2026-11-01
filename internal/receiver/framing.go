package receiver

import (
	"strconv"
	"strings"
)

// ProfileFramingFallbacks supplies additional normal-reception decoders for a
// narrowly bounded legacy PWM framing error. Some analyzer suggestions placed
// the row-gap limit inside the long data-symbol timing band. Small timing changes
// then split a complete word into short rows, even though every bit is present.
//
// Keep the original decoder, measured pulse widths/tolerance, reset delimiter,
// exact 25-bit length and model. The extra decoder uses the reset delimiter to
// frame words. It cannot pad, repair or match a prefix of an incomplete word.
// Stored profiles and their timing-derived IDs remain unchanged. This is not
// used during learning: duplicate decoder output must never count as repeated
// air frames when validating a newly learned profile.
func ProfileFramingFallbacks(profiles []DecoderProfile) []string {
	var out []string
	for _, p := range profiles {
		if p.Bits != 25 || p.Validate(true) != nil ||
			!strings.HasPrefix(p.FlexDecoder, "n="+p.Model+",m=OOK_PWM,") ||
			!strings.HasSuffix(p.FlexDecoder, ",repeats>=1") {
			continue
		}
		values := map[string]int{}
		for _, part := range strings.Split(p.FlexDecoder, ",") {
			key, value, _ := strings.Cut(part, "=")
			values[key], _ = strconv.Atoi(value)
		}
		s, l, tolerance, gap := values["s"], values["l"], values["t"], values["g"]
		if values["y"] != 0 || tolerance <= 0 || 2*tolerance >= l-s ||
			gap <= s+tolerance || gap < l-tolerance || gap > l+tolerance ||
			values["r"] <= 4*(l+tolerance) {
			continue
		}
		out = append(out, strings.Replace(p.FlexDecoder, ",g="+strconv.Itoa(gap)+",", ",g=0,", 1))
	}
	return out
}
