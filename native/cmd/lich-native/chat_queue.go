package main

import (
	"context"
	"log"
	"runtime"
	"slices"

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
// in one message, Send now (or Ctrl+Enter) sends all of it into the turn
// running, and ↑ on an empty box takes the queue back to edit.

var (
	icQueued    = icons.Lucide("clock")
	icQueueDrop = icons.Lucide("x")
	icQueueSend = icons.Lucide("arrow-up")
)

// chatQueueJoin is what the queued messages are joined with into the one the
// turn's end sends.
const chatQueueJoin = "\n\n"

func (s *chatStore) enqueue(id string, m chatMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.queue == nil {
		s.queue = map[string][]chatMessage{}
	}
	s.queue[id] = append(s.queue[id], m)
	delete(s.suggestion, id)
}

// queued is session id's queue, a copy.
func (s *chatStore) queued(id string) []chatMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.queue[id])
}

// unqueue takes message i out of session id's queue.
func (s *chatStore) unqueue(id string, i int) (chatMessage, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	q := s.queue[id]
	if i < 0 || i >= len(q) {
		return chatMessage{}, false
	}
	m := q[i]
	s.queue[id] = slices.Delete(q, i, i+1)
	return m, true
}

// takeQueue empties session id's queue and returns it.
func (s *chatStore) takeQueue(id string) []chatMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	q := s.queue[id]
	delete(s.queue, id)
	return q
}

// requeue puts msgs back at the head of session id's queue.
func (s *chatStore) requeue(id string, msgs []chatMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.queue == nil {
		s.queue = map[string][]chatMessage{}
	}
	s.queue[id] = append(slices.Clone(msgs), s.queue[id]...)
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
	joined := joinMessages(q)
	go func() {
		if err := m.client.ChatPrompt(ctx, t.SessionID, joined.text, joined.files); err != nil {
			log.Printf("chat queue of %s: %v", t.SessionID, err)
			m.chat.requeue(t.SessionID, q)
			m.invalidate()
		}
	}()
}

// queueChat puts the composer's message in session id's queue.
func (u *window) queueChat(p *chatPane, id string) {
	if u.takeShell(p, id) {
		return
	}
	m := p.composed()
	if m.empty() || (len(m.files) == 0 && u.openTools(p, id, m.text)) {
		return
	}
	p.clearComposer()
	u.model.chat.enqueue(id, m)
}

// steerChat sends m into session id's turn running; a refusal puts it back in
// the queue, and so does a turn not begun, its message still waiting to go:
// steered, m would reach the agent first.
func (u *window) steerChat(id string, m chatMessage) {
	if u.model.chat.holding(id) {
		u.model.chat.requeue(id, []chatMessage{m})
		return
	}
	go func() {
		if err := u.client.ChatSteer(u.ctx, id, m.text, m.files); err != nil {
			u.model.chat.requeue(id, []chatMessage{m})
			u.chatFailed(err)
		}
	}()
}

// sendQueueNow sends session id's queue, and what the box holds after it,
// in one message: into the turn running, or as the next one when none runs
// (a stopped turn keeps its queue). A box in shell mode keeps its command.
func (u *window) sendQueueNow(p *chatPane, id string) bool {
	if len(u.model.chat.queued(id)) == 0 {
		return false
	}
	q := u.model.chat.takeQueue(id)
	if !u.model.chat.isRunning(id) {
		p.restore(joinMessages(q))
		return u.sendChat(p, id)
	}
	if m := p.composed(); !m.empty() && !u.inShellMode(p, id) {
		q = append(q, m)
		p.clearComposer()
	}
	u.steerChat(id, joinMessages(q))
	return true
}

// inShellMode is whether session id's box runs its line as a shell command.
func (u *window) inShellMode(p *chatPane, id string) bool {
	sv, ok := u.model.sessionViewOf(id)
	if !ok {
		return false
	}
	_, shell := p.shellLine(sv.Kind)
	return shell
}

// editQueue takes session id's whole queue into the empty composer.
func (u *window) editQueue(p *chatPane, id string) {
	q := u.model.chat.takeQueue(id)
	if len(q) == 0 {
		return
	}
	p.restore(joinMessages(q))
}

// chatQueueRow is one queued message's controls: the row, which takes it back
// to the box, and ×.
type chatQueueRow struct {
	edit, drop ui.Clickable
	dropTip    ui.Tooltip
}

// queueActs runs the queue rows' clicks.
func (u *window) queueActs(gtx C, p *chatPane, id string) {
	if p.queueSend.Clicked(gtx) && u.sendQueueNow(p, id) {
		p.toEnd(gtx.Now)
	}
	for i := range p.queueRows {
		r := &p.queueRows[i]
		switch {
		case r.drop.Clicked(gtx):
			u.model.chat.unqueue(id, i)
		case r.edit.Clicked(gtx):
			if m, ok := u.model.chat.unqueue(id, i); ok {
				p.editor.Insert(m.text)
				p.attached = addFiles(p.attached, m.files)
			}
		}
	}
}

// chatQueue draws session id's queue over the composer: a head with the count
// and Send now, which sends it all into the turn as Ctrl+Enter does, then a
// muted line per message, its × shown while the pointer is on it.
func (u *window) chatQueue(gtx C, th *ui.Theme, p *chatPane, id string) D {
	q := u.model.chat.queued(id)
	if len(q) == 0 {
		return D{}
	}
	if len(p.queueRows) < len(q) {
		p.queueRows = append(p.queueRows, make([]chatQueueRow, len(q)-len(p.queueRows))...)
	}
	rows := make([]layout.FlexChild, 0, len(q)+1)
	rows = append(rows, layout.Rigid(func(gtx C) D { return u.chatQueueHead(gtx, th, p, len(q)) }))
	for i, m := range q {
		r := &p.queueRows[i]
		rows = append(rows, layout.Rigid(func(gtx C) D { return u.chatQueueLine(gtx, th, r, m.line()) }))
	}
	return layout.Inset{Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
	})
}

// chatQueueHead is the queue's head line: the clock glyph and the count muted,
// and at its end Send now with the chord that does the same.
func (u *window) chatQueueHead(gtx C, th *ui.Theme, p *chatPane, n int) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	send := th.Button(&p.queueSend, i18n.T("chat.queue.sendNow"))
	send.Icon, send.Variant, send.Size = icQueueSend, ui.VariantGhost, ui.SizeXS
	chord := formatCombo(combo{Mod: true, Key: "Enter"}, runtime.GOOS == "darwin")
	return layout.Inset{Left: ui.Space(2), Right: ui.Space(1), Bottom: ui.Space(0.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return icQueued.Layout(gtx, chatCrumbGlyph, th.MutedForeground) }),
			ui.Gap(2),
			layout.Flexed(1, th.Text(ui.TextXS, i18n.T("chat.queue.count", i18n.Args{"count": n})).Muted().Layout),
			layout.Rigid(send.Layout),
			ui.Gap(1),
			layout.Rigid(th.KeyCap(chord).Layout),
		)
	})
}

func (u *window) chatQueueLine(gtx C, th *ui.Theme, r *chatQueueRow, text string) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	shown := r.edit.Hovered() || r.drop.Hovered()
	ink := th.MutedForeground
	if shown {
		ink = th.Foreground
	}
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
					// the head's glyph stands for every line: the text starts under its count
					layout.Rigid(layout.Spacer{Width: chatCrumbGlyph}.Layout),
					ui.Gap(2),
					layout.Flexed(1, th.Text(ui.TextSM, text).In(ink).Truncated().Layout),
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
// on macOS) sends the queue and the text after it into the turn running, or
// the text alone when nothing is queued, or as a message when none runs, ↑ on an empty box takes the queue back to edit, and otherwise the
// arrows walk the messages sent. Shift+Tab cycles the mode, as in Claude
// Code's prompt; held here, it never moves the focus off the box. Tab takes
// the suggestion the idle box shows, and is held only while it shows one.
func (u *window) composerKeys(gtx C, p *chatPane, id string, sent []string, keys bool) {
	if !keys {
		return
	}
	filters := []event.Filter{
		key.Filter{Focus: &p.editor, Name: key.NameReturn, Required: key.ModShortcut},
		key.Filter{Focus: &p.editor, Name: key.NameEnter, Required: key.ModShortcut},
		key.Filter{Focus: &p.editor, Name: key.NameTab, Required: key.ModShift},
	}
	if u.shownSuggestion(p, id) != "" {
		filters = append(filters, key.Filter{Focus: &p.editor, Name: key.NameTab})
	}
	queue := p.editor.Len() == 0 && len(p.attached) == 0 && len(u.model.chat.queued(id)) > 0
	filters = append(filters, p.history.filters(&p.editor, sent, queue)...)
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		e, isKey := ev.(key.Event)
		if !isKey || e.State != key.Press {
			continue
		}
		switch {
		case e.Name == key.NameUpArrow && queue:
			u.editQueue(p, id)
			continue
		case e.Name == key.NameUpArrow:
			p.history.back(&p.editor, sent)
			continue
		case e.Name == key.NameDownArrow:
			p.history.forward(&p.editor, sent)
			continue
		case e.Name == key.NameTab && !e.Modifiers.Contain(key.ModShift):
			u.takeSuggestion(p, id)
			continue
		case e.Name == key.NameTab:
			u.cycleMode(id)
			continue
		}
		if u.sendQueueNow(p, id) {
			p.toEnd(gtx.Now)
			continue
		}
		m := p.composed()
		if m.empty() || (len(m.files) == 0 && u.openTools(p, id, m.text)) || u.takeShell(p, id) {
			continue
		}
		if !u.model.chat.isRunning(id) {
			if u.sendChat(p, id) {
				p.toEnd(gtx.Now)
			}
			continue
		}
		p.clearComposer()
		u.steerChat(id, m)
	}
}
