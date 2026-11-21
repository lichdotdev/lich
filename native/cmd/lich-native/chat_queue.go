package main

import (
	"context"
	"log"
	"slices"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/paint"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// A message written while the agent works waits in the session's queue, over
// the composer, as Claude Code's prompt does: once the turn ends the queue goes
// in one message, Force sends one of them into the turn running, and ↑ on an
// empty box takes the queue back to edit.

var (
	icQueued    = icons.Lucide("clock")
	icQueueDrop = icons.Lucide("x")
)

// chatQueueJoin is what the queued messages are joined with into the one the
// turn's end sends.
const chatQueueJoin = "\n\n"

func (s *chatStore) enqueue(id, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.queue == nil {
		s.queue = map[string][]string{}
	}
	s.queue[id] = append(s.queue[id], text)
}

// queued is session id's queue, a copy.
func (s *chatStore) queued(id string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.queue[id])
}

// unqueue takes message i out of session id's queue.
func (s *chatStore) unqueue(id string, i int) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	q := s.queue[id]
	if i < 0 || i >= len(q) {
		return "", false
	}
	text := q[i]
	s.queue[id] = slices.Delete(q, i, i+1)
	return text, true
}

// takeQueue empties session id's queue and returns it.
func (s *chatStore) takeQueue(id string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	q := s.queue[id]
	delete(s.queue, id)
	return q
}

// requeue puts texts back at the head of session id's queue.
func (s *chatStore) requeue(id string, texts []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.queue == nil {
		s.queue = map[string][]string{}
	}
	s.queue[id] = append(slices.Clone(texts), s.queue[id]...)
}

// flushQueue sends session t.SessionID's queue in one message once its turn
// ended, unless the person stopped it (stopCancelled): then it stays, for
// them to edit or send. A message the backend refuses goes back
// to the queue.
func (m *model) flushQueue(ctx context.Context, t api.ChatTurn) {
	if t.Running || t.StopReason == stopCancelled {
		return
	}
	q := m.chat.takeQueue(t.SessionID)
	if len(q) == 0 {
		return
	}
	go func() {
		if err := m.client.ChatPrompt(ctx, t.SessionID, strings.Join(q, chatQueueJoin)); err != nil {
			log.Printf("chat queue of %s: %v", t.SessionID, err)
			m.chat.requeue(t.SessionID, q)
			m.invalidate()
		}
	}()
}

// queueChat puts the composer's text in session id's queue.
func (u *window) queueChat(p *chatPane, id string) {
	text := strings.TrimSpace(p.editor.Text())
	if text == "" {
		return
	}
	p.editor.SetText("")
	p.undo.Reset()
	u.model.chat.enqueue(id, text)
}

// steerChat sends text into session id's turn running; a refusal puts it
// back in the queue.
func (u *window) steerChat(id, text string) {
	go func() {
		if err := u.client.ChatSteer(u.ctx, id, text); err != nil {
			u.model.chat.requeue(id, []string{text})
			u.chatFailed(err)
		}
	}()
}

// editQueue takes session id's whole queue into the empty composer.
func (u *window) editQueue(p *chatPane, id string) {
	q := u.model.chat.takeQueue(id)
	if len(q) == 0 {
		return
	}
	text := strings.Join(q, chatQueueJoin)
	p.editor.SetText(text)
	p.editor.SetCaret(len([]rune(text)), len([]rune(text)))
}

// chatQueueRow is one queued message's controls: the row, which takes it back
// to the box, Force and ×.
type chatQueueRow struct {
	edit, force, drop ui.Clickable
	dropTip           ui.Tooltip
}

// queueActs runs the queue rows' clicks.
func (u *window) queueActs(gtx C, p *chatPane, id string) {
	for i := range p.queueRows {
		r := &p.queueRows[i]
		switch {
		case r.force.Clicked(gtx):
			if text, ok := u.model.chat.unqueue(id, i); ok {
				u.steerChat(id, text)
			}
		case r.drop.Clicked(gtx):
			u.model.chat.unqueue(id, i)
		case r.edit.Clicked(gtx):
			if text, ok := u.model.chat.unqueue(id, i); ok {
				p.editor.Insert(text)
			}
		}
	}
}

// chatQueue draws session id's queue over the composer: a muted line per
// message, its Force and × shown while the pointer is on it.
func (u *window) chatQueue(gtx C, th *ui.Theme, p *chatPane, id string) D {
	q := u.model.chat.queued(id)
	if len(q) == 0 {
		return D{}
	}
	if len(p.queueRows) < len(q) {
		p.queueRows = append(p.queueRows, make([]chatQueueRow, len(q)-len(p.queueRows))...)
	}
	rows := make([]layout.FlexChild, len(q))
	for i, text := range q {
		r := &p.queueRows[i]
		rows[i] = layout.Rigid(func(gtx C) D { return u.chatQueueLine(gtx, th, r, text) })
	}
	return layout.Inset{Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
	})
}

func (u *window) chatQueueLine(gtx C, th *ui.Theme, r *chatQueueRow, text string) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	shown := r.edit.Hovered() || r.force.Hovered() || r.drop.Hovered()
	ink := th.MutedForeground
	if shown {
		ink = th.Foreground
	}
	force := th.Button(&r.force, i18n.T("chat.queue.force"))
	force.Variant, force.Size = ui.VariantGhost, ui.SizeXS
	drop := th.Button(&r.drop, "")
	drop.Icon, drop.Variant, drop.Size = icQueueDrop, ui.VariantGhost, ui.SizeIconXS
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return r.edit.Layout(gtx, func(gtx C) D {
				rowFill(gtx, th, false, shown)
				return D{Size: gtx.Constraints.Min}
			})
		}),
		layout.Stacked(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Inset{Left: ui.Space(2), Right: ui.Space(1)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return icQueued.Layout(gtx, chatCrumbGlyph, ink) }),
					ui.Gap(2),
					layout.Flexed(1, th.Text(ui.TextSM, strings.ReplaceAll(text, "\n", " ")).In(ink).Truncated().Layout),
					layout.Rigid(func(gtx C) D { return revealed(gtx, shown, force.Layout) }),
					layout.Rigid(func(gtx C) D {
						return th.Tooltip(&u.root, &r.dropTip, i18n.T("chat.queue.drop")).Layout(gtx, func(gtx C) D { return revealed(gtx, shown, drop.Layout) })
					}),
				)
			})
		}),
	)
}

// revealed lays w out, drawn only while shown: hidden, it still takes the
// pointer, so moving onto it shows it.
func revealed(gtx C, shown bool, w layout.Widget) D {
	if !shown {
		defer paint.PushOpacity(gtx.Ops, 0).Pop()
	}
	return w(gtx)
}

// composerKeys runs the queue's keys in the composer: Ctrl+Enter (Cmd+Enter
// on macOS) sends the text into the turn running, or as a message when none
// runs, and ↑ on an empty box takes the queue back to edit.
func (u *window) composerKeys(gtx C, p *chatPane, id string, keys bool) {
	if !keys {
		return
	}
	filters := []event.Filter{
		key.Filter{Focus: &p.editor, Name: key.NameReturn, Required: key.ModShortcut},
		key.Filter{Focus: &p.editor, Name: key.NameEnter, Required: key.ModShortcut},
	}
	if p.editor.Len() == 0 && len(u.model.chat.queued(id)) > 0 {
		filters = append(filters, key.Filter{Focus: &p.editor, Name: key.NameUpArrow})
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		e, isKey := ev.(key.Event)
		if !isKey || e.State != key.Press {
			continue
		}
		if e.Name == key.NameUpArrow {
			u.editQueue(p, id)
			continue
		}
		text := strings.TrimSpace(p.editor.Text())
		if text == "" {
			continue
		}
		if !u.model.chat.isRunning(id) {
			if u.sendChat(p, id) {
				p.toEnd(gtx.Now)
			}
			continue
		}
		p.editor.SetText("")
		p.undo.Reset()
		u.steerChat(id, text)
	}
}
