package extrender

import (
	"testing"

	"gioui.org/io/key"
	"gioui.org/io/pointer"

	"github.com/lichdotdev/lich/extension"
)

func everyInput() extension.Node {
	button := extension.Button.New(extension.ButtonProps{Text: "Copy", OnPress: "copy"})
	return column(
		extension.Input.New(extension.InputProps{Value: "v", Placeholder: "Name", Leading: "search", LeadingSize: 14, Height: 28,
			Width: 200, TextSize: 12, Mono: true, Password: true, Borderless: true, Bare: true, Clearable: true, Disabled: true,
			Invalid: true, OnChange: "change", OnSubmit: "submit"}),
		extension.InputGroup.New(extension.InputGroupProps{Value: "g", Placeholder: "URL", Start: []extension.Node{label("https://")},
			End: []extension.Node{button}, Width: 240, Disabled: true, Invalid: true, OnChange: "change", OnSubmit: "submit"}),
		extension.InputGroup.New(extension.InputGroupProps{Placeholder: "Message", Multiline: true, End: []extension.Node{button}}),
		extension.Textarea.New(extension.TextareaProps{Value: "t", Placeholder: "Notes", Resizable: true, MaxHeight: 80, Width: 240,
			Disabled: true, Invalid: true, OnChange: "change"}),
		extension.InlineEdit.New(extension.InlineEditProps{Value: "name", Size: 13, Mono: true, PadRight: 4, OnCommit: "commit", OnCancel: "cancel"}),
	)
}

func TestEveryInputDrawsWithEveryProp(t *testing.T) {
	drawsWithoutPanic(t, everyInput())
}

func TestTypingInAnInputSendsItsText(t *testing.T) {
	input := func(value string) extension.Node {
		return column(extension.Input.New(extension.InputProps{Value: value, OnChange: "change", OnSubmit: "submit"}))
	}
	g := newRig(t, input(""))
	g.click(pointer.ButtonPrimary, 10, 10)
	g.typed("skipo")
	if e := g.once("change"); e.Text != "skipo" {
		t.Errorf("change event = %+v, want skipo", e)
	}
	s := stateOf[inputState](g.r, "/0")
	g.frame()
	if got := s.editor.Text(); got != "skipo" {
		t.Errorf("a stale render left %q in the field, want the typed skipo", got)
	}
	g.press(key.NameReturn)
	if e := g.once("submit"); e.Text != "skipo" {
		t.Errorf("submit event = %+v, want skipo", e)
	}
	g.tree = input("reset")
	g.frame()
	g.frame()
	if got := s.editor.Text(); got != "reset" {
		t.Errorf("a new value from the extension left %q, want reset", got)
	}
	if len(g.fired("change")) != 1 {
		t.Errorf("the extension's own value was sent back to it: %+v", g.fired("change"))
	}
}

func TestAPasswordInputMasksItsText(t *testing.T) {
	g := newRig(t, column(extension.Input.New(extension.InputProps{Value: "secret", Password: true})))
	if s := stateOf[inputState](g.r, "/0"); s.editor.Mask != passwordMask {
		t.Errorf("mask = %q, want %q", s.editor.Mask, passwordMask)
	}
}

func TestTypingInAGroupAndATextareaSendsTheirText(t *testing.T) {
	for _, n := range []extension.Node{
		extension.InputGroup.New(extension.InputGroupProps{Multiline: true, OnChange: "change"}),
		extension.Textarea.New(extension.TextareaProps{OnChange: "change"}),
	} {
		g := newRig(t, column(n))
		g.click(pointer.ButtonPrimary, 20, 10)
		g.typed("hi")
		if e := g.once("change"); e.Text != "hi" {
			t.Errorf("%s change event = %+v, want hi", n.Type, e)
		}
	}
}

func TestAnInlineEditCommitsOrCancels(t *testing.T) {
	edit := column(extension.InlineEdit.New(extension.InlineEditProps{Value: "old", OnCommit: "commit", OnCancel: "cancel"}))
	g := newRig(t, edit)
	g.frame()
	// Start selected all of "old"; a platform's edit names the range it
	// replaces, where a bare EditEvent would insert at 0.
	g.router.Queue(key.EditEvent{Range: key.Range{Start: 0, End: len("old")}, Text: "new"})
	g.frame()
	g.press(key.NameReturn)
	if e := g.once("commit"); e.Text != "new" {
		t.Errorf("commit event = %+v, want new", e)
	}
	g.frame()

	g = newRig(t, edit)
	g.frame()
	g.press(key.NameEscape)
	g.once("cancel")
	g.frame()
}

func keyedFields(extra ...extension.Node) extension.Node {
	return column(append([]extension.Node{
		extension.Input.New(extension.InputProps{OnChange: "a"}).WithKey("a"),
		extension.InputGroup.New(extension.InputGroupProps{OnChange: "b"}).WithKey("b"),
		extension.Textarea.New(extension.TextareaProps{OnChange: "c"}).WithKey("c"),
	}, extra...)...)
}

func TestFocusGivesTheKeyboardToTheFieldUnderItsKey(t *testing.T) {
	g := newRig(t, keyedFields())
	for _, k := range []string{"b", "c", "a"} {
		g.r.Focus(k)
		g.frame()
		g.typed(k)
		if e := g.once(extension.Handler(k)); e.Text != k {
			t.Errorf("after Focus(%q) typing sent %+v", k, e)
		}
	}
	if len(g.got) != 3 {
		t.Errorf("typing reached other fields: %+v", g.got)
	}
}

func TestFocusOnAKeyNoFieldHasIsDroppedAfterThatFrame(t *testing.T) {
	g := newRig(t, keyedFields(label("ghost").WithKey("ghost")))
	g.r.Focus("a")
	g.frame()
	g.r.Focus("ghost")
	g.frame()
	g.tree = keyedFields(extension.Input.New(extension.InputProps{OnChange: "ghost"}).WithKey("ghost"))
	g.frame()
	g.typed("x")
	if len(g.fired("ghost")) != 0 || len(g.fired("a")) != 1 {
		t.Errorf("a focus on a label moved the keyboard or waited for a field: %+v", g.got)
	}
}
