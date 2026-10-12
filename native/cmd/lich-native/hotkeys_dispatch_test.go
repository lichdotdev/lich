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

	"github.com/omartelo/lich/native/gioterm"
)

// terminalRig is a router with a focused terminal-like handler, and a frame
// that polls the chords the way window.layout does: hotkeys first, then the
// terminal's own filters.
type terminalRig struct {
	router   input.Router
	terminal bool // the focus tag
	editor   hotkeyEditor
}

func (r *terminalRig) frame(t *testing.T, available func(hotkeyID) bool) (fired []hotkeyID, typed []key.Event) {
	t.Helper()
	ops := new(op.Ops)
	gtx := layout.Context{
		Ops: ops, Source: r.router.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(100, 100)),
	}
	fired = pollChords(gtx, &r.editor, available)
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
	r := new(terminalRig)
	r.frame(t, func(hotkeyID) bool { return true })
	r.frame(t, func(hotkeyID) bool { return true })
	return r
}

func chord(name key.Name, mods key.Modifiers) key.Event {
	return key.Event{Name: name, Modifiers: mods, State: key.Press}
}

func always(hotkeyID) bool { return true }

func TestABoundChordIsTheWindowsAndNeverReachesTheFocusedTerminal(t *testing.T) {
	r := newTerminalRig(t)
	r.router.Queue(chord("T", key.ModCtrl|key.ModShift), chord("T", key.ModCtrl))
	fired, typed := r.frame(t, always)
	if !slices.Equal(fired, []hotkeyID{hotkeyNewSession}) {
		t.Errorf("fired = %v, want newSession", fired)
	}
	if len(typed) != 1 || typed[0].Modifiers != key.ModCtrl {
		t.Errorf("the terminal got %v, want only Ctrl+T, which no action is bound to", typed)
	}
}

// A chord of an action that declines, or this window has no screen for, is
// the terminal's as if lich had never bound it.
func TestAnActionThatDeclinesLeavesItsChordToTheTerminal(t *testing.T) {
	r := newTerminalRig(t)
	r.router.Queue(chord("T", key.ModCtrl|key.ModShift))
	fired, typed := r.frame(t, func(id hotkeyID) bool { return id != hotkeyNewSession })
	if len(fired) != 0 || len(typed) != 1 {
		t.Errorf("fired %v, terminal got %v: want the chord passed through", fired, typed)
	}
}

func TestAnUnassignedActionLeavesItsFormerChordToTheTerminal(t *testing.T) {
	r := newTerminalRig(t)
	r.editor.current()[hotkeyNewSession] = comboUnassigned
	r.router.Queue(chord("T", key.ModCtrl|key.ModShift))
	fired, typed := r.frame(t, always)
	if len(fired) != 0 || len(typed) != 1 {
		t.Errorf("fired %v, terminal got %v", fired, typed)
	}
}

// Rebinding in Settings takes effect on the next frame.
func TestARebindTakesEffectAtOnceAndFreesTheOldChord(t *testing.T) {
	r := newTerminalRig(t)
	r.editor.current()[hotkeyNewSession] = combo{Mod: true, Shift: true, Key: "j"}
	r.router.Queue(chord("J", key.ModCtrl|key.ModShift), chord("T", key.ModCtrl|key.ModShift))
	fired, typed := r.frame(t, always)
	if !slices.Equal(fired, []hotkeyID{hotkeyNewSession}) {
		t.Errorf("fired = %v", fired)
	}
	if len(typed) != 1 || typed[0].Name != "T" {
		t.Errorf("terminal got %v, want the freed Ctrl+Shift+T", typed)
	}
}

// While a capture button records, no hotkey fires and the keys are left for
// the button.
func TestNoHotkeyFiresWhileACaptureButtonRecords(t *testing.T) {
	r := newTerminalRig(t)
	r.editor.recording = hotkeyNewSession
	r.router.Queue(chord("T", key.ModCtrl|key.ModShift))
	fired, typed := r.frame(t, always)
	if len(fired) != 0 {
		t.Errorf("fired = %v", fired)
	}
	if len(typed) != 1 {
		t.Errorf("terminal got %v: the chord stays in the queue for whoever records", typed)
	}
}

func TestAChordWithCommandOrSuperIsThePrimaryModifierToo(t *testing.T) {
	r := newTerminalRig(t)
	r.router.Queue(chord("T", key.ModCommand|key.ModShift), chord("B", key.ModSuper|key.ModShift))
	fired, _ := r.frame(t, always)
	if !slices.Equal(fired, []hotkeyID{hotkeyNewSession, hotkeyNewWorktree}) {
		t.Errorf("fired = %v", fired)
	}
}

func TestNamedKeysFireAsTheirWebNames(t *testing.T) {
	r := newTerminalRig(t)
	r.router.Queue(
		chord(key.NameDownArrow, key.ModCtrl|key.ModShift),
		chord(key.NameReturn, key.ModCtrl|key.ModShift),
		chord(",", key.ModCtrl),
	)
	fired, _ := r.frame(t, always)
	want := []hotkeyID{hotkeyNextSession, hotkeyFocusTerminal, hotkeySettings}
	if !slices.Equal(fired, want) {
		t.Errorf("fired = %v, want %v", fired, want)
	}
}
