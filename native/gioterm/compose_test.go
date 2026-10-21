package gioterm

import (
	"testing"

	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
)

var noComposition = key.CompositionEvent{Start: -1, End: -1}

// macOS's setMarkedText and insertText, as Gio's app package delivers them.
func markedText(at key.Range, text string, comp key.Range, caret int) []event.Event {
	return []event.Event{key.EditEvent{Range: at, Text: text}, key.CompositionEvent(comp), key.SelectionEvent{Start: caret, End: caret}}
}

func insertedText(at key.Range, text string, caret int) []event.Event {
	return []event.Event{key.EditEvent{Range: at, Text: text}, noComposition, key.SelectionEvent{Start: caret, End: caret}}
}

func typeEvents(c *Composition, tag event.Tag, evs []event.Event) (typed string, cmds []input.Command) {
	for _, e := range evs {
		typed += c.Update(e, tag, func(cmd input.Command) { cmds = append(cmds, cmd) })
	}
	return typed, cmds
}

func TestCompositionCommitsDeadKeyAccent(t *testing.T) {
	var c Composition
	tag := new(int)
	typed, cmds := typeEvents(&c, tag, markedText(key.Range{}, "˜", key.Range{End: 1}, 1))
	if typed != "" {
		t.Fatalf("the dead key typed %q before it composed", typed)
	}
	want := key.Snippet{Range: key.Range{End: 1}, Text: "˜"}
	if !holdsSnippet(cmds, want) {
		t.Fatalf("the composition was not mirrored to Gio as %v: %v", want, cmds)
	}
	typed, _ = typeEvents(&c, tag, insertedText(key.Range{End: 1}, "ã", 1))
	if typed != "ã" {
		t.Fatalf("~ then a typed %q, want ã", typed)
	}
}

func TestCompositionTypesPlainTextAsIs(t *testing.T) {
	var c Composition
	tag := new(int)
	typed, cmds := typeEvents(&c, tag, insertedText(key.Range{}, "a", 1))
	// Gio still edits at the caret it saw after a, until the next frame.
	more, _ := typeEvents(&c, tag, insertedText(key.Range{Start: 1, End: 1}, "b", 2))
	if typed+more != "ab" {
		t.Fatalf("typed %q, want ab", typed+more)
	}
	if len(cmds) != 0 {
		t.Fatalf("plain typing touched Gio's editor state: %v", cmds)
	}
}

func TestCompositionDropsPreeditOnFocusChange(t *testing.T) {
	var c Composition
	tag := new(int)
	typeEvents(&c, tag, markedText(key.Range{}, "˜", key.Range{End: 1}, 1))
	typed, _ := typeEvents(&c, tag, append([]event.Event{key.FocusEvent{}}, insertedText(key.Range{}, "x", 1)...))
	if typed != "x" {
		t.Fatalf("typed %q after focus moved, want x", typed)
	}
}

func holdsSnippet(cmds []input.Command, want key.Snippet) bool {
	for _, cmd := range cmds {
		if s, ok := cmd.(key.SnippetCmd); ok && s.Snippet == want {
			return true
		}
	}
	return false
}
