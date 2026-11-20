package main

import (
	"image"
	"slices"
	"strings"
	"sync"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/markdown"
)

var (
	// chatPadX is the breathing edge of a screen on the island, kept either
	// side of the chat in a pane of any width, and chatBoxMax the widest the
	// composer's box grows: wide enough to hold a long line of code or a
	// paragraph at the prose size without the eye losing its line.
	chatPadX   = ui.Space(6)
	chatBoxMax = unit.Dp(960)
	// chatTextInset is how far the conversation sits inside the composer's
	// edges, so its text lines up inside the box rather than with its border.
	chatTextInset = ui.Space(4)
	chatRowGap    = ui.Space(3.5)
	// chatTurnGap is the room before a turn starts, in the person's message or
	// the day line over it: more than between a turn's own rows, so turns read
	// as groups.
	chatTurnGap = ui.Space(9)
	// chatTopPad keeps the first row clear of the selector floating over the
	// column's top-right corner.
	chatTopPad = 2*balloonInset + chatSwitchItem
	// chatUserShare is how much of the column the person's message may take,
	// mockup A's max-w-[78%].
	chatUserShare   = float32(0.78)
	chatComposerMax = ui.Space(48)
	// chatComposerMin is the text's floor: three body lines (15sp at 1.5) and
	// the field's padding above and under them.
	chatComposerMin = ui.Space(24)
	chatComposerPad = ui.Space(2)
	// chatComposerGap is the room between the end of the conversation and the
	// top of the composer's box: the last message, and its time under it, never
	// touch the box, whether the list rests on it or is pinned to its end.
	chatComposerGap = ui.Space(6)
	// chatComposerPadX and chatComposerPadY are the room the text and its
	// placeholder keep from the box's left and top, a body line's worth, so a
	// question has air around it.
	chatComposerPadX = ui.Space(4)
	chatComposerPadY = ui.Space(3.5)
)

// chatView is one session's chat, kept across frames.
type chatView struct {
	list             widget.List
	editor           widget.Editor
	send, stop       ui.Clickable
	sendTip, stopTip ui.Tooltip
	picker           modelPicker
	// rows brings the conversation's rows in, and seen is the row each key
	// last was, for one that is playing its exit.
	rows ui.Presence
	seen map[string]chatRow
	// keys are the rows of the frame before, swap the live line turning into
	// the closing line, stream the answer the caret follows, cards the
	// questions drawn waiting, by item id, and run the list's way back to
	// its end.
	keys   []string
	swap   chatSwap
	stream chatStream
	cards  map[string]chatCard
	run    chatToEnd
	// stamps are the tooltips of the time under each message, by item id.
	stamps map[string]*ui.Tooltip
	// copies are the copy buttons of the messages, by item id.
	copies map[string]*chatCopy
	folds  map[string]*chatFold
	// optionMenus are the menus of the option chips, by category.
	optionMenus map[string]*ui.MenuState
	answers     map[chatAnswer]*ui.Clickable
	md          markdown.Cache
	// quizzes are the waiting questions' cards, by item id, and asked the
	// last one the keys were handed to.
	quizzes map[string]*chatQuiz
	asked   string
	// undo is the composer's undo and redo, forgotten when a message is sent.
	undo ui.EditHistory
	// unsent is a message the backend refused, handed back to an empty
	// editor on the next frame.
	mu     sync.Mutex
	unsent string
}

type chatFold struct {
	click ui.Clickable
	open  bool
	// body folds a thought's text; calls fold a group's calls each, by id,
	// and enter brings in the ones that come once the group is drawn.
	body  ui.Collapse
	calls map[string]*chatCall
	enter ui.Presence
	// clicked is when the fold was last clicked.
	clicked time.Time
}

// shut reports whether the fold is folded and done folding, so what it
// hides need not be laid out.
func (f *chatFold) shut(now time.Time) bool {
	return !f.open && now.Sub(f.clicked) >= ui.CollapseDuration
}

// chatCall is one call of a group: its fold, and whether it stood open for
// running the frame before.
type chatCall struct {
	fold    ui.Collapse
	running bool
}

type chatAnswer struct{ item, option string }

func newChatPane() *chatPane {
	p := &chatPane{}
	p.list.Axis = layout.Vertical
	p.editor.Submit = true
	p.md = markdown.Cache{}
	p.folds = map[string]*chatFold{}
	p.answers = map[chatAnswer]*ui.Clickable{}
	p.cards = map[string]chatCard{}
	return p
}

func (v *chatView) optionMenu(category string) *ui.MenuState {
	m, ok := v.optionMenus[category]
	if !ok {
		if v.optionMenus == nil {
			v.optionMenus = map[string]*ui.MenuState{}
		}
		m = new(ui.MenuState)
		v.optionMenus[category] = m
	}
	return m
}

func (v *chatView) stamp(id string) *ui.Tooltip {
	t, ok := v.stamps[id]
	if !ok {
		if v.stamps == nil {
			v.stamps = map[string]*ui.Tooltip{}
		}
		t = new(ui.Tooltip)
		v.stamps[id] = t
	}
	return t
}

// enter takes the conversation's rows into the presence that animates them,
// and answers the keys to draw, in order: the rows, and those that left a
// moment ago. Until the conversation has been read the rows are drawn as they
// are, so a history arriving later does not play as a hundred arrivals.
func (v *chatView) enter(now time.Time, rows []chatRow, read bool) []string {
	if v.seen == nil {
		v.seen = map[string]chatRow{}
	}
	keys := make([]string, len(rows))
	for i, r := range rows {
		keys[i] = chatRowKey(r)
		v.seen[keys[i]] = r
	}
	before := v.keys
	v.keys = keys
	if !read {
		return keys
	}
	shown := v.rows.Update(now, keys)
	// The turn ending turns the live line into the closing line in its slot:
	// neither plays its own motion.
	if done := lastKey(keys); strings.HasPrefix(done, "done:") && slices.Contains(before, chatLiveKey) && !slices.Contains(before, done) {
		v.rows.Settle(done)
		v.swap = chatSwap{key: done, at: now, live: v.seen[chatLiveKey]}
	}
	// The closing line plays no exit: while it fades, the turn that took it
	// away is drawn under it, and it would stand beside that turn's live line.
	return slices.DeleteFunc(shown, func(k string) bool {
		return v.rows.Leaving(k) && (strings.HasPrefix(k, "done:") || k == chatLiveKey && v.swap.playing(now))
	})
}

// lastKey is the last of keys, "" for none.
func lastKey(keys []string) string {
	if len(keys) == 0 {
		return ""
	}
	return keys[len(keys)-1]
}

// call is the group's call id, which stands open for running while the group
// is folded. A call that starts or ends running in a folded group shows or
// goes at once: only a click plays the fold.
func (f *chatFold) call(id string, running bool) *chatCall {
	if f.calls == nil {
		f.calls = map[string]*chatCall{}
	}
	c, ok := f.calls[id]
	if !ok {
		c = new(chatCall)
		f.calls[id] = c
	}
	if c.running != running && !f.open {
		c.fold = ui.Collapse{}
	}
	c.running = running
	return c
}

func (v *chatView) fold(key string) *chatFold {
	f, ok := v.folds[key]
	if !ok {
		f = new(chatFold)
		v.folds[key] = f
	}
	return f
}

func (v *chatView) answer(item, option string) *ui.Clickable {
	k := chatAnswer{item, option}
	c, ok := v.answers[k]
	if !ok {
		c = new(ui.Clickable)
		v.answers[k] = c
	}
	return c
}

// chatScreen draws session sv's conversation over gtx's area, the composer at
// its foot, keys typing into it when the session takes them.
func (u *window) chatScreen(gtx C, sv sessionView, keys bool) {
	p := u.chats.pane(sv.ID)
	items := u.model.chat.of(sv.ID)
	working := chatWorking(sv, u.model.chat.isRunning(sv.ID))
	u.chatActs(gtx, p, sv, keys, working, permissionWaiting(items), questionWaiting(items))
	rows := chatRows(items, working, gtx.Now)
	th := u.theme.On(u.theme.Background)
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			shown := p.enter(gtx.Now, rows, u.model.chat.holds(sv.ID))
			p.runToEnd(gtx, len(shown))
			atEnd := !p.list.Position.BeforeEnd
			drawList := func(lead int) (D, op.CallOp) {
				m := op.Record(gtx.Ops)
				d := th.ThinScrollArea(&p.list).Layout(gtx, len(shown), func(gtx C, i int) D {
					top := chatRowGap
					switch {
					case i == 0:
						top = chatTopPad + gtx.Metric.PxToDp(lead)
					case startsTurn(p.seen[shown[i]], p.seen[shown[i-1]]):
						top = chatTurnGap
					}
					key := shown[i]
					return p.rows.LayoutFlat(gtx, key, func(gtx C) D {
						return chatColumn(gtx, top, 0, chatTextInset, func(gtx C) D { return u.chatRow(gtx, th, p, sv, p.seen[key]) })
					})
				})
				return d, m.Stop()
			}
			// A conversation shorter than the view rests on the composer: what
			// the view has over, which is what its content leaves, goes above it.
			// The list is laid out with none first, because that is the one
			// measure that lays every row out and so tells their length, and
			// again with the room it left, which is what is drawn.
			d, call := drawList(0)
			lead := 0
			if len(shown) > 0 {
				if lead = max(gtx.Constraints.Max.Y-p.list.Position.Length, 0); lead > 0 {
					d, call = drawList(lead)
				}
			}
			call.Add(gtx.Ops)
			// A row that comes in at no height at the list's end makes Gio
			// count the list as scrolled up, and it would no longer follow the end
			// as the row grows: a list that was at its end stays there while its
			// rows move.
			if atEnd && p.rows.Moving() {
				p.list.Position.BeforeEnd = false
			}
			// Once the conversation outgrows the view, where nobody could have
			// scrolled it, it goes to its end and stays there until scrolled up.
			if over := p.list.Position.Length-lead > gtx.Constraints.Max.Y; over != p.list.ScrollToEnd {
				p.list.ScrollToEnd, p.list.Position.BeforeEnd = over, false
				gtx.Execute(op.InvalidateCmd{})
			}
			return d
		}),
		layout.Rigid(func(gtx C) D {
			return chatColumn(gtx, chatComposerGap, chatComposerPad, 0, func(gtx C) D { return u.chatComposer(gtx, th, p, sv, working) })
		}),
	)
}

// chatColumn centres w on the pane's axis, as wide as the composer's box,
// which is the pane less chatPadX each side up to chatBoxMax, and inset from
// its edges by inset; top is above it and bottom under.
func chatColumn(gtx C, top, bottom, inset unit.Dp, w layout.Widget) D {
	full := gtx.Constraints.Max.X
	width := max(min(full-2*gtx.Dp(chatPadX), gtx.Dp(chatBoxMax)), 0)
	side := gtx.Metric.PxToDp((full-width)/2) + inset
	return layout.Inset{Top: top, Left: side, Right: side, Bottom: bottom}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return w(gtx)
	})
}

// chatActs runs the view's keys and clicks before it is drawn: Enter sends,
// and on an empty editor answers a waiting permission with its allow, Esc
// with its reject. A waiting question takes the keys to its card, and a
// message sent past it declines it.
func (u *window) chatActs(gtx C, p *chatPane, sv sessionView, keys, working bool, waiting, asking *backend.ChatItem) {
	p.mu.Lock()
	if p.unsent != "" && p.editor.Len() == 0 {
		p.editor.SetText(p.unsent)
		p.undo.Reset()
	}
	p.unsent = ""
	p.mu.Unlock()
	if typed := u.chats.takeTyped(sv.ID); typed != "" {
		p.editor.Insert(typed)
	}
	p.dropQuizzes(asking)
	if asking != nil {
		u.questionActs(gtx, p, sv, asking)
	}
	// The model panel's search holds the keys while the panel is open.
	switch {
	case !keys || p.picker.panel.Opened():
	case asking != nil:
		p.focusQuestion(gtx, asking, p.quiz(asking.ID, asking.Question.Questions))
	case !gtx.Focused(&p.editor):
		gtx.Execute(key.FocusCmd{Tag: &p.editor})
	}
	if p.editor.Len() > 0 {
		waiting = nil
	}
	for {
		ev, ok := gtx.Event(key.Filter{Focus: &p.editor, Name: key.NameEscape})
		if !ok {
			break
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press && waiting != nil {
			u.answerChat(sv.ID, waiting.ID, permitOption(waiting.Permission, api.PermitRejectOnce))
		}
	}
	p.undo.Handle(gtx, &p.editor)
	for {
		ev, ok := p.editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.SubmitEvent); !ok {
			continue
		}
		switch {
		case waiting != nil:
			u.answerChat(sv.ID, waiting.ID, permitOption(waiting.Permission, api.PermitAllowOnce, api.PermitAllowAlways))
		case !working || asking != nil:
			if u.sendChat(p, sv.ID) {
				p.toEnd(gtx.Now)
			}
		}
	}
	if p.send.Clicked(gtx) && !working && u.sendChat(p, sv.ID) {
		p.toEnd(gtx.Now)
	}
	if p.stop.Clicked(gtx) {
		id := sv.ID
		go func() { u.chatFailed(u.client.ChatCancel(u.ctx, id)) }()
	}
	for k, c := range p.answers {
		if c.Clicked(gtx) {
			u.answerChat(sv.ID, k.item, k.option)
		}
	}
}

// sendChat hands the editor's text to the agent, and reports whether it had
// any. The person's message comes back as an item; a refused one goes back
// into the editor.
func (u *window) sendChat(p *chatPane, id string) bool {
	text := strings.TrimSpace(p.editor.Text())
	if text == "" {
		return false
	}
	p.editor.SetText("")
	p.undo.Reset()
	go func() {
		err := u.client.ChatPrompt(u.ctx, id, text)
		if err == nil {
			return
		}
		p.mu.Lock()
		p.unsent = text
		p.mu.Unlock()
		u.chatFailed(err)
	}()
	return true
}

func (u *window) answerChat(id, item, option string) {
	if option == "" {
		return
	}
	go func() { u.chatFailed(u.client.ChatAnswer(u.ctx, id, item, option)) }()
}

func (u *window) chatFailed(err error) {
	if err == nil {
		return
	}
	u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("chat.view.failed"), Description: backend.ErrorText(err)})
	u.model.invalidate()
}

// chatComposer is the message field with the provider's chips and Send, or
// Stop while a turn runs, inside its box, and the project and branch with the
// changes under it. Every pane has its own line; only the session the footer
// follows has the attach "+" and the context ring.
func (u *window) chatComposer(gtx C, th *ui.Theme, p *chatPane, sv sessionView, working bool) D {
	name := sv.Kind
	if info, ok := u.model.providers.find(sv.Kind); ok && info.name != "" {
		name = info.name
	}
	// The line under the box is the session's own, followed by the footer or
	// not. The attach "+" and the context ring belong to the followed one.
	in := u.footer.shown
	followed := in.sessionID == sv.ID
	if !followed {
		in = u.footer.sessionInput(u.model, sv, in.now)
	}
	l, _ := u.footer.snapshot()
	ft := chatFootSlots(l, in)
	if !followed {
		ft.attach, ft.context = nil, nil
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			ta := th.Textarea(&p.editor, i18n.T("chat.composer.placeholder", i18n.Args{"provider": name}))
			ta.MaxHeight, ta.MinHeight, ta.Radius, ta.Steady, ta.History = chatComposerMax, chatComposerMin, ui.RadiusLG, true, &p.undo
			ta.Size, ta.Leading, ta.PadX, ta.PadY = markdown.BodySize, markdown.BodySize*markdown.LineHeightScale, chatComposerPadX, chatComposerPadY
			ta.Foot = u.chatChips(th, p, sv, working, ft)
			return ta.Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D { return u.chatCrumbs(gtx, th, sv, in, ft) }),
	)
}

// chatUser is the person's message, in an Accent box against the column's
// right edge.
func chatUser(gtx C, th *ui.Theme, text string) D {
	maxW := int(float32(gtx.Constraints.Max.X) * chatUserShare)
	on := th.On(th.Accent)
	return layout.E.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min = image.Point{}
		gtx.Constraints.Max.X = maxW
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, th.Accent, ui.RadiusLG) },
			func(gtx C) D {
				return layout.Inset{Top: ui.Space(2), Bottom: ui.Space(2), Left: ui.Space(3), Right: ui.Space(3)}.Layout(gtx,
					on.Text(ui.TextSM, expandTabs(text)).Wrapped().Layout)
			})
	})
}

// startsTurn is whether row r starts a turn after row prev: the person's
// message, or the day line over it; not the message a day line already opened.
func startsTurn(r, prev chatRow) bool {
	switch {
	case r.kind == chatRowDay:
		return true
	case r.kind == chatRowItem && r.item.Kind == api.ItemUser:
		return prev.kind != chatRowDay
	}
	return false
}
