package main

import (
	"image"
	"slices"
	"testing"

	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/gioterm"
)

// terminalRig is a router with a focused terminal-like handler, and a frame
// that polls the chords the way window.layout does: hotkeys first, then the
// terminal's own filters.
type terminalRig struct {
	router   input.Router
	terminal bool // the focus tag
	editor   hotkeyEditor
	goos     string
	outside  bool // the keyboard is somewhere other than a terminal
}

func (r *terminalRig) frame(t *testing.T, available func(hotkeyID) bool) (fired []hotkeyID, typed []key.Event) {
	t.Helper()
	ops := new(op.Ops)
	gtx := layout.Context{
		Ops: ops, Source: r.router.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(100, 100)),
	}
	fired = pollChords(gtx, &r.editor, r.goos, !r.outside, available)
	for {
		ev, ok := gtx.Event(gioterm.KeyFilters(&r.terminal)...)
		if !ok {
			break
		}
		if e, isKey := ev.(key.Event); isKey && e.State == key.Press {
			typed = append(typed, e)
		}
	}
	gtx.Execute(key.FocusCmd{Tag: &r.terminal})
	event.Op(ops, &r.terminal)
	r.router.Frame(ops)
	return fired, typed
}

func newTerminalRig(t *testing.T) *terminalRig {
	r := &terminalRig{goos: "linux"}
	r.frame(t, func(hotkeyID) bool { return true })
	r.frame(t, func(hotkeyID) bool { return true })
	return r
}

func chord(name key.Name, mods key.Modifiers) key.Event {
	return key.Event{Name: name, Modifiers: mods, State: key.Press}
}

func always(hotkeyID) bool { return true }

// In a terminal, a Ctrl+letter hotkey answers to Ctrl+Shift+letter, and
// Ctrl+letter is the terminal's.
func TestABoundChordIsTheWindowsAndNeverReachesTheFocusedTerminal(t *testing.T) {
	r := newTerminalRig(t)
	r.router.Queue(chord("N", key.ModCtrl|key.ModShift), chord("N", key.ModCtrl))
	fired, typed := r.frame(t, always)
	if !slices.Equal(fired, []hotkeyID{hotkeyNewSession}) {
		t.Errorf("fired = %v, want newSession", fired)
	}
	if len(typed) != 1 || typed[0].Modifiers != key.ModCtrl {
		t.Errorf("the terminal got %v, want only Ctrl+N", typed)
	}
}

func TestOutsideATerminalBothCtrlLetterAndCtrlShiftLetterFire(t *testing.T) {
	r := newTerminalRig(t)
	r.outside = true
	r.frame(t, always) // the router delivers to the filters of the frame before
	r.router.Queue(chord("N", key.ModCtrl), chord("N", key.ModCtrl|key.ModShift))
	fired, _ := r.frame(t, always)
	if !slices.Equal(fired, []hotkeyID{hotkeyNewSession, hotkeyNewSession}) {
		t.Errorf("fired = %v", fired)
	}
}

// Command reaches no terminal, so on macOS Command+letter is the chord
// everywhere and Ctrl+letter stays the terminal's control code.
func TestOnMacOSCommandLetterFiresAndCtrlLetterIsTheTerminals(t *testing.T) {
	r := newTerminalRig(t)
	r.goos = "darwin"
	r.frame(t, always)
	r.router.Queue(chord("N", key.ModCommand), chord("N", key.ModCtrl), chord(key.NameTab, key.ModCtrl))
	fired, typed := r.frame(t, always)
	if !slices.Equal(fired, []hotkeyID{hotkeyNewSession, hotkeyNextSession}) {
		t.Errorf("fired = %v", fired)
	}
	if len(typed) != 1 || typed[0].Name != "N" {
		t.Errorf("the terminal got %v, want Ctrl+N", typed)
	}
}

// A chord of an action that declines, or this window has no screen for, is
// the terminal's as if lich had never bound it.
func TestAnActionThatDeclinesLeavesItsChordToTheTerminal(t *testing.T) {
	r := newTerminalRig(t)
	r.router.Queue(chord("N", key.ModCtrl|key.ModShift))
	fired, typed := r.frame(t, func(id hotkeyID) bool { return id != hotkeyNewSession })
	if len(fired) != 0 || len(typed) != 1 {
		t.Errorf("fired %v, terminal got %v: want the chord passed through", fired, typed)
	}
}

func TestAnUnassignedActionLeavesItsFormerChordToTheTerminal(t *testing.T) {
	r := newTerminalRig(t)
	r.editor.current()[hotkeyNewSession] = comboUnassigned
	r.router.Queue(chord("N", key.ModCtrl|key.ModShift))
	fired, typed := r.frame(t, always)
	if len(fired) != 0 || len(typed) != 1 {
		t.Errorf("fired %v, terminal got %v", fired, typed)
	}
}

// Rebinding in Settings takes effect on the next frame.
func TestARebindTakesEffectAtOnceAndFreesTheOldChord(t *testing.T) {
	r := newTerminalRig(t)
	r.editor.current()[hotkeyNewSession] = combo{Mod: true, Shift: true, Key: "j"}
	r.editor.current()[hotkeyToggleDock] = comboUnassigned
	r.router.Queue(chord("J", key.ModCtrl|key.ModShift), chord("N", key.ModCtrl|key.ModShift))
	fired, typed := r.frame(t, always)
	if !slices.Equal(fired, []hotkeyID{hotkeyNewSession}) {
		t.Errorf("fired = %v", fired)
	}
	if len(typed) != 1 || typed[0].Name != "N" {
		t.Errorf("terminal got %v, want the freed Ctrl+Shift+N", typed)
	}
}

// While a capture button records, no hotkey fires and the keys are left for
// the button.
func TestNoHotkeyFiresWhileACaptureButtonRecords(t *testing.T) {
	r := newTerminalRig(t)
	r.editor.recording = hotkeyNewSession
	r.router.Queue(chord("N", key.ModCtrl|key.ModShift))
	fired, typed := r.frame(t, always)
	if len(fired) != 0 {
		t.Errorf("fired = %v", fired)
	}
	if len(typed) != 1 {
		t.Errorf("terminal got %v: the chord stays in the queue for whoever records", typed)
	}
}

func TestAChordWithSuperIsThePrimaryModifierToo(t *testing.T) {
	r := newTerminalRig(t)
	r.router.Queue(chord("N", key.ModSuper|key.ModShift), chord("B", key.ModSuper|key.ModShift))
	fired, _ := r.frame(t, always)
	if !slices.Equal(fired, []hotkeyID{hotkeyNewSession, hotkeyNewWorktree}) {
		t.Errorf("fired = %v", fired)
	}
}

func TestNamedKeysFireAsTheirWebNames(t *testing.T) {
	r := newTerminalRig(t)
	r.router.Queue(
		chord(key.NameTab, key.ModCtrl),
		chord(key.NameReturn, key.ModCtrl|key.ModShift),
		chord(",", key.ModCtrl),
		chord(key.NameF1, 0),
	)
	fired, _ := r.frame(t, always)
	want := []hotkeyID{hotkeyNextSession, hotkeyZoomPane, hotkeySettings, hotkeyShortcuts}
	if !slices.Equal(fired, want) {
		t.Errorf("fired = %v, want %v", fired, want)
	}
}

func TestDigitsAndTextSizeChordsFireInATerminal(t *testing.T) {
	r := newTerminalRig(t)
	r.router.Queue(chord("1", key.ModCtrl), chord("9", key.ModCtrl), chord("=", key.ModCtrl), chord("-", key.ModCtrl), chord("0", key.ModCtrl))
	fired, typed := r.frame(t, always)
	want := []hotkeyID{hotkeyTab(1), hotkeyLastTab, hotkeyTextLarger, hotkeyTextSmaller, hotkeyTextReset}
	if !slices.Equal(fired, want) || len(typed) != 0 {
		t.Errorf("fired %v, terminal got %v; want %v", fired, typed, want)
	}
}

func TestLeftToFocusKeepsF6ForATerminalAndDigitsForAPicker(t *testing.T) {
	for _, c := range []struct {
		id                  hotkeyID
		inTerminal, picking bool
		want                bool
	}{
		{hotkeyFocusTerminal, true, false, true},
		{hotkeyFocusTerminal, false, false, false},
		{hotkeyTab(3), false, true, true},
		{hotkeyLastTab, false, true, true},
		{hotkeyTab(3), true, false, false},
		{hotkeyTextLarger, true, true, false},
	} {
		if got := leftToFocus(c.id, c.inTerminal, c.picking); got != c.want {
			t.Errorf("%s (terminal %v, picker %v) = %v", c.id, c.inTerminal, c.picking, got)
		}
	}
}

func TestStripTabAtCountsFromOneAndZeroIsTheLast(t *testing.T) {
	tabs := []projectTab{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	for n, want := range map[int]string{1: "a", 3: "c", 0: "c"} {
		if got, ok := stripTabAt(tabs, n); !ok || got != want {
			t.Errorf("%d = %q, %v", n, got, ok)
		}
	}
	if _, ok := stripTabAt(tabs, 4); ok {
		t.Error("a tab the strip does not have")
	}
	if _, ok := stripTabAt(nil, 0); ok {
		t.Error("the last tab of an empty strip")
	}
}

func TestSteppedTextSizeClampsAndResets(t *testing.T) {
	for _, c := range []struct{ size, step, want int }{
		{14, 1, 15}, {14, -1, 13}, {terminalFontSizeMax, 1, terminalFontSizeMax}, {terminalFontSizeMin, -1, terminalFontSizeMin}, {20, 0, defaultTerminalSize},
	} {
		if got := steppedTextSize(c.size, c.step); got != c.want {
			t.Errorf("%d%+d = %d, want %d", c.size, c.step, got, c.want)
		}
	}
}

// The chat's Ctrl+B takes New worktree's default chord, outside macOS where
// that chord is Cmd+B, and leaves a rebound one alone.
func TestTheChatsCtrlBTakesTheChordItShares(t *testing.T) {
	for _, c := range []struct {
		combo combo
		goos  string
		want  bool
	}{
		{modChord("b"), "linux", true},
		{modChord("b"), "windows", true},
		{modChord("b"), "darwin", false},
		{shiftChord("b"), "linux", false},
		{modChord("n"), "linux", false},
	} {
		if got := takesBackgroundKey(c.combo, c.goos, false); got != c.want {
			t.Errorf("%+v on %s: %v, want %v", c.combo, c.goos, got, c.want)
		}
	}
}
