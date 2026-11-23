package main

import (
	"image"
	"slices"
	"strings"
	"sync"
	"time"

	"gioui.org/io/key"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
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
	// composer's box grows under Comfortable: wide enough to hold a long line of code or a
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
	// chatComposerTint is how far the box's edge goes from the input color
	// toward the theme's primary (centre-mix mockup), and chatComposerFillTint
	// its fill, which the user wanted near the theme's own ground.
	chatComposerTint     = 0.45
	chatComposerFillTint = 0.08
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
	// notes are the clicks on a notification's row, by item id and place.
	notes map[string]*ui.Clickable
	// copies are the copy buttons of the messages, by item id.
	copies map[string]*chatCopy
	folds  map[string]*chatFold
	// optionMenus are the menus of the option chips, by category.
	optionMenus map[string]*ui.MenuState
	answers     map[chatAnswer]*ui.Clickable
	md          markdown.Cache
	// files is the checkout's listing the paths in answers are looked up in,
	// and paths what makes them links this frame.
	files remote[*chatFiles]
	paths *markdown.Paths
	// quizzes are the waiting questions' cards, by item id, and asked the
	// last one the keys were handed to.
	quizzes map[string]*chatQuiz
	asked   string
	// undo is the composer's undo and redo, forgotten when a message is sent.
	undo ui.EditHistory
	// start is the start screen of the chat while it has nothing in it.
	start chatStart
	// queueRows are the controls of the session's queued messages.
	queueRows []chatQueueRow
	// queueSend is the queue's Send now.
	queueSend ui.Clickable
	plan      chatPlanState
	// slash is the list of the provider's commands over the composer, which
	// may grow over the conversation up to slashRoom, the pane's height (on
	// the start screen, what is above its composer and the composer).
	slash     chatSlash
	slashRoom int
	// mention is the list of the checkout's paths an @ opens, in the slash
	// list's place.
	mention chatMention
	// tools is the /mcp or /skills panel, in the slash list's place, and
	// search the Ctrl+R search of the messages sent; history is the ↑/↓ walk.
	tools chatTools
	// shellsChip is the composer's chip of the shells running.
	shellsChip footerWidgets
	search     chatSearch
	history    chatHistory
	// attached are the files the next message carries, drawn as cards over
	// the text in cardList; fileCards are the cards' controls, by path in the
	// composer and by item and path in a sent message.
	attached  []backend.Attachment
	cardList  layout.List
	fileCards map[string]*chatFileCard
	// unsent is a message the backend refused, handed back to the composer,
	// ahead of what was typed since, on the next frame.
	mu     sync.Mutex
	unsent chatMessage
}

type chatFold struct {
	click ui.Clickable
	open  bool
	// body folds a thought's text; calls fold a group's calls each, by id,
	// and enter brings in the ones that come once the group is drawn.
	body  ui.Collapse
	calls map[string]*ui.Collapse
	enter ui.Presence
	// turned is when the fold last opened or shut.
	turned time.Time
	// calling is whether the group's turn was in a run of tool calls the
	// frame before.
	calling bool
}

// shut reports whether the fold is folded and done folding, so what it
// hides need not be laid out.
func (f *chatFold) shut(now time.Time) bool {
	return !f.open && now.Sub(f.turned) >= ui.CollapseDuration
}

// follow opens a group as its turn starts a run of tool calls and shuts it
// as the run ends, when the model writes text or the turn is over. Only the
// run's edges move it, so a click in between holds until the run ends.
func (f *chatFold) follow(now time.Time, calling bool) {
	if calling != f.calling {
		f.open, f.turned = calling, now
	}
	f.calling = calling
}

type chatAnswer struct{ item, option string }

func newChatPane() *chatPane {
	p := &chatPane{}
	p.list.Axis = layout.Vertical
	p.cardList.Axis = layout.Horizontal
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
	// A conversation replaced whole, by /new or a history read again on
	// another, lands as it is rather than playing every old row out.
	if len(before) > 0 && !slices.ContainsFunc(keys, func(k string) bool { return slices.Contains(before, k) }) {
		v.rows = ui.Presence{}
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

// call is the fold of the group's call id.
func (f *chatFold) call(id string) *ui.Collapse {
	if f.calls == nil {
		f.calls = map[string]*ui.Collapse{}
	}
	c, ok := f.calls[id]
	if !ok {
		c = new(ui.Collapse)
		f.calls[id] = c
	}
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
	// The chat's own turn stirs the backdrop, not the session's status: a
	// session reports itself busy while its chat opens, and can stay so.
	u.backdrops.stirOf(sv.ID).keep(gtx.Now, u.model.chat.isRunning(sv.ID), p.editor.Len(), p.start.empty)
	u.chatActs(gtx, p, sv, keys, working, permissionWaiting(items), questionWaiting(items))
	rows := chatRows(items, sv.Kind, working, gtx.Now)
	u.terms.readLinkKey(gtx)
	p.paths = u.chatPaths(p, sv, len(items))
	th := u.theme.On(u.theme.Background)
	p.slashRoom = gtx.Constraints.Max.Y
	list := func(gtx C) D {
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
					return chatColumn(gtx, u.chatWidth, top, 0, chatTextInset, func(gtx C) D { return u.chatRow(gtx, th, p, sv, p.seen[key]) })
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
	}
	composer := func(gtx C) D {
		return chatColumn(gtx, u.chatWidth, chatComposerGap, chatComposerPad, 0, func(gtx C) D { return u.chatComposer(gtx, th, p, sv, working) })
	}
	// A turn of the chat's own, not a hook's report, keeps the start screen
	// away: a session reports itself busy while its chat opens.
	if p.start.shows(gtx.Now, p.start.emptyNow(u.model.chat.holds(sv.ID), sv, items) && !u.model.chat.isRunning(sv.ID)) {
		u.chatStartScreen(gtx, th, p, sv, list, composer)
		return
	}
	layout.Flex{Axis: layout.Vertical}.Layout(gtx, layout.Flexed(1, list), layout.Rigid(composer))
}

// chatColumn centres w on the pane's axis, as wide as the composer's box,
// and inset from its edges by inset; top is above it and bottom under.
func chatColumn(gtx C, choice string, top, bottom, inset unit.Dp, w layout.Widget) D {
	full := gtx.Constraints.Max.X
	width := chatColumnWidth(gtx, full, choice)
	side := gtx.Metric.PxToDp((full-width)/2) + inset
	return layout.Inset{Top: top, Left: side, Right: side, Bottom: bottom}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return w(gtx)
	})
}

// chatColumnWidth is the composer's box in a pane full wide: the pane less
// chatPadX each side, up to the chat width setting's cap.
func chatColumnWidth(gtx C, full int, choice string) int {
	width := full - 2*gtx.Dp(chatPadX)
	if box := chatBox(choice); box > 0 {
		width = min(width, gtx.Dp(box))
	}
	return max(width, 0)
}

// chatActs runs the view's keys and clicks before it is drawn: Enter sends,
// and on an empty editor answers a waiting permission with its allow, Esc
// with its reject. A waiting question takes the keys to its card, and a
// message sent past it declines it.
func (u *window) chatActs(gtx C, p *chatPane, sv sessionView, keys, working bool, waiting, asking *backend.ChatItem) {
	p.mu.Lock()
	if !p.unsent.empty() {
		p.restore(p.unsent)
		p.undo.Reset()
	}
	p.unsent = chatMessage{}
	p.mu.Unlock()
	if typed := u.chats.takeTyped(sv.ID); typed != "" {
		p.editor.Insert(typed)
	}
	p.attached = addFiles(p.attached, u.chats.takeAttached(sv.ID))
	p.cardActs(gtx)
	p.dropQuizzes(asking)
	if asking != nil {
		u.questionActs(gtx, p, sv, asking)
	}
	// Typing in the box shuts the panel, and so does a click back in it while
	// a field of the panel has the keys, as one outside a popover does.
	if p.tools.opened() && (p.editor.Len() != p.tools.base || p.tools.keyTag(p) != &p.editor && !p.tools.focus && gtx.Focused(&p.editor)) {
		p.tools.shut()
	}
	sent, submit := sentHistory(u.model.chat.of(sv.ID)), false
	if keys && asking == nil && !p.picker.panel.Opened() && !p.tools.opened() {
		submit = p.search.acts(gtx, &p.editor, sent)
	}
	p.slash.blocked = !keys || asking != nil || p.picker.panel.Opened() || p.tools.opened() || p.search.open
	p.mention.blocked = p.slash.blocked
	if !p.slash.blocked {
		p.slash.act(gtx, &p.editor)
		p.mention.act(gtx, &p.editor)
	}
	if keys {
		u.shellsChipActs(gtx, p)
		u.toolsActs(gtx, p, sv.ID)
	}
	// The model panel's search holds the keys while the panel is open.
	switch {
	case !keys || p.picker.panel.Opened() || p.tools.opened() || p.search.open:
	case asking != nil:
		p.focusQuestion(gtx, asking, p.quiz(asking.ID, asking.Question.Questions))
	case !gtx.Focused(&p.editor):
		gtx.Execute(key.FocusCmd{Tag: &p.editor})
	}
	u.escapeChat(gtx, p, sv.ID, waiting, asking)
	if p.editor.Len() > 0 || len(p.attached) > 0 {
		waiting = nil
		u.model.chat.dropSuggestion(sv.ID)
	}
	u.queueActs(gtx, p, sv.ID)
	u.composerKeys(gtx, p, sv.ID, sent, keys && asking == nil && !p.search.open)
	if keys && asking == nil {
		u.backgroundKeys(gtx, p, sv.ID)
	}
	u.pasteKeys(gtx, p, sv.ID)
	p.undo.Handle(gtx, &p.editor)
	for {
		ev, ok := p.editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.SubmitEvent); ok {
			u.submitChat(gtx, p, sv.ID, working, waiting, asking)
		}
	}
	if submit {
		u.submitChat(gtx, p, sv.ID, working, waiting, asking)
	}
	if p.send.Clicked(gtx) && !working && !p.search.open && u.sendChat(p, sv.ID) {
		p.toEnd(gtx.Now)
	}
	if p.stop.Clicked(gtx) && !u.unholdChat(p, sv.ID) {
		id := sv.ID
		go func() { u.chatFailed(u.client.ChatCancel(u.ctx, id)) }()
	}
	for k, c := range p.answers {
		if c.Clicked(gtx) {
			u.answerChat(sv.ID, k.item, k.option)
		}
	}
}

// submitChat is Enter in the box: it allows a waiting permission on an empty
// box, queues the text while a turn runs, and otherwise sends it.
func (u *window) submitChat(gtx C, p *chatPane, id string, working bool, waiting, asking *backend.ChatItem) {
	switch {
	case waiting != nil:
		u.answerChat(id, waiting.ID, permitOption(waiting.Permission, api.PermitAllowOnce, api.PermitAllowAlways))
	case asking == nil && u.model.chat.isRunning(id):
		u.queueChat(p, id)
	case !working || asking != nil:
		if u.sendChat(p, id) {
			p.toEnd(gtx.Now)
		}
	}
}

// escapeChat runs Esc on the box once the slash list and the panel took
// theirs; the search reads its own on its field, as Claude Code's prompt does: it takes a waiting message
// back, rejects a waiting permission on an empty box, and otherwise stops the
// turn running as Stop does, the box's text kept. A waiting question has the
// keys on its card.
func (u *window) escapeChat(gtx C, p *chatPane, id string, waiting, asking *backend.ChatItem) {
	for {
		ev, ok := gtx.Event(key.Filter{Focus: &p.editor, Name: key.NameEscape})
		if !ok {
			return
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press || u.unholdChat(p, id) {
			continue
		}
		switch {
		case waiting != nil:
			if p.editor.Len() == 0 && len(p.attached) == 0 {
				u.answerChat(id, waiting.ID, permitOption(waiting.Permission, api.PermitRejectOnce))
			}
		case asking == nil && u.model.chat.isRunning(id):
			go func() { u.chatFailed(u.client.ChatCancel(u.ctx, id)) }()
		}
	}
}

// sendChat hands the composer's message to the agent, once it waited
// chatSendHold, and reports whether it had any; a refused one goes back into
// the composer. /new alone starts an empty conversation instead, and a "!"
// line runs in the shell.
func (u *window) sendChat(p *chatPane, id string) bool {
	if u.takeShell(p, id) {
		return true
	}
	m := p.composed()
	if m.empty() || (len(m.files) == 0 && u.openTools(p, id, m.text)) {
		return false
	}
	p.clearComposer()
	if isNewCommand(m.text) && len(m.files) == 0 {
		go func() { u.chatFailed(u.client.ChatNew(u.ctx, id)) }()
		return true
	}
	// A message sent while the agent's questions wait declines them, and goes
	// at once: Esc there is the questions'.
	if u.model.chat.isRunning(id) {
		go func() {
			if err := u.client.ChatPrompt(u.ctx, id, m.text, m.files); err != nil {
				u.chatRefused(p, m, err)
			}
		}()
		return true
	}
	u.holdChat(p, id, m)
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
		layout.Rigid(func(gtx C) D { return u.chatPlan(gtx, th, &p.plan, sv.ID, working) }),
		layout.Rigid(func(gtx C) D { return u.chatQueue(gtx, th, p, sv.ID) }),
		layout.Rigid(func(gtx C) D {
			// the box's fill keeps near the theme's own ground and its edge
			// leans to the theme's primary, as the backdrop around it does
			// in shell mode it leans to ToneWait instead, the tone of the bolt
			// on a mode that runs unchecked: the command runs with no
			// permission check
			lean := th.Primary
			if _, shell := p.shellLine(sv.Kind); shell {
				lean = th.ToneWait
			}
			tinted := *th
			tinted.Palette.Input = ui.MixOKLab(th.Palette.Input, lean, chatComposerFillTint)
			hint := u.shownSuggestion(p, sv.ID)
			if hint == "" {
				hint = i18n.T("chat.composer.placeholder", i18n.Args{"provider": name})
			}
			ta := tinted.Textarea(&p.editor, hint)
			ta.MaxHeight, ta.MinHeight, ta.Radius, ta.Steady, ta.History = chatComposerMax, chatComposerMin, ui.RadiusLG, true, &p.undo
			ta.Size, ta.Leading, ta.PadX, ta.PadY = markdown.BodySize, markdown.BodySize*markdown.LineHeightScale, chatComposerPadX, chatComposerPadY
			// the cards stand over the text, adding to the box's ceiling
			// rather than taking from the text's room
			if len(p.attached) > 0 {
				ta.MaxHeight += chatCardSize + chatComposerPadY
			}
			ta.Head = func(gtx C) D { return u.composerCards(gtx, th, p, tinted.Palette.Input) }
			ta.Foot = u.chatChips(th, p, sv, working, ft)
			d := ta.Layout(gtx)
			ui.InsetBorder(gtx, d.Size, gtx.Dp(ui.RadiusLG), th.Over(ui.MixOKLab(th.Palette.Input, lean, chatComposerTint)))
			p.slash.refresh(gtx, &p.editor, slashCommands(u.model.chat.commandsOf(sv.ID), sv.Kind))
			gtx.Constraints.Max.X = d.Size.X
			p.slash.layout(gtx, th, p.slashRoom-d.Size.Y-gtx.Dp(2*chatComposerGap))
			files, _, _ := p.files.get()
			p.mention.refresh(gtx, &p.editor, files)
			p.mention.layout(gtx, th, p.slashRoom-d.Size.Y-gtx.Dp(2*chatComposerGap))
			u.chatToolsPanel(gtx, th, p, sv, p.slashRoom-d.Size.Y-gtx.Dp(2*chatComposerGap))
			return d
		}),
		layout.Rigid(func(gtx C) D {
			// on the start screen the line sits on the backdrop, and reads in
			// the foreground
			crumbs, size := th, ui.TextXS
			if p.start.empty {
				bright := *th
				bright.Palette.MutedForeground = th.Foreground
				crumbs, size = &bright, chatStartCrumbSize
			}
			d := u.chatCrumbs(gtx, crumbs, sv, in, ft, size)
			p.start.lineH = d.Size.Y
			return d
		}),
	)
}

// chatUser is the person's message, in an Accent box against the column's
// right edge, its text Markdown as the answers' is; key names it in p's cache.
func (u *window) chatUser(gtx C, th *ui.Theme, p *chatPane, key, text string) D {
	return u.chatUserFrom(gtx, th, p, key, "", text)
}

// chatUserFrom is chatUser with a muted line over the text naming who sent it,
// none when from is empty.
func (u *window) chatUserFrom(gtx C, th *ui.Theme, p *chatPane, key, from, text string) D {
	maxW := int(float32(gtx.Constraints.Max.X) * chatUserShare)
	on := th.On(th.Accent)
	return layout.E.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min = image.Point{}
		gtx.Constraints.Max.X = maxW
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, th.Accent, ui.RadiusLG) },
			func(gtx C) D {
				return layout.Inset{Top: ui.Space(2), Bottom: ui.Space(2), Left: ui.Space(3), Right: ui.Space(3)}.Layout(gtx, func(gtx C) D {
					body := func(gtx C) D { return u.userMarkdown(gtx, on, p, key, text) }
					if from == "" {
						return body(gtx)
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(on.Text(ui.TextXS, from).Muted().Layout),
						ui.VGap(0.5),
						layout.Rigid(body),
					)
				})
			})
	})
}

// userMarkdown is the person's text as Markdown at the box's TextSM: the
// renderer sets prose at markdown.BodySize, so the sp are scaled to match.
// Every line they broke stays broken, as they typed it.
func (u *window) userMarkdown(gtx C, th *ui.Theme, p *chatPane, key, text string) D {
	gtx.Metric.PxPerSp *= float32(ui.TextSM) / float32(markdown.BodySize)
	notify := func(t ui.Toast) { u.toaster.Show(t) }
	links := markdown.Links{Open: func(url string) { openLink(u.client, notify, url) }, Paths: p.paths}
	dims := p.md.LayoutLinks(gtx, th, "user:"+key, hardBreaks(text), links)
	// the message as typed, for whoever reads the window without seeing it
	defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()
	semantic.LabelOp(text).Add(gtx.Ops)
	return dims
}

// hardBreaks ends every line of text that another follows in a Markdown
// hard break, two spaces, except inside a fenced code block: a chat message's
// newline is a new line, where Markdown would fold it into a space.
func hardBreaks(text string) string {
	lines := strings.Split(text, "\n")
	fenced := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
			continue
		}
		if fenced || i == len(lines)-1 || line == "" || lines[i+1] == "" || strings.HasPrefix(strings.TrimSpace(lines[i+1]), "```") {
			continue
		}
		lines[i] = line + "  "
	}
	return strings.Join(lines, "\n")
}

// startsTurn is whether row r starts a turn after row prev: the person's
// message, or the day line over it; not the message a day line already opened,
// nor a notification right under another, which read as one list of rows.
func startsTurn(r, prev chatRow) bool {
	switch {
	case r.kind == chatRowDay:
		return true
	case r.kind == chatRowItem && r.item.Kind == api.ItemUser:
		return prev.kind != chatRowDay && !(isEnvelope(r) && isEnvelope(prev))
	}
	return false
}

// isEnvelope is whether row r is a notification lich handed the agent: not a
// task or a request, which stand as a message of the person's and start a
// turn.
func isEnvelope(r chatRow) bool {
	if r.kind != chatRowItem || r.item.Kind != api.ItemUser || !strings.HasPrefix(r.item.Text, envelopeOpen) {
		return false
	}
	head, _, _ := strings.Cut(r.item.Text, "\n")
	return !strings.Contains(head, ` kind="request"`) && !strings.Contains(head, ` kind="task"`)
}
