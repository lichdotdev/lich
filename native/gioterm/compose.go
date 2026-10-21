package gioterm

import (
	"slices"

	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
)

// Composition is the text an input method is still composing in a terminal
// (a dead key's accent, an IME's preedit), which the program in the terminal
// must not see until it is committed. Gio checks after every frame that the
// focused handler's editor state still holds what the input method wrote, and
// cancels the composition when it does not, so while composing the terminal
// mirrors that state back: without it, ~ then a types ~a instead of ã.
type Composition struct {
	text      []rune
	selection key.Range
	composing bool
	mirrored  bool
}

// Update applies an editor event the terminal under tag received and returns
// the text it commits, if any. Every platform's input method ends an edit
// with a selection or a composition event, so that is where text commits.
func (c *Composition) Update(e event.Event, tag event.Tag, execute func(input.Command)) string {
	switch e := e.(type) {
	case key.EditEvent:
		c.replace(e.Range, e.Text)
	case key.CompositionEvent:
		c.composing = e.Start != -1
		return c.settle(tag, execute)
	case key.SelectionEvent:
		c.selection = key.Range(e)
		return c.settle(tag, execute)
	case key.FocusEvent:
		// A focus change empties Gio's editor state and drops what was composing.
		*c = Composition{}
	}
	return ""
}

// replace clamps r to the text held, because between a commit and the next
// frame Gio still edits at the caret of the text it saw before the commit.
func (c *Composition) replace(r key.Range, text string) {
	start := min(max(min(r.Start, r.End), 0), len(c.text))
	end := min(max(max(r.Start, r.End), start), len(c.text))
	c.text = slices.Concat(c.text[:start], []rune(text), c.text[end:])
}

func (c *Composition) settle(tag event.Tag, execute func(input.Command)) string {
	if c.composing {
		execute(key.SnippetCmd{Tag: tag, Snippet: key.Snippet{Range: key.Range{End: len(c.text)}, Text: string(c.text)}})
		execute(key.SelectionCmd{Tag: tag, Range: c.selection})
		c.mirrored = true
		return ""
	}
	committed := string(c.text)
	c.text, c.selection = nil, key.Range{}
	if c.mirrored {
		execute(key.SnippetCmd{Tag: tag})
		execute(key.SelectionCmd{Tag: tag})
		c.mirrored = false
	}
	return committed
}
