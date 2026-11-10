package terminal

import (
	"bytes"
	"fmt"
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
	cursorStyle [2]int        // DECSCUSR per screen, 0 for the default
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
	// kittyColorKeysMax is libghostty-vt's limit on the keys of one OSC 21
	// (src/terminal/osc/parsers/kitty_color.zig, twice Kind.max): past it the
	// whole sequence is dropped.
	kittyColorKeysMax    = 526
	paletteSize          = 256
	maxUTF8Continuations = 3
	// specialColors are the OSC 4 indexes past the palette that libghostty-vt
	// accepts (src/terminal/color.zig, Special).
	specialColors = 5
	// colorSpecMax bounds a color spec, and with it the preamble; the longest
	// form libghostty-vt reads is an X11 name of 22 bytes or an rgbi: triple.
	colorSpecMax   = 64
	maxCursorStyle = 6
)

// DEC private modes the scanner acts on.
const (
	modeCursorBlink   = 12
	modeAltLegacy     = 47
	modeKeypad        = 66 // DECNKM: ESC = and ESC > set the same libghostty-vt flag
	modeAlt           = 1047
	modeAltSaveCursor = 1049
)

// OSC numbers the scanner follows.
const (
	oscPalette         = 4
	oscForeground      = 10
	oscCursorColor     = 12
	oscKittyColor      = 21
	oscPaletteReset    = 104
	oscResetForeground = 110
	oscResetCursor     = 112
)

const (
	chBEL = 0x07
	chCAN = 0x18
	chSUB = 0x1a
	chESC = 0x1b
)

const (
	primaryScreen = iota
	altScreen
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
		// Text, CSI parameters and string payloads are most of the stream:
		// take them in runs.
		switch s.parse {
		case stGround:
			i += firstOf(p[i:], chESC)
		case stCSI:
			j := i
			for j < len(p) && p[j] >= 0x20 && p[j] < 0x40 {
				j++
			}
			s.collectRun(p[i:j])
			i = j
		case stOSC:
			end := i + firstOf(p[i:], chESC, chBEL, chCAN, chSUB)
			s.collectPayload(p[i:end])
			i = end
		case stString:
			i += firstOf(p[i:], chESC, chCAN, chSUB)
		}
		if i >= len(p) {
			return
		}
		s.step(p[i])
	}
}

// firstOf is the index of the first needle in p, or len(p). IndexByte is
// vectorized, so narrowing with one pass per needle beats a byte loop.
func firstOf(p []byte, needles ...byte) int {
	end := len(p)
	for _, n := range needles {
		if i := bytes.IndexByte(p[:end], n); i >= 0 {
			end = i
		}
	}
	return end
}

func (s *modeState) step(c byte) {
	if c == chCAN || c == chSUB { // abort any sequence, an OSC undispatched
		s.parse = stGround
		return
	}
	switch s.parse {
	case stGround:
		if c == chESC {
			s.begin(stEscape)
		}
	case stEscape, stEscInter:
		s.escape(c)
	case stCSI:
		s.csi(c)
	case stOSC:
		s.osc(c)
	case stString:
		if c == chESC {
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

// collectPayload takes an OSC's bytes up to its cap: past it they are only
// stepped over.
func (s *modeState) collectPayload(run []byte) {
	for _, c := range run {
		if s.overflow {
			return
		}
		if c >= 0x20 {
			s.collect(c)
		}
	}
}

func (s *modeState) escape(c byte) {
	switch {
	case c == chESC:
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
		s.setMode(modeKeypad, true)
	case '>':
		s.setMode(modeKeypad, false)
	}
}

func (s *modeState) csi(c byte) {
	switch {
	case c == chESC:
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
	case c == chBEL:
		s.parse = stGround
		s.oscEnd()
	case c == chESC:
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
		s.applyCursorStyle(0)
	case len(p) == 1 && p[0] <= maxCursorStyle:
		s.applyCursorStyle(p[0])
	}
}

// applyCursorStyle also writes the blink mode, as libghostty-vt's
// setCursorStyle does: the odd styles blink, and lich leaves the default
// style's blink at the library's off.
func (s *modeState) applyCursorStyle(style int) {
	s.cursorStyle[s.screen()] = style
	if blink := style%2 == 1; blink != modeValue(s.modes, modeCursorBlink) {
		s.setMode(modeCursorBlink, blink)
	}
}

func (s *modeState) screen() int {
	if s.alt {
		return altScreen
	}
	return primaryScreen
}

func (s *modeState) setMode(n int, set bool) {
	if n <= 0 {
		return
	}
	if n == modeAltLegacy || n == modeAlt || n == modeAltSaveCursor {
		s.switchScreen(n, set)
	}
	s.modes = withMode(s.modes, decMode{n, set})
}

// switchScreen copies the cursor style the way libghostty-vt's
// switchScreenMode copies the cursor: on every real switch but leaving 1049,
// which restores the primary screen's own.
func (s *modeState) switchScreen(n int, enter bool) {
	if s.alt == enter {
		return
	}
	s.alt = enter
	switch {
	case enter:
		s.cursorStyle[altScreen] = s.cursorStyle[primaryScreen]
	case n != modeAltSaveCursor:
		s.cursorStyle[primaryScreen] = s.cursorStyle[altScreen]
	}
}

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
	s.applyCursorStyle(0)
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
	k := &s.kitty[s.screen()]
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

// preamble is the bytes that bring a fresh terminal to the state at the ring
// head, ending with the start of the sequence the head cut in two, when it can
// be replayed, so the tail completes it.
func (s *modeState) preamble() []byte {
	var b strings.Builder
	s.kitty[primaryScreen].replay(&b)
	if s.kitty[altScreen] != (kittyStack{}) {
		writeMode(&b, decMode{modeAlt, true})
		s.kitty[altScreen].replay(&b)
		writeMode(&b, decMode{modeAlt, false})
	}
	// Before the modes: a style also sets blink, and the mode entry owns that.
	if s.cursorStyle[primaryScreen] != 0 {
		writeCursorStyle(&b, s.cursorStyle[primaryScreen])
	}
	// A saved value is set to be saved; the modes below then put each one back.
	for _, m := range s.saved {
		writeMode(&b, m)
		fmt.Fprintf(&b, "\x1b[?%ds", m.n)
	}
	for _, m := range s.modes {
		writeMode(&b, m)
	}
	// Entering the alternate screen copied the primary's style over it.
	if s.alt && s.cursorStyle[altScreen] != s.cursorStyle[primaryScreen] {
		writeCursorStyle(&b, s.cursorStyle[altScreen])
		writeMode(&b, decMode{modeCursorBlink, modeValue(s.modes, modeCursorBlink)})
	}
	if s.modifyOther {
		b.WriteString("\x1b[>4;2m")
	}
	s.replayColors(&b)
	s.replayPartial(&b)
	return []byte(b.String())
}

func writeCursorStyle(b *strings.Builder, style int) {
	fmt.Fprintf(b, "\x1b[%d q", style)
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
	probe := modeState{parse: s.parse}
	for i, c := range tail {
		if c == chESC {
			if i+1 < len(tail) && tail[i+1] == '\\' {
				return i + 2
			}
			return i
		}
		// Overflow stops the step dispatching: the probe has no palette to write.
		probe.overflow = true
		probe.step(c)
		if probe.parse == stGround {
			return i + 1
		}
	}
	return len(tail)
}
