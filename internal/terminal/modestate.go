package terminal

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// modeState follows the terminal state set by the bytes the replay ring has
// evicted, so a rebuilt terminal gets it back as a preamble: it holds the state
// at the ring head, not the current one, because the tail replays everything
// after it. That is what keeps non-idempotent state right: the kitty keyboard
// stack, and an alternate screen entered before the head.
type modeState struct {
	parse parseState
	// raw holds the sequence in progress: CSI parameters and intermediates,
	// an OSC payload, or ESC intermediates. Past its cap the sequence is
	// dropped rather than replayed.
	raw      []byte
	overflow bool

	modes       []decMode
	saved       []decMode // XTSAVE, only values a fresh terminal does not hold
	alt         bool
	kitty       [2]kittyStack // primary, alternate
	cursorStyle int           // DECSCUSR, 0 for the default
	modifyOther bool          // modifyOtherKeys 2
	palette     map[int]string
	dynamic     [3]string // OSC 10, 11, 12
}

// softResetModes are the tracked DEC private modes DECSTR puts back to their
// default (libghostty-vt, src/terminal/Terminal.zig, softReset). The mouse
// modes are not among them: it leaves those as they are.
var softResetModes = []int{1, 6, 7, 25, 45, 66, 69, 1045}

// defaultOnModes are the DEC private modes libghostty-vt starts with set
// (src/terminal/modes.zig). An XTRESTORE with no XTSAVE goes to the default.
var defaultOnModes = []int{7, 25, 1007, 1035, 1036}

type parseState uint8

const (
	stGround parseState = iota
	stEscape
	stEscInter
	stCSI
	stOSC
	stString // DCS, SOS, PM, APC: never replayed
)

const (
	// rawCap bounds the in-progress sequence; a full OSC 4 palette is ~6 KB,
	// an OSC 52 clipboard or a kitty image chunk can be far longer and is
	// dropped, which changes no tracked state.
	rawCap = 8 << 10
	// modesCap bounds the distinct DEC private modes kept; a real program
	// sets a few dozen.
	modesCap = 128
	// kittyStackLen is libghostty-vt's fixed kitty keyboard stack
	// (src/terminal/kitty/key.zig, FlagStack.len): a push past it evicts the
	// oldest entry and a pop of at least it clears the stack.
	kittyStackLen = 8
	kittyFlagsMax = 31 // five flag bits
	paletteSize   = 256
	// maxUTF8Continuations is how many continuation bytes a UTF-8 character
	// has at most.
	maxUTF8Continuations = 3
	// specialColors are the OSC 4 indexes past the palette that libghostty-vt
	// accepts (src/terminal/color.zig, Special).
	specialColors = 5
	// colorSpecMax bounds a color spec, and with it the preamble; the longest
	// form libghostty-vt reads is an X11 name of 22 bytes or an rgbi: triple.
	colorSpecMax   = 64
	maxCursorStyle = 6
	keypadMode     = 66 // DECNKM: ESC = and ESC > set the same libghostty-vt flag
)

// decMode is a DEC private mode as last set or reset. The modes are kept in
// the order they last changed: the mouse tracking modes and the mouse
// encodings each replace one another, so the last one set must replay last.
type decMode struct {
	n   int
	set bool
}

type kittyStack struct {
	flags [kittyStackLen]int
	idx   int
}

func (k *kittyStack) push(f int) {
	k.idx = (k.idx + 1) % kittyStackLen
	k.flags[k.idx] = f
}

func (k *kittyStack) pop(n int) {
	if n >= kittyStackLen {
		*k = kittyStack{}
		return
	}
	for range n {
		k.flags[k.idx] = 0
		k.idx = (k.idx + kittyStackLen - 1) % kittyStackLen
	}
}

func (k *kittyStack) set(f, mode int) {
	switch mode {
	case 2:
		k.flags[k.idx] |= f
	case 3:
		k.flags[k.idx] &^= f
	default:
		k.flags[k.idx] = f
	}
}

func (s *modeState) feed(p []byte) {
	for i := 0; i < len(p); i++ {
		// Text and CSI parameters are most of the stream: take them in runs.
		switch s.parse {
		case stGround:
			e := bytes.IndexByte(p[i:], 0x1b)
			if e < 0 {
				return
			}
			i += e
		case stCSI:
			j := i
			for j < len(p) && p[j] >= 0x20 && p[j] < 0x40 {
				j++
			}
			s.collectRun(p[i:j])
			i = j
		}
		if i == len(p) {
			return
		}
		s.step(p[i])
	}
}

func (s *modeState) step(c byte) {
	if c == 0x18 || c == 0x1a { // CAN, SUB abort any sequence, an OSC undispatched
		s.parse = stGround
		return
	}
	switch s.parse {
	case stGround:
		if c == 0x1b {
			s.begin(stEscape)
		}
	case stEscape, stEscInter:
		s.escape(c)
	case stCSI:
		s.csi(c)
	case stOSC:
		s.osc(c)
	case stString:
		if c == 0x1b {
			s.begin(stEscape)
		}
	}
}

func (s *modeState) begin(st parseState) {
	s.parse = st
	s.raw = s.raw[:0]
	s.overflow = false
}

func (s *modeState) collect(c byte) {
	if len(s.raw) >= rawCap {
		s.overflow = true
		return
	}
	s.raw = append(s.raw, c)
}

func (s *modeState) collectRun(p []byte) {
	if len(s.raw)+len(p) > rawCap {
		s.overflow = true
		return
	}
	s.raw = append(s.raw, p...)
}

func (s *modeState) escape(c byte) {
	switch {
	case c == 0x1b:
		s.begin(stEscape)
	case c < 0x20 || c == 0x7f:
	case c < 0x30:
		s.parse = stEscInter
		s.collect(c)
	case s.parse == stEscInter:
		s.parse = stGround
		s.escDispatch(c)
	case c == '[':
		s.begin(stCSI)
	case c == ']':
		s.begin(stOSC)
	case c == 'P' || c == 'X' || c == '^' || c == '_':
		s.begin(stString)
	default:
		s.parse = stGround
		s.escDispatch(c)
	}
}

func (s *modeState) escDispatch(final byte) {
	if len(s.raw) > 0 {
		return
	}
	switch final {
	case 'c':
		*s = *newModeState()
	case '=':
		s.setMode(keypadMode, true)
	case '>':
		s.setMode(keypadMode, false)
	}
}

func (s *modeState) csi(c byte) {
	switch {
	case c == 0x1b:
		s.begin(stEscape)
	case c < 0x20 || c == 0x7f:
	case c >= 0x40 && c <= 0x7e:
		s.parse = stGround
		if !s.overflow {
			s.csiDispatch(c)
		}
	default:
		s.collect(c)
	}
}

// osc follows libghostty-vt's parser, which dispatches on every exit from the
// string but CAN and SUB (src/terminal/osc.zig, end): the ESC of an ST ends it,
// and so does the ESC of any other sequence.
func (s *modeState) osc(c byte) {
	switch {
	case c == 0x07:
		s.parse = stGround
		s.oscEnd()
	case c == 0x1b:
		s.oscEnd()
		s.begin(stEscape)
	case c >= 0x20:
		s.collect(c)
	}
}

func (s *modeState) oscEnd() {
	if !s.overflow {
		s.oscDispatch(string(s.raw))
	}
}

func newModeState() *modeState {
	return &modeState{palette: map[int]string{}}
}

// csiParams splits a CSI's parameters the way libghostty-vt's parser does: an
// empty string has none, ':' separates like ';' and an empty field is 0.
func csiParams(raw string) []int {
	if raw == "" {
		return nil
	}
	var out []int
	for f := range strings.SplitSeq(strings.ReplaceAll(raw, ":", ";"), ";") {
		n, err := strconv.Atoi(f)
		if err != nil {
			n = 0
		}
		out = append(out, n)
	}
	return out
}

// csiTracked is whether a CSI can change tracked state. Checked before the
// parameters are copied: SGR and cursor moves are most of a repaint.
func csiTracked(final byte, raw []byte) bool {
	switch final {
	case 'h', 'l', 'q', 'u':
		return true
	case 'm', 'n':
		return len(raw) > 0 && raw[0] == '>'
	case 'r', 's':
		return len(raw) > 0 && raw[0] == '?'
	case 'p':
		return string(raw) == "!"
	}
	return false
}

func (s *modeState) csiDispatch(final byte) {
	if !csiTracked(final, s.raw) {
		return
	}
	if final == 'p' {
		s.softReset()
		return
	}
	raw := string(s.raw)
	prefix := byte(0)
	if raw != "" && raw[0] >= 0x3c && raw[0] <= 0x3f {
		prefix, raw = raw[0], raw[1:]
	}
	if final == 'q' {
		var ok bool
		if raw, ok = strings.CutSuffix(raw, " "); !ok || prefix != 0 {
			return
		}
	}
	// Anything else, intermediates included, is a sequence libghostty-vt
	// ignores; so is a colon outside SGR and XTMODKEYS.
	if strings.Trim(raw, "0123456789;:") != "" || (final != 'm' && strings.Contains(raw, ":")) {
		return
	}
	s.csiApply(prefix, final, csiParams(raw))
}

func (s *modeState) csiApply(prefix, final byte, p []int) {
	switch {
	case prefix == '?' && (final == 'h' || final == 'l'):
		for _, n := range p {
			s.setMode(n, final == 'h')
		}
	case prefix == '?' && final == 's':
		for _, n := range p {
			s.saveMode(n)
		}
	case prefix == '?' && final == 'r':
		for _, n := range p {
			s.restoreMode(n)
		}
	case prefix == 0 && final == 'q':
		s.setCursorStyle(p)
	case prefix == '>' && final == 'm':
		s.modifyKeys(p)
	case prefix == '>' && final == 'n':
		s.modifyOther = false
	case final == 'u':
		s.kittyKeyboard(prefix, p)
	}
}

func (s *modeState) setCursorStyle(p []int) {
	switch {
	case len(p) == 0:
		s.cursorStyle = 0
	case len(p) == 1 && p[0] <= maxCursorStyle:
		s.cursorStyle = p[0]
	}
}

func (s *modeState) setMode(n int, set bool) {
	if n <= 0 {
		return
	}
	switch n {
	case 47, 1047, 1049:
		s.alt = set
	}
	s.modes = withMode(s.modes, decMode{n, set})
}

// withMode puts m last in list, replacing the entry of its number; a full
// list loses its oldest.
func withMode(list []decMode, m decMode) []decMode {
	list = slices.DeleteFunc(list, func(e decMode) bool { return e.n == m.n })
	if len(list) == modesCap {
		list = list[1:]
	}
	return append(list, m)
}

func modeDefault(n int) bool { return slices.Contains(defaultOnModes, n) }

func modeValue(list []decMode, n int) bool {
	if i := slices.IndexFunc(list, func(e decMode) bool { return e.n == n }); i >= 0 {
		return list[i].set
	}
	return modeDefault(n)
}

func (s *modeState) saveMode(n int) {
	if n <= 0 {
		return
	}
	v := modeValue(s.modes, n)
	if v == modeDefault(n) {
		s.saved = slices.DeleteFunc(s.saved, func(e decMode) bool { return e.n == n })
		return
	}
	s.saved = withMode(s.saved, decMode{n, v})
}

func (s *modeState) restoreMode(n int) {
	s.setMode(n, modeValue(s.saved, n))
}

func (s *modeState) softReset() {
	for i, m := range s.modes {
		if slices.Contains(softResetModes, m.n) {
			s.modes[i].set = modeDefault(m.n)
		}
	}
	s.cursorStyle = 0
	s.modifyOther = false
	clear(s.palette)
}

// modifyKeys is libghostty-vt's one modify_key_format slot: every valid
// sequence replaces it, and only the numeric other-keys form leaves it on.
func (s *modeState) modifyKeys(p []int) {
	if len(p) == 0 {
		s.modifyOther = false
		return
	}
	if len(p) > 2 || (p[0] != 0 && p[0] != 1 && p[0] != 2 && p[0] != 4) {
		return
	}
	s.modifyOther = p[0] == 4 && len(p) == 2 && p[1] == 2
}

// kittyKeyboard reads parameters as libghostty-vt does (src/terminal/stream.zig):
// push and pop only take one when it is the only one.
func (s *modeState) kittyKeyboard(prefix byte, p []int) {
	k := &s.kitty[0]
	if s.alt {
		k = &s.kitty[1]
	}
	switch prefix {
	case '>':
		flags := 0
		if len(p) == 1 {
			flags = p[0]
		}
		if flags <= kittyFlagsMax {
			k.push(flags)
		}
	case '<':
		n := 1
		if len(p) == 1 {
			n = p[0]
		}
		k.pop(n)
	case '=':
		s.kittySet(k, p)
	}
}

func (s *modeState) kittySet(k *kittyStack, p []int) {
	flags, mode := 0, 1
	if len(p) >= 1 {
		flags = p[0]
	}
	if len(p) >= 2 {
		mode = p[1]
	}
	if flags <= kittyFlagsMax && mode >= 1 && mode <= 3 {
		k.set(flags, mode)
	}
}

func (s *modeState) oscDispatch(payload string) {
	num, rest, _ := strings.Cut(payload, ";")
	n, err := strconv.Atoi(num)
	if err != nil {
		return
	}
	// libghostty-vt tokenizes the body: an empty field is not a field.
	f := strings.FieldsFunc(rest, func(r rune) bool { return r == ';' })
	switch {
	case n == 4:
		s.setPalette(f)
	case n == 104:
		s.resetPalette(f)
	case n >= 10 && n <= 12:
		s.setDynamic(n, f)
	case n >= 110 && n <= 112 && len(f) == 0:
		s.dynamic[n-110] = ""
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
		if n+i > 12 {
			return
		}
		spec := strings.Trim(field, " \t")
		if field == "?" {
			continue
		}
		if !validColor(spec) {
			return
		}
		s.dynamic[n+i-10] = spec
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

// preamble is the bytes that bring a fresh terminal to the state at the ring
// head, ending with the start of the sequence the head cut in two, when it can
// be replayed, so the tail completes it.
func (s *modeState) preamble() []byte {
	var b strings.Builder
	s.kitty[0].replay(&b)
	if s.kitty[1] != (kittyStack{}) {
		b.WriteString("\x1b[?1047h")
		s.kitty[1].replay(&b)
		b.WriteString("\x1b[?1047l")
	}
	// A saved value is set to be saved; the modes below then put each one back.
	for _, m := range s.saved {
		writeMode(&b, m)
		fmt.Fprintf(&b, "\x1b[?%ds", m.n)
	}
	for _, m := range s.modes {
		writeMode(&b, m)
	}
	if s.cursorStyle != 0 {
		fmt.Fprintf(&b, "\x1b[%d q", s.cursorStyle)
	}
	if s.modifyOther {
		b.WriteString("\x1b[>4;2m")
	}
	s.replayColors(&b)
	s.replayPartial(&b)
	return []byte(b.String())
}

func writeMode(b *strings.Builder, m decMode) {
	final := 'l'
	if m.set {
		final = 'h'
	}
	fmt.Fprintf(b, "\x1b[?%d%c", m.n, final)
}

// replay rebuilds the stack on a fresh one, which every kitty operation sees
// only relative to its top: pushing each entry from the one above the top
// round to the top itself leaves the same stack, rotated.
func (k *kittyStack) replay(b *strings.Builder) {
	if *k == (kittyStack{}) {
		return
	}
	for i := 1; i <= kittyStackLen; i++ {
		fmt.Fprintf(b, "\x1b[>%du", k.flags[(k.idx+i)%kittyStackLen])
	}
}

func (s *modeState) replayColors(b *strings.Builder) {
	for _, i := range slices.Sorted(maps.Keys(s.palette)) {
		fmt.Fprintf(b, "\x1b]4;%d;%s\x1b\\", i, s.palette[i])
	}
	for i, spec := range s.dynamic {
		if spec != "" {
			fmt.Fprintf(b, "\x1b]%d;%s\x1b\\", 10+i, spec)
		}
	}
}

func (s *modeState) replayPartial(b *strings.Builder) {
	if !s.resumable() {
		return
	}
	b.WriteString("\x1b")
	switch s.parse {
	case stEscInter:
		b.Write(s.raw)
	case stCSI:
		b.WriteByte('[')
		b.Write(s.raw)
	case stOSC:
		b.WriteByte(']')
		b.Write(s.raw)
	}
}

func (s *modeState) resumable() bool {
	switch s.parse {
	case stEscape, stEscInter, stCSI, stOSC:
		return !s.overflow
	}
	return false
}

// skip is how many bytes at the head of tail to drop when the preamble carries
// the cut sequence: none if it resumes it.
func (s *modeState) skip(tail []byte) int {
	if s.resumable() {
		return 0
	}
	return s.toGround(tail)
}

// toGround is how many bytes at the head of tail to drop so it starts outside
// the sequence the head cut in two. An ESC ends that sequence and opens the
// next one, which stays; a whole ST goes with it.
func (s *modeState) toGround(tail []byte) int {
	if s.parse == stGround {
		n := 0
		for n < len(tail) && n < maxUTF8Continuations && tail[n]&0xc0 == 0x80 {
			n++
		}
		return n
	}
	probe := newModeState()
	probe.parse, probe.raw, probe.overflow = s.parse, slices.Clone(s.raw), s.overflow
	for i, c := range tail {
		if c == 0x1b {
			if i+1 < len(tail) && tail[i+1] == '\\' {
				return i + 2
			}
			return i
		}
		probe.step(c)
		if probe.parse == stGround {
			return i + 1
		}
	}
	return len(tail)
}
