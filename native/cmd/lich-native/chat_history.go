package main

import (
	"slices"
	"strings"
	"unicode/utf8"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
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

// sentHistory is the messages sent in items, newest first, each as the chat
// shows it: a task lich handed over is its prompt, and the other notes lich
// handed the agent are left out, since nobody sent them to send again.
func sentHistory(items []backend.ChatItem) []string {
	var sent []string
	for _, it := range slices.Backward(items) {
		if it.Kind != api.ItemUser {
			continue
		}
		text := it.Text
		if envs, ok := parseEnvelopes(text); ok {
			text = ""
			if len(envs) == 1 && envs[0].kind == "task" {
				text = envs[0].text["prompt"]
			}
		}
		if strings.TrimSpace(text) != "" {
			sent = append(sent, text)
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

// chatSearch is Ctrl+R's reverse search through the messages sent, inline as
// Claude Code's is (its HistorySearch keys, 2.1.330): the query is typed in a
// field on the box's foot, the box shows the newest message that holds it with
// the match selected, Ctrl+R goes on to the next older one, Esc or Tab keep it
// in the box to edit, Enter sends it, and Ctrl+C or Backspace on an empty query
// put back the draft the box held.
type chatSearch struct {
	open  bool
	draft string
	query widget.Editor
	// last is the query the box was searched for, at the message it shows in
	// sent, -1 for none, and seen the messages shown for it, so Ctrl+R skips a
	// repeat; failed is a query nothing older holds.
	last   string
	at     int
	seen   []string
	caret  int
	failed bool
}

func searchFilter(tag event.Tag, name key.Name) key.Filter {
	return key.Filter{Focus: tag, Name: name, Required: key.ModCtrl}
}

// acts runs the search's keys before the box reads its own, and reports
// whether Enter asked for the box to be sent.
func (s *chatSearch) acts(gtx C, ed *widget.Editor, sent []string) (submit bool) {
	if !s.open {
		for len(sent) > 0 {
			ev, ok := gtx.Event(searchFilter(ed, "R"))
			if !ok {
				break
			}
			if e, ok := ev.(key.Event); ok && e.State == key.Press {
				s.open, s.draft, s.last, s.at, s.seen, s.failed = true, ed.Text(), "", -1, nil, false
				s.query.SetText("")
				ed.PaintSelectionWhenUnfocused = true
			}
		}
		if !s.open {
			return false
		}
	}
	if !gtx.Focused(&s.query) {
		gtx.Execute(key.FocusCmd{Tag: &s.query})
	}
	filters := []event.Filter{searchFilter(&s.query, "R"), searchFilter(&s.query, "C")}
	for _, name := range []key.Name{key.NameEscape, key.NameTab, key.NameReturn, key.NameEnter} {
		filters = append(filters, key.Filter{Focus: &s.query, Name: name})
	}
	if s.query.Len() == 0 {
		filters = append(filters, key.Filter{Focus: &s.query, Name: key.NameDeleteBackward})
	}
	for s.open {
		ev, ok := gtx.Event(filters...)
		if !ok {
			break
		}
		e, isKey := ev.(key.Event)
		if !isKey || e.State != key.Press {
			continue
		}
		switch e.Name {
		case "R":
			s.find(ed, sent, s.at+1)
		case key.NameEscape, key.NameTab:
			s.close(ed)
		case key.NameReturn, key.NameEnter:
			submit = s.at >= 0 || s.query.Len() == 0
			s.close(ed)
		default:
			ed.SetText(s.draft)
			s.caret = len([]rune(s.draft))
			s.close(ed)
		}
	}
	if !s.open {
		return submit
	}
	for {
		if _, ok := s.query.Update(gtx); !ok {
			break
		}
	}
	if q := s.query.Text(); q != s.last {
		s.last, s.at, s.seen, s.failed = q, -1, nil, false
		if q == "" {
			ed.SetText(s.draft)
		} else {
			s.find(ed, sent, 0)
		}
	}
	return false
}

// find shows the first message in sent from index from that holds the query
// and was not shown for it yet, or marks the search failed, the box as it was.
func (s *chatSearch) find(ed *widget.Editor, sent []string, from int) {
	q := strings.ToLower(s.last)
	if q == "" {
		return
	}
	for i := max(from, 0); i < len(sent); i++ {
		lower := strings.ToLower(sent[i])
		at := strings.Index(lower, q)
		if at < 0 || slices.Contains(s.seen, sent[i]) {
			continue
		}
		s.at, s.failed, s.seen = i, false, append(s.seen, sent[i])
		s.caret = utf8.RuneCountInString(lower[:at])
		ed.SetText(sent[i])
		ed.SetCaret(s.caret+utf8.RuneCountInString(q), s.caret)
		return
	}
	s.failed = true
}

// close ends the search with the box as it is, the caret where the match was.
func (s *chatSearch) close(ed *widget.Editor) {
	s.open = false
	ed.PaintSelectionWhenUnfocused = false
	ed.SetCaret(s.caret, s.caret)
}

// chatSearchLine is the search's field on the box's foot, after what it is.
func chatSearchLine(gtx C, th *ui.Theme, s *chatSearch) D {
	label := i18n.T("chat.search.label")
	if s.failed {
		label = i18n.T("chat.search.failed")
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(th.Text(ui.TextXS, label).Muted().Layout),
		layout.Flexed(1, func(gtx C) D {
			in := th.Input(&s.query, "")
			in.Bare, in.TextSize, in.Height = true, ui.TextXS, balloonItemHeight
			return in.Layout(gtx)
		}),
	)
}
