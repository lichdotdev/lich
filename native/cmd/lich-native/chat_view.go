package main

import (
	"image"
	"strings"
	"sync"

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
	// chatColumnMax and chatColumnPadX are mockup A's reading column,
	// max-w-[720px] px-6.
	chatColumnMax  = unit.Dp(720)
	chatColumnPadX = ui.Space(6)
	chatRowGap     = ui.Space(3.5)
	// chatTopPad keeps the first row clear of the selector floating over the
	// column's top-right corner.
	chatTopPad = 2*balloonInset + chatSwitchItem
	// chatUserShare is how much of the column the person's message may take,
	// mockup A's max-w-[78%].
	chatUserShare   = float32(0.78)
	chatComposerMax = ui.Space(48)
	chatComposerPad = ui.Space(2)
)

// chatView is one session's chat, kept across frames.
type chatView struct {
	list       widget.List
	editor     widget.Editor
	send, stop widget.Clickable
	folds      map[string]*chatFold
	answers    map[chatAnswer]*widget.Clickable
	md         markdown.Cache
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
	rows := chatRows(items, working, gtx.Now)
	th := u.theme.On(u.theme.Background)
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D {
			d := th.ThinScrollArea(&p.list).Layout(gtx, len(rows), func(gtx C, i int) D {
				top := chatRowGap
				if i == 0 {
					top = chatTopPad
				}
				return chatColumn(gtx, top, 0, func(gtx C) D { return u.chatRow(gtx, th, p, sv, rows[i]) })
			})
			// A conversation shorter than the view starts at its top; once it
			// outgrows the view, where nobody could have scrolled it, it goes to
			// its end and stays there until scrolled up.
			if over := p.list.Position.Length > gtx.Constraints.Max.Y; over != p.list.ScrollToEnd {
				p.list.ScrollToEnd, p.list.Position.BeforeEnd = over, false
				gtx.Execute(op.InvalidateCmd{})
			}
			return d
		}),
		layout.Rigid(func(gtx C) D {
			return chatColumn(gtx, chatComposerPad, chatComposerPad, func(gtx C) D { return u.chatComposer(gtx, th, p, sv, working) })
		}),
	)
}

// chatColumn centres w in the reading column, top above it and bottom under.
func chatColumn(gtx C, top, bottom unit.Dp, w layout.Widget) D {
	full := gtx.Constraints.Max.X
	width := min(full, gtx.Dp(chatColumnMax))
	side := gtx.Metric.PxToDp((full-width)/2) + chatColumnPadX
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
	if keys && !gtx.Focused(&p.editor) {
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

// chatComposer is the message field, and Send, or Stop while a turn runs.
func (u *window) chatComposer(gtx C, th *ui.Theme, p *chatPane, sv sessionView, working bool) D {
	name := sv.Kind
	if info, ok := u.model.providers.find(sv.Kind); ok && info.name != "" {
		name = info.name
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			ta := th.Textarea(&p.editor, i18n.T("chat.composer.placeholder", i18n.Args{"provider": name}))
			ta.MaxHeight = chatComposerMax
			return ta.Layout(gtx)
		}),
		ui.VGap(2),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.E.Layout(gtx, func(gtx C) D {
				if working {
					b := th.Button(&p.stop, i18n.T("chat.composer.stop"))
					b.Variant, b.Size, b.Icon = ui.VariantOutline, ui.SizeSM, icChatStop
					return b.Layout(gtx)
				}
				b := th.Button(&p.send, i18n.T("chat.composer.send"))
				b.Size, b.Icon = ui.SizeSM, icChatSend
				return b.Layout(gtx)
			})
		}),
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
