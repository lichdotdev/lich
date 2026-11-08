package terminal

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

func (s *modeState) oscDispatch(payload string) {
	num, rest, _ := strings.Cut(payload, ";")
	n, err := strconv.Atoi(num)
	if err != nil {
		return
	}
	if n == oscKittyColor {
		s.kittyColors(rest)
		return
	}
	// libghostty-vt tokenizes the body: an empty field is not a field.
	f := strings.FieldsFunc(rest, func(r rune) bool { return r == ';' })
	switch {
	case n == oscPalette:
		s.setPalette(f)
	case n == oscPaletteReset:
		s.resetPalette(f)
	case n >= oscForeground && n <= oscCursorColor:
		s.setDynamic(n, f)
	case n >= oscResetForeground && n <= oscResetCursor && len(f) == 0:
		s.dynamic[n-oscResetForeground] = ""
	}
}

// kittyColors reads an OSC 21 body as libghostty-vt does: an empty value
// resets, an unreadable pair is skipped, too many keys void it all. A spec of
// "" in a colorOp is a reset.
func (s *modeState) kittyColors(body string) {
	var ops []colorOp
	keys := 0
	for kv := range strings.SplitSeq(body, ";") {
		if keys >= kittyColorKeysMax {
			return
		}
		key, value, _ := strings.Cut(kv, "=")
		op, ok := kittyColorKey(key)
		value = strings.Trim(value, " ")
		if !ok || (value != "" && value != "?" && !validColor(strings.Trim(value, " \t"))) {
			continue
		}
		keys++
		if value != "?" {
			op.spec = strings.Trim(value, " \t")
			ops = append(ops, op)
		}
	}
	for _, op := range ops {
		s.applyColor(op)
	}
}

// colorOp is a palette entry or a dynamic color to set, or neither for a key
// libghostty-vt accepts and does not store.
type colorOp struct {
	palette, dynamic int
	spec             string
}

var kittySpecialColors = map[string]int{
	"foreground": 0, "background": 1, "cursor": 2,
	"selection_foreground": -1, "selection_background": -1, "cursor_text": -1,
	"visual_bell": -1, "second_transparent_background": -1,
}

func kittyColorKey(key string) (colorOp, bool) {
	if d, ok := kittySpecialColors[key]; ok {
		return colorOp{palette: -1, dynamic: d}, true
	}
	i, err := strconv.Atoi(key)
	if err != nil || i < 0 || i >= paletteSize {
		return colorOp{}, false
	}
	return colorOp{palette: i, dynamic: -1}, true
}

func (s *modeState) applyColor(op colorOp) {
	switch {
	case op.palette >= 0 && op.spec == "":
		delete(s.palette, op.palette)
	case op.palette >= 0:
		s.palette[op.palette] = op.spec
	case op.dynamic >= 0:
		s.dynamic[op.dynamic] = op.spec
	}
}

// setPalette stops at the first bad pair, as libghostty-vt does.
func (s *modeState) setPalette(f []string) {
	for i := 0; i+1 < len(f); i += 2 {
		idx, err := strconv.Atoi(f[i])
		if err != nil || idx < 0 || idx >= paletteSize+specialColors {
			return
		}
		spec := strings.Trim(f[i+1], " \t")
		if f[i+1] == "?" {
			continue
		}
		if !validColor(spec) {
			return
		}
		if idx < paletteSize {
			s.palette[idx] = spec
		}
	}
}

func (s *modeState) resetPalette(f []string) {
	reset := 0
	for _, field := range f {
		if idx, err := strconv.Atoi(field); err == nil && idx >= 0 && idx < paletteSize+specialColors {
			delete(s.palette, idx)
			reset++
		}
	}
	if reset == 0 {
		clear(s.palette)
	}
}

// setDynamic stops at the first bad spec; each spec after the first sets the
// next color, and only 10 to 12 are kept.
func (s *modeState) setDynamic(n int, f []string) {
	for i, field := range f {
		if n+i > oscCursorColor {
			return
		}
		spec := strings.Trim(field, " \t")
		if field == "?" {
			continue
		}
		if !validColor(spec) {
			return
		}
		s.dynamic[n+i-oscForeground] = spec
	}
}

// validColor accepts what libghostty-vt's RGB.parse does, and bounds the spec.
// shortcut: any short word passes as an X11 name, so an unknown name replays
// and is ignored, losing an earlier color; embed rgb.txt if that ever shows.
func validColor(spec string) bool {
	if spec == "" || len(spec) > colorSpecMax {
		return false
	}
	if hex, ok := strings.CutPrefix(spec, "#"); ok {
		return isHex(hex) && (len(hex) == 3 || len(hex) == 6 || len(hex) == 9 || len(hex) == 12)
	}
	if body, ok := strings.CutPrefix(spec, "rgb:"); ok {
		return validTriple(body, isHexChannel)
	}
	if body, ok := strings.CutPrefix(spec, "rgbi:"); ok {
		return validTriple(body, isIntensity)
	}
	return strings.Trim(strings.ToLower(spec), "abcdefghijklmnopqrstuvwxyz0123456789 ") == ""
}

func validTriple(body string, channel func(string) bool) bool {
	f := strings.Split(body, "/")
	return len(f) == 3 && channel(f[0]) && channel(f[1]) && channel(f[2])
}

func isHexChannel(v string) bool { return len(v) >= 1 && len(v) <= 4 && isHex(v) }

func isHex(v string) bool { return strings.Trim(strings.ToLower(v), "0123456789abcdef") == "" }

func isIntensity(v string) bool {
	if strings.Trim(v, "+-.0123456789") != "" {
		return false
	}
	f, err := strconv.ParseFloat(v, 64)
	return err == nil && f >= 0 && f <= 1
}

func (s *modeState) replayColors(b *strings.Builder) {
	for _, i := range slices.Sorted(maps.Keys(s.palette)) {
		fmt.Fprintf(b, "\x1b]%d;%d;%s\x1b\\", oscPalette, i, s.palette[i])
	}
	for i, spec := range s.dynamic {
		if spec != "" {
			fmt.Fprintf(b, "\x1b]%d;%s\x1b\\", oscForeground+i, spec)
		}
	}
}
