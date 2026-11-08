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

	modes     []decMode
	alt       bool
	kitty     [2]kittyStack // primary, alternate
	keypadApp bool
	cursor    string // DECSCUSR parameters
	margins   string // DECSTBM parameters, "" for the full screen
	modKeys   map[int]string
	palette   map[int]string
	dynamic   [3]string // OSC 10, 11, 12
	charsets  map[byte]string
	shiftOut  bool
}

type parseState uint8

const (
	stGround parseState = iota
	stEscape
	stEscInter
	stCSI
	stOSC
	stOSCEsc
	stString    // DCS, SOS, PM, APC: never replayed
	stStringEsc // ESC inside one of those strings
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
			i = s.text(p, i)
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
	if c == 0x18 || c == 0x1a { // CAN, SUB abort any sequence
		s.parse = stGround
		return
	}
	switch s.parse {
	case stGround:
		s.ground(c)
	case stEscape, stEscInter:
		s.escape(c)
	case stCSI:
		s.csi(c)
	case stOSC, stOSCEsc:
		s.osc(c)
	case stString, stStringEsc:
		s.str(c)
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

// text skips the run of text from p[i] to the next ESC, keeping the last
// shift it holds, and returns where the run ends.
func (s *modeState) text(p []byte, i int) int {
	end := len(p)
	if e := bytes.IndexByte(p[i:], 0x1b); e >= 0 {
		end = i + e
	}
	run := p[i:end]
	// LastIndexByte is a plain loop and IndexByte is vectorized: look first.
	if bytes.IndexByte(run, 0x0e) < 0 && bytes.IndexByte(run, 0x0f) < 0 {
		return end
	}
	if so, si := bytes.LastIndexByte(run, 0x0e), bytes.LastIndexByte(run, 0x0f); so != si {
		s.shiftOut = so > si
	}
	return end
}

func (s *modeState) collectRun(p []byte) {
	if len(s.raw)+len(p) > rawCap {
		s.overflow = true
		return
	}
	s.raw = append(s.raw, p...)
}

func (s *modeState) ground(c byte) {
	switch c {
	case 0x1b:
		s.begin(stEscape)
	case 0x0e:
		s.shiftOut = true
	case 0x0f:
		s.shiftOut = false
	}
}

func (s *modeState) escape(c byte) {
	switch {
	case c == 0x1b:
		s.begin(stEscape)
	case c < 0x20:
		s.ground(c)
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
	switch {
	case len(s.raw) > 0 && strings.IndexByte("()*+-./", s.raw[0]) >= 0:
		if !s.overflow {
			s.charsets[s.raw[0]] = string(s.raw[1:]) + string(final)
		}
	case len(s.raw) > 0:
	case final == 'c':
		*s = modeState{raw: s.raw[:0]}
		s.init()
	case final == '=':
		s.keypadApp = true
	case final == '>':
		s.keypadApp = false
	}
}

func (s *modeState) csi(c byte) {
	switch {
	case c == 0x1b:
		s.begin(stEscape)
	case c < 0x20:
		s.ground(c)
	case c >= 0x40 && c <= 0x7e:
		s.parse = stGround
		if !s.overflow {
			s.csiDispatch(c)
		}
	default:
		s.collect(c)
	}
}

func (s *modeState) osc(c byte) {
	switch {
	case s.parse == stOSCEsc && c == '\\':
		s.parse = stGround
		s.oscEnd()
	case s.parse == stOSCEsc:
		s.begin(stEscape)
		s.escape(c)
	case c == 0x07:
		s.parse = stGround
		s.oscEnd()
	case c == 0x1b:
		s.parse = stOSCEsc
	case c >= 0x20:
		s.collect(c)
	}
}

func (s *modeState) oscEnd() {
	if !s.overflow {
		s.oscDispatch(string(s.raw))
	}
}

func (s *modeState) str(c byte) {
	switch {
	case s.parse == stStringEsc && c == '\\':
		s.parse = stGround
	case s.parse == stStringEsc:
		s.begin(stEscape)
		s.escape(c)
	case c == 0x1b:
		s.parse = stStringEsc
	}
}

func (s *modeState) init() {
	if s.modKeys == nil {
		s.modKeys = map[int]string{}
		s.palette = map[int]string{}
		s.charsets = map[byte]string{}
	}
}

func newModeState() *modeState {
	s := &modeState{}
	s.init()
	return s
}

// csiParams splits a CSI's parameters into numbers, a missing one as def.
func csiParams(p string, def int) []int {
	var out []int
	for f := range strings.SplitSeq(p, ";") {
		f, _, _ = strings.Cut(f, ":")
		n, err := strconv.Atoi(f)
		if err != nil {
			n = def
		}
		out = append(out, n)
	}
	return out
}

func (s *modeState) csiDispatch(final byte) {
	if strings.IndexByte("hlqrmu", final) < 0 || final == 'm' && (len(s.raw) == 0 || s.raw[0] != '>') {
		return // SGR and cursor moves, most of a repaint
	}
	raw := string(s.raw)
	prefix := byte(0)
	if raw != "" && raw[0] >= 0x3c && raw[0] <= 0x3f {
		prefix, raw = raw[0], raw[1:]
	}
	switch {
	case prefix == '?' && (final == 'h' || final == 'l'):
		for _, n := range csiParams(raw, -1) {
			s.setMode(n, final == 'h')
		}
	case prefix == 0 && final == 'q' && strings.HasSuffix(raw, " "):
		s.cursor = strings.TrimSuffix(raw, " ")
	case prefix == 0 && final == 'r':
		s.margins = raw
	case prefix == '>' && final == 'm':
		s.modifyKeys(raw)
	case final == 'u':
		s.kittyKeyboard(prefix, raw)
	}
}

func (s *modeState) setMode(n int, set bool) {
	if n < 0 {
		return
	}
	switch n {
	case 47, 1047, 1049:
		s.alt = set
	}
	s.modes = slices.DeleteFunc(s.modes, func(m decMode) bool { return m.n == n })
	if len(s.modes) == modesCap {
		s.modes = s.modes[1:]
	}
	s.modes = append(s.modes, decMode{n, set})
}

func (s *modeState) modifyKeys(raw string) {
	if raw == "" {
		clear(s.modKeys)
		return
	}
	res, val, hasVal := strings.Cut(raw, ";")
	r, err := strconv.Atoi(res)
	if err != nil || r < 0 || r > 7 {
		return
	}
	if !hasVal {
		delete(s.modKeys, r)
		return
	}
	s.modKeys[r] = val
}

func (s *modeState) kittyKeyboard(prefix byte, raw string) {
	k := &s.kitty[0]
	if s.alt {
		k = &s.kitty[1]
	}
	p := csiParams(raw, -1)
	arg := func(i, def int) int {
		if i < len(p) && p[i] >= 0 {
			return p[i]
		}
		return def
	}
	flags := arg(0, 0)
	switch {
	case prefix == '<':
		k.pop(arg(0, 1))
	case flags > kittyFlagsMax: // libghostty-vt drops the sequence
	case prefix == '>':
		k.push(flags)
	case prefix == '=':
		k.set(flags, arg(1, 1))
	}
}

func (s *modeState) oscDispatch(payload string) {
	num, rest, _ := strings.Cut(payload, ";")
	n, err := strconv.Atoi(num)
	if err != nil {
		return
	}
	switch {
	case n == 4:
		s.setPalette(rest)
	case n == 104:
		s.resetPalette(rest)
	case n >= 10 && n <= 12:
		for i, spec := range strings.Split(rest, ";") {
			if n+i <= 12 && spec != "?" {
				s.dynamic[n+i-10] = spec
			}
		}
	case n >= 110 && n <= 112:
		s.dynamic[n-110] = ""
	}
}

func (s *modeState) setPalette(rest string) {
	f := strings.Split(rest, ";")
	for i := 0; i+1 < len(f); i += 2 {
		idx, err := strconv.Atoi(f[i])
		if err == nil && idx >= 0 && idx < paletteSize && f[i+1] != "?" {
			s.palette[idx] = f[i+1]
		}
	}
}

func (s *modeState) resetPalette(rest string) {
	if rest == "" {
		clear(s.palette)
		return
	}
	for f := range strings.SplitSeq(rest, ";") {
		if idx, err := strconv.Atoi(f); err == nil {
			delete(s.palette, idx)
		}
	}
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
	for _, m := range s.modes {
		final := 'l'
		if m.set {
			final = 'h'
		}
		fmt.Fprintf(&b, "\x1b[?%d%c", m.n, final)
	}
	if s.keypadApp {
		b.WriteString("\x1b=")
	}
	if s.margins != "" {
		fmt.Fprintf(&b, "\x1b[%sr", s.margins)
	}
	if s.cursor != "" {
		fmt.Fprintf(&b, "\x1b[%s q", s.cursor)
	}
	for _, r := range sortedKeys(s.modKeys) {
		fmt.Fprintf(&b, "\x1b[>%d;%sm", r, s.modKeys[r])
	}
	s.replayColors(&b)
	for _, g := range sortedKeys(s.charsets) {
		fmt.Fprintf(&b, "\x1b%c%s", g, s.charsets[g])
	}
	if s.shiftOut {
		b.WriteByte(0x0e)
	}
	s.replayPartial(&b)
	return []byte(b.String())
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
	for _, i := range sortedKeys(s.palette) {
		fmt.Fprintf(b, "\x1b]4;%d;%s\x1b\\", i, s.palette[i])
	}
	for i, spec := range s.dynamic {
		if spec != "" {
			fmt.Fprintf(b, "\x1b]%d;%s\x1b\\", 10+i, spec)
		}
	}
}

func (s *modeState) replayPartial(b *strings.Builder) {
	switch {
	case s.parse == stStringEsc || s.parse == stOSCEsc && s.overflow:
		// The ESC ends the string, as ST or by starting the next sequence;
		// a lone ST is a no-op.
		b.WriteString("\x1b")
		return
	case s.overflow:
		return
	}
	switch s.parse {
	case stEscape:
		b.WriteString("\x1b")
	case stEscInter:
		b.WriteString("\x1b" + string(s.raw))
	case stCSI:
		b.WriteString("\x1b[" + string(s.raw))
	case stOSC:
		b.WriteString("\x1b]" + string(s.raw))
	case stOSCEsc:
		b.WriteString("\x1b]" + string(s.raw) + "\x1b")
	}
}

// skip is how many bytes at the head of tail to drop to reach a point a fresh
// terminal can start from: the rest of a sequence too long to replay, of a
// DCS, SOS, PM or APC string, or of a UTF-8 character.
func (s *modeState) skip(tail []byte) int {
	switch {
	case s.parse == stString || (s.overflow && s.parse == stOSC):
		return skipString(tail, s.parse == stOSC)
	case s.overflow && s.parse == stCSI:
		for i, c := range tail {
			switch {
			case c == 0x1b:
				return i
			case c >= 0x40 && c <= 0x7e:
				return i + 1
			}
		}
		return len(tail)
	case s.parse == stGround:
		n := 0
		for n < len(tail) && n < 3 && tail[n]&0xc0 == 0x80 {
			n++
		}
		return n
	}
	return 0
}

// toGround is how many bytes at the head of tail to drop so it starts outside
// any sequence, the one the head cut in two included: skip keeps a sequence
// the preamble resumes, and a reader without a preamble cannot resume it.
func (s *modeState) toGround(tail []byte) int {
	if s.parse == stGround {
		return s.skip(tail)
	}
	probe := newModeState()
	probe.parse, probe.raw, probe.overflow = s.parse, slices.Clone(s.raw), s.overflow
	i := 0
	for i < len(tail) && probe.parse != stGround {
		probe.step(tail[i])
		i++
	}
	return i
}

// skipString finds the end of a string whose start was evicted: its ST, BEL
// for an OSC, or the ESC of the sequence that aborts it.
func skipString(tail []byte, bel bool) int {
	for i, c := range tail {
		switch {
		case c == 0x1b && i+1 < len(tail) && tail[i+1] == '\\':
			return i + 2
		case c == 0x1b:
			return i
		case bel && c == 0x07:
			return i + 1
		}
	}
	return len(tail)
}

func sortedKeys[K int | byte, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
