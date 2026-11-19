package ui

import (
	"time"

	"gioui.org/io/key"
	"gioui.org/widget"
)

// editGroupGap is the pause that ends a run of typing: edits of one kind, side
// by side and closer together than this, are one step to undo.
const editGroupGap = time.Second

// editHistoryDepth is how many steps it keeps.
const editHistoryDepth = 200

// editKind is what an edit did, for grouping: a run of single characters typed
// at the caret is one step, and so is a run of single characters deleted, but a
// paste, a cut or a replaced selection is a step of its own.
type editKind uint8

const (
	editNone editKind = iota
	editType
	editDelete
	editOther
)

// editState is a text with the caret and the other end of the selection.
type editState struct {
	text          string
	caret, anchor int
}

// EditHistory is the undo and redo of one text field: Ctrl+Z, or Cmd+Z, brings
// back the text as it was before the last step, with the caret and selection it
// had, Ctrl+Shift+Z and Ctrl+Y redo it, and Reset forgets it all. Gio's Editor
// has an undo of its own, one keystroke a step and with no way to clear it, so
// a field that has an EditHistory takes the keys away from it.
type EditHistory struct {
	undo, redo []editState
	cur        editState
	synced     bool
	kind       editKind
	at         time.Time
}

// Reset forgets every step and takes the field's text as it stands as the
// start: for a text the program set, which is not an edit of the person's.
func (h *EditHistory) Reset() {
	h.undo, h.redo, h.synced, h.kind = nil, nil, false, editNone
}

// Handle carries out the undo and redo keys of the frame on ed. It must run
// before anything reads ed's events (ed.Update, the Textarea's layout): those
// would give the keys to Gio's own undo.
func (h *EditHistory) Handle(gtx C, ed *widget.Editor) {
	for {
		ev, ok := gtx.Event(
			key.Filter{Focus: ed, Name: "Z", Required: key.ModShortcut, Optional: key.ModShift},
			key.Filter{Focus: ed, Name: "Y", Required: key.ModShortcut},
		)
		if !ok {
			return
		}
		k, isKey := ev.(key.Event)
		if !isKey || k.State != key.Press || ed.ReadOnly {
			continue
		}
		if k.Name == "Y" || k.Modifiers.Contain(key.ModShift) {
			h.step(ed, &h.redo, &h.undo)
		} else {
			h.step(ed, &h.undo, &h.redo)
		}
	}
}

// step moves ed to the last state of from, keeping where it was in to.
func (h *EditHistory) step(ed *widget.Editor, from, to *[]editState) {
	if !h.synced || len(*from) == 0 {
		return
	}
	state := (*from)[len(*from)-1]
	*from = (*from)[:len(*from)-1]
	*to = append(*to, h.cur)
	h.cur, h.kind = state, editNone
	ed.SetText(state.text)
	ed.SetCaret(state.caret, state.anchor)
}

// observe reads what the frame did to ed: an edit is a step, or goes into the
// one before it when it goes on from it.
func (h *EditHistory) observe(now time.Time, ed *widget.Editor) {
	caret, anchor := ed.Selection()
	state := editState{text: ed.Text(), caret: caret, anchor: anchor}
	if !h.synced {
		h.cur, h.synced = state, true
		return
	}
	if state.text == h.cur.text {
		if state != h.cur {
			// The caret moved: typing after it starts a new step.
			h.cur, h.kind = state, editNone
		}
		return
	}
	kind := classifyEdit(h.cur, state.text)
	if kind == editOther || kind != h.kind || now.Sub(h.at) >= editGroupGap {
		if len(h.undo) == editHistoryDepth {
			h.undo = h.undo[1:]
		}
		h.undo = append(h.undo, h.cur)
	}
	h.redo = nil
	h.cur, h.kind, h.at = state, kind, now
}

// classifyEdit is what turning old's text into text did. A single character
// typed or deleted at the caret is a kind of its own; anything else is other.
func classifyEdit(old editState, text string) editKind {
	was, now := []rune(old.text), []rune(text)
	start := 0
	for start < len(was) && start < len(now) && was[start] == now[start] {
		start++
	}
	end := 0
	for end < len(was)-start && end < len(now)-start && was[len(was)-1-end] == now[len(now)-1-end] {
		end++
	}
	deleted, inserted := len(was)-start-end, len(now)-start-end
	switch {
	case deleted == 0 && inserted == 1 && now[start] != '\n' && start == old.caret:
		return editType
	case inserted == 0 && deleted == 1 && (start+1 == old.caret || start == old.caret):
		return editDelete
	}
	return editOther
}
