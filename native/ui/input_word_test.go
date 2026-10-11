package ui

import (
	"image"
	"testing"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/widget"
)

// focusedInputRig lays out an Input over ed, focused, holding text with the
// caret at caret.
func focusedInputRig(t *testing.T, in InputStyle, text string, caret int) *inputRig {
	t.Helper()
	rig := newInputRig(t, 160, 36, in.Layout)
	rig.frame()
	rig.router.Source().Execute(key.FocusCmd{Tag: in.Editor})
	in.Editor.SetText(text)
	in.Editor.SetCaret(caret, caret)
	rig.frame()
	return rig
}

func (r *inputRig) press(name key.Name, mods key.Modifiers) {
	r.router.Queue(key.Event{Name: name, Modifiers: mods, State: key.Press}, key.Event{Name: name, Modifiers: mods, State: key.Release})
	r.frame()
}

// Ctrl+Backspace and Ctrl+Delete (Alt on macOS) delete a word with the spaces
// before it, as a browser's field does; lichdotdev/gio's editor carries both.
func TestShortcutAltDeletesAWord(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		caret int
		key   key.Name
		want  string
	}{
		{"backward at the end", "foo bar baz", 11, key.NameDeleteBackward, "foo bar "},
		{"backward over trailing spaces", "foo bar  ", 9, key.NameDeleteBackward, "foo "},
		{"backward mid-word", "foo barbaz", 7, key.NameDeleteBackward, "foo baz"},
		{"backward at the start", "foo", 0, key.NameDeleteBackward, "foo"},
		{"forward at the start", "foo bar", 0, key.NameDeleteForward, " bar"},
		{"forward over leading spaces", "foo  bar baz", 3, key.NameDeleteForward, "foo baz"},
		{"forward at the end", "foo", 3, key.NameDeleteForward, "foo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ed := new(widget.Editor)
			rig := focusedInputRig(t, testTheme().Input(ed, "Name"), tc.text, tc.caret)
			rig.press(tc.key, key.ModShortcutAlt)
			if got := ed.Text(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestShortcutAltDeleteTakesTheSelectionFirst(t *testing.T) {
	ed := new(widget.Editor)
	rig := focusedInputRig(t, testTheme().Input(ed, "Name"), "foo bar baz", 0)
	ed.SetCaret(5, 9)
	rig.press(key.NameDeleteBackward, key.ModShortcutAlt)
	if got, want := ed.Text(), "foo baz"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestInputClearButton(t *testing.T) {
	th, ed, clr := testTheme(), new(widget.Editor), new(widget.Clickable)
	in := th.Input(ed, "Filter")
	in.Clear = clr
	rig := focusedInputRig(t, in, "skipo", 5)
	// The icon-xs button's center: 4px in from the right edge, 24px wide.
	at := inputPointF32(image.Pt(rig.margin+160-4-12, rig.margin+18))
	rig.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at},
	)
	rig.frame()
	rig.frame()
	if got := ed.Text(); got != "" {
		t.Fatalf("text after the press: %q, want it empty", got)
	}
	if !rig.router.Source().Focused(ed) {
		t.Error("the field lost its focus to the button")
	}
	rig.press("Z", key.ModShortcut)
	if got := ed.Text(); got != "skipo" {
		t.Errorf("undo after clearing: %q, want %q back", got, "skipo")
	}
}

// Without text there is nothing to clear: the press lands on the field.
func TestInputClearButtonHidesWhenEmpty(t *testing.T) {
	th, ed, clr := testTheme(), new(widget.Editor), new(widget.Clickable)
	in := th.Input(ed, "Filter")
	in.Clear = clr
	rig := newInputRig(t, 160, 36, in.Layout)
	rig.frame()
	at := inputPointF32(image.Pt(rig.margin+160-4-12, rig.margin+18))
	rig.router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at})
	rig.frame()
	if !rig.router.Source().Focused(ed) {
		t.Error("a press where the button would be did not focus the empty field")
	}
}
