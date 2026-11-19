package main

import (
	"image"
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
	send, stop       widget.Clickable
	sendTip, stopTip ui.Tooltip
	picker           modelPicker
	// rows brings the conversation's rows in, and seen is the row each key
	// last was, for one that is playing its exit.
	rows ui.Presence
	// lead is the room the last frame put above a conversation shorter than
	// the view, once there was a frame to measure it by.
	lead   int
	leaded bool
	seen   map[string]chatRow
	// stamps are the tooltips of the time under each message, by item id.
	stamps map[string]*ui.Tooltip
	// copies are the copy buttons of the messages, by item id.
	copies map[string]*chatCopy
	folds  map[string]*chatFold
	// optionMenus are the menus of the option chips, by category.
	optionMenus map[string]*ui.MenuState
	answers     map[chatAnswer]*widget.Clickable
	md          markdown.Cache
	// unsent is a message the backend refused, handed back to an empty
	// editor on the next frame.
	mu     sync.Mutex
	unsent string
}

type chatFold struct {
	click widget.Clickable
	open  bool
}

type chatAnswer struct{ item, option string }

func newChatPane() *chatPane {
	p := &chatPane{}
	p.list.Axis = layout.Vertical
	p.editor.Submit = true
	p.md = markdown.Cache{}
	p.folds = map[string]*chatFold{}
	p.answers = map[chatAnswer]*widget.Clickable{}
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
	if !read {
		return keys
	}
	return v.rows.Update(now, keys)
}

func (v *chatView) fold(key string) *chatFold {
	f, ok := v.folds[key]
	if !ok {
		f = new(chatFold)
		v.folds[key] = f
	}
	return f
}

func (v *chatView) answer(item, option string) *widget.Clickable {
	k := chatAnswer{item, option}
	c, ok := v.answers[k]
	if !ok {
		c = new(widget.Clickable)
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
	u.chatActs(gtx, p, sv, keys, working, permissionWaiting(items))
	rows := chatRowsWith(items, working, gtx.Now, func(turn int) bool { _, ok := u.model.chat.changesOf(sv.ID, turn); return ok })
	th := u.theme.On(u.theme.Background)
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			shown := p.enter(gtx.Now, rows, u.model.chat.holds(sv.ID))
			atEnd := !p.list.Position.BeforeEnd
			// A conversation shorter than the view rests on the composer: what
			// the view has over, which is what its content left, goes above it.
			// The content is the list's length less the room of the last frame;
			// the first frame, which has no length yet, has none.
			lead := 0
			if p.leaded && len(shown) > 0 {
				lead = max(gtx.Constraints.Max.Y-(p.list.Position.Length-p.lead), 0)
			}
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
					return p.rows.Layout(gtx, key, func(gtx C) D {
						return chatColumn(gtx, top, 0, chatTextInset, func(gtx C) D { return u.chatRow(gtx, th, p, sv, p.seen[key]) })
					})
				})
				return d, m.Stop()
			}
			d, call := drawList(lead)
			// The lead of the last frame is stale when a row grew in this one:
			// the rows would overshoot the view for a frame and the list draw its
			// scrollbar over content that fits. Measure again with the room the
			// rows take now, and draw that.
			if len(shown) > 0 {
				if fit := max(gtx.Constraints.Max.Y-(p.list.Position.Length-lead), 0); fit != lead {
					lead = fit
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
			p.lead, p.leaded = lead, true
			// Content that changed in this frame leaves its room to the next:
			// ask for it rather than leave the rows where they were put.
			if len(shown) > 0 && lead != max(gtx.Constraints.Max.Y-(p.list.Position.Length-lead), 0) {
				gtx.Execute(op.InvalidateCmd{})
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
// with its reject.
func (u *window) chatActs(gtx C, p *chatPane, sv sessionView, keys, working bool, waiting *backend.ChatItem) {
	p.mu.Lock()
	if p.unsent != "" && p.editor.Len() == 0 {
		p.editor.SetText(p.unsent)
	}
	p.unsent = ""
	p.mu.Unlock()
	// The model panel's search holds the keys while the panel is open.
	if keys && !p.picker.panel.Opened() && !gtx.Focused(&p.editor) {
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
		case !working:
			u.sendChat(p, sv.ID)
		}
	}
	if p.send.Clicked(gtx) && !working {
		u.sendChat(p, sv.ID)
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

// sendChat hands the editor's text to the agent. The person's message comes
// back as an item; a refused one goes back into the editor.
func (u *window) sendChat(p *chatPane, id string) {
	text := strings.TrimSpace(p.editor.Text())
	if text == "" {
		return
	}
	p.editor.SetText("")
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
// changes under it. Only the session the footer follows carries the footer's
// items: another pane of a layout has the provider's chips and Send.
func (u *window) chatComposer(gtx C, th *ui.Theme, p *chatPane, sv sessionView, working bool) D {
	name := sv.Kind
	if info, ok := u.model.providers.find(sv.Kind); ok && info.name != "" {
		name = info.name
	}
	var ft chatFooter
	if u.footer.shown.sessionID == sv.ID {
		l, _ := u.footer.snapshot()
		ft = chatFootSlots(l, u.footer.shown)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			ta := th.Textarea(&p.editor, i18n.T("chat.composer.placeholder", i18n.Args{"provider": name}))
			ta.MaxHeight, ta.MinHeight, ta.Radius = chatComposerMax, chatComposerMin, ui.RadiusLG
			ta.Size, ta.Leading, ta.PadX, ta.PadY = markdown.BodySize, markdown.BodySize*markdown.LineHeightScale, chatComposerPadX, chatComposerPadY
			ta.Foot = u.chatChips(th, p, sv, working, ft)
			return ta.Layout(gtx)
		}),
		layout.Rigid(func(gtx C) D { return u.chatCrumbs(gtx, th, sv, ft) }),
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
