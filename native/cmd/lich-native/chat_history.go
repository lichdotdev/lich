package main

import (
	"image"
	"slices"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// The composer walks the session's messages already sent, as Claude Code's
// prompt does: ↑ on the box's first line takes the one before, ↓ on its last
// the one after, and past the newest the draft that was in the box comes back.
// Ctrl+R searches them in a list over the composer, the box as its query.

// sentHistory is the person's messages in items, newest first.
func sentHistory(items []backend.ChatItem) []string {
	var sent []string
	for _, it := range slices.Backward(items) {
		if it.Kind == api.ItemUser && strings.TrimSpace(it.Text) != "" {
			sent = append(sent, it.Text)
		}
	}
	return sent
}

// chatHistory is the composer's walk through the messages sent: at is how far
// back it stands, 1 the newest and 0 none, shown what it put in the box and
// draft what the box held before it began. The walk holds while the box holds
// shown: anything else put there, typed or sent, ends it.
type chatHistory struct {
	at           int
	shown, draft string
}

func (h *chatHistory) live(ed *widget.Editor, sent []string) bool {
	if h.at > len(sent) || h.at > 0 && ed.Text() != h.shown {
		h.at = 0
	}
	return h.at > 0
}

// filters is the arrows the walk takes from the next frame: ↑ with the caret
// on the box's first line and an older message to go to, or queue to edit,
// and ↓ on its last line while walking.
func (h *chatHistory) filters(ed *widget.Editor, sent []string, queue bool) []event.Filter {
	var fs []event.Filter
	live := h.live(ed, sent)
	if line, _ := ed.CaretPos(); queue || line == 0 && h.at < len(sent) {
		fs = append(fs, key.Filter{Focus: ed, Name: key.NameUpArrow})
	}
	if live && onLastLine(ed) {
		fs = append(fs, key.Filter{Focus: ed, Name: key.NameDownArrow})
	}
	return fs
}

// onLastLine is whether the caret is on the text's last line.
// shortcut: a line, not a row of its wrap, so ↓ on a wrapped last line walks
// before it reaches the bottom row; read the laid-out rows if that bothers.
func onLastLine(ed *widget.Editor) bool {
	_, caret := ed.Selection()
	runes := []rune(ed.Text())
	return !slices.Contains(runes[min(max(caret, 0), len(runes)):], '\n')
}

// back puts the message before the one shown in the box, the caret at its
// start so the next ↑ goes on.
func (h *chatHistory) back(ed *widget.Editor, sent []string) {
	if !h.live(ed, sent) {
		h.draft = ed.Text()
	}
	if h.at >= len(sent) {
		return
	}
	h.at++
	h.shown = sent[h.at-1]
	ed.SetText(h.shown)
}

// forward puts the message after the one shown in the box, or the draft past
// the newest, the caret at its end so the next ↓ goes on.
func (h *chatHistory) forward(ed *widget.Editor, sent []string) {
	if !h.live(ed, sent) {
		return
	}
	h.at--
	text := h.draft
	if h.at > 0 {
		text = sent[h.at-1]
	}
	h.shown = text
	ed.SetText(text)
	n := len([]rune(text))
	ed.SetCaret(n, n)
}

// chatSearch is the Ctrl+R list of the messages sent over the composer, in the
// slash list's place. While it is open the box is its query, and draft what
// the box held when it opened, back on Escape.
type chatSearch struct {
	open   bool
	draft  string
	query  string
	shown  []string
	cursor int
	rows   []ui.Clickable
	list   widget.List
	moved  bool
}

func searchFilter(ed *widget.Editor) key.Filter {
	return key.Filter{Focus: ed, Name: "R", Required: key.ModCtrl}
}

// acts runs the search's clicks and keys before the box reads its own:
// Ctrl+R opens it while there are messages to search, and once open it takes
// the keys a pick needs, so Enter puts the message in the box instead of
// sending the query.
func (s *chatSearch) acts(gtx C, ed *widget.Editor, sent []string) {
	if !s.open {
		if len(sent) == 0 {
			return
		}
		for {
			ev, ok := gtx.Event(searchFilter(ed))
			if !ok {
				return
			}
			if e, ok := ev.(key.Event); ok && e.State == key.Press {
				s.open, s.draft, s.query, s.shown = true, ed.Text(), "", nil
				ed.SetText("")
			}
		}
	}
	for i := range s.shown {
		if s.rows[i].Clicked(gtx) {
			s.pick(ed, i)
			return
		}
	}
	s.keys(gtx, ed)
}

// refresh filters the list on what the box holds once it has been laid out,
// as the slash list does, and asks for its keys in that frame.
func (s *chatSearch) refresh(gtx C, ed *widget.Editor, sent []string) {
	if !s.open {
		return
	}
	query := strings.ToLower(strings.TrimSpace(ed.Text()))
	s.shown = slices.DeleteFunc(slices.Clone(sent), func(m string) bool { return !strings.Contains(strings.ToLower(m), query) })
	if query != s.query {
		s.query, s.cursor, s.moved = query, 0, true
	}
	s.cursor = min(s.cursor, max(len(s.shown)-1, 0))
	if len(s.rows) < len(s.shown) {
		s.rows = make([]ui.Clickable, len(s.shown))
	}
	s.keys(gtx, ed)
}

func (s *chatSearch) keys(gtx C, ed *widget.Editor) {
	filters := []event.Filter{searchFilter(ed)}
	for _, name := range []key.Name{key.NameUpArrow, key.NameDownArrow, key.NameReturn, key.NameEnter, key.NameTab, key.NameEscape} {
		filters = append(filters, key.Filter{Focus: ed, Name: name})
	}
	for s.open {
		ev, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		e, isKey := ev.(key.Event)
		if !isKey || e.State != key.Press {
			continue
		}
		n := len(s.shown)
		switch {
		case e.Name == key.NameEscape:
			s.shut(ed, s.draft)
		case n == 0:
		case e.Name == key.NameUpArrow:
			s.cursor, s.moved = (s.cursor+n-1)%n, true
		case e.Name == key.NameDownArrow || e.Name == "R":
			s.cursor, s.moved = (s.cursor+1)%n, true
		default:
			s.pick(ed, s.cursor)
		}
	}
}

func (s *chatSearch) pick(ed *widget.Editor, i int) { s.shut(ed, s.shown[i]) }

// shut puts text in the box, the caret at its end, and shuts the list.
func (s *chatSearch) shut(ed *widget.Editor, text string) {
	s.open, s.shown = false, nil
	ed.SetText(text)
	n := len([]rune(text))
	ed.SetCaret(n, n)
}

// layout draws the open list over the composer's box as the slash list is
// drawn, a line for each message, or a muted line when none holds the query.
func (s *chatSearch) layout(gtx C, th *ui.Theme, room int) {
	if !s.open {
		return
	}
	s.list.Axis = layout.Vertical
	if s.moved {
		s.list.ScrollTo(max(s.cursor-2, 0))
		s.moved = false
	}
	g := gtx
	g.Constraints = layout.Constraints{Max: image.Pt(gtx.Constraints.Max.X, max(min(gtx.Dp(slashMaxH), room), 0))}
	macro := op.Record(gtx.Ops)
	d := th.PopoverPanel(g, func(gtx C, th *ui.Theme) D {
		if len(s.shown) == 0 {
			gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, gtx.Dp(pickerRowH))
			return layout.W.Layout(gtx, func(gtx C) D {
				return layout.Inset{Left: ui.Space(2)}.Layout(gtx, th.Text(ui.TextSM, i18n.T("chat.search.empty")).Muted().Layout)
			})
		}
		return th.ThinScrollArea(&s.list).Layout(gtx, len(s.shown), func(gtx C, i int) D {
			return pickerRow(gtx, th, &s.rows[i], i == s.cursor, func(gtx C, th *ui.Theme) D {
				return th.Text(ui.TextSM, expandTabs(strings.Join(strings.Fields(s.shown[i]), " "))).Truncated().Layout(gtx)
			})
		})
	})
	call := macro.Stop()
	m := op.Record(gtx.Ops)
	op.Offset(image.Pt(0, -d.Size.Y-gtx.Dp(ui.Space(2)))).Add(gtx.Ops)
	call.Add(gtx.Ops)
	op.Defer(gtx.Ops, m.Stop())
}
