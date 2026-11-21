package main

import (
	"context"
	"image"
	"sync"

	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icChatGUI = icons.Lucide("sparkles")
	icChatTUI = icons.Lucide("square-terminal")
	// The selector's items are h-5 with a Text2XS label and a size-3 icon, a
	// step under the footer's h-8: it sits in a strip or a pane header, where
	// the room is short.
	chatSwitchItem = ui.Space(5)
	chatSwitchPadX = ui.Space(1.5)
	chatSwitchIcon = ui.Space(3)
	chatSwitchBox  = ui.ToggleGroupHeight(chatSwitchItem)
)

// chatSwitch draws session sv's GUI | TUI selector, on th's ground, for a
// provider that can be drawn as a chat, and nothing for another. Its click is
// read before it is drawn, so the frame that took it already shows the
// switch on its way.
func (u *window) chatSwitch(gtx C, th *ui.Theme, sv sessionView) D {
	c := &u.model.chat
	shown, block := chatSwitchState(c.offers(sv.Kind), sv, c.isRunning(sv.ID), c.isSwitching(sv.ID))
	if !shown {
		return D{}
	}
	s := u.chats.pane(sv.ID)
	current := chatViewOf(sv)
	s.view.Value = current
	if to := c.switchingTo(sv.ID); to != "" {
		s.view.Value = to
	}
	if block == switchFree && s.view.Update(gtx) && s.view.Value != current {
		u.switchView(sv.ID, s.view.Value)
		block = switchPending
	}
	off := block != switchFree
	group := th.ToggleGroup(&s.view,
		ui.ToggleItem{Value: api.ViewChat, Text: i18n.T("chat.switch.gui"), Icon: icChatGUI, Disabled: off},
		ui.ToggleItem{Value: api.ViewTerminal, Text: i18n.T("chat.switch.tui"), Icon: icChatTUI, Disabled: off})
	group.Size, group.ItemHeight, group.ItemPadX, group.ItemText, group.ItemIcon = ui.ToggleSizeSM, chatSwitchItem, chatSwitchPadX, ui.Text2XS, chatSwitchIcon
	// The selector sits on the backdrop's dots; a ground of its own keeps
	// them from showing through it.
	grounded := func(gtx C) D {
		m := op.Record(gtx.Ops)
		d := group.Layout(gtx)
		call := m.Stop()
		paint.FillShape(gtx.Ops, th.Surface, clip.UniformRRect(image.Rectangle{Max: d.Size}, gtx.Dp(ui.RadiusMD)).Op(gtx.Ops))
		call.Add(gtx.Ops)
		return d
	}
	if !off {
		return grounded(gtx)
	}
	reason := i18n.T("chat.switch.working")
	if block == switchPending {
		reason = i18n.T("chat.switch.pending")
	}
	return th.Tooltip(&u.root, &s.switchTip, reason).Layout(gtx, grounded)
}

// chatViewOf is the view sv is drawn in, "" read as the terminal.
func chatViewOf(sv sessionView) string {
	if sv.View == api.ViewChat {
		return api.ViewChat
	}
	return api.ViewTerminal
}

// switchView asks the backend for session id in view; the selector waits
// for SessionViewEvent, or for the refusal, which says why.
func (u *window) switchView(id, view string) {
	if !u.model.chat.beginSwitch(id, view) {
		return
	}
	go func() {
		err := u.client.ChatSetView(u.ctx, id, view)
		if err == nil {
			return
		}
		u.model.chat.switched(id)
		// The terminal may have ended on the way; its toast was held for the switch.
		u.raiseExit(id)
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("chat.switch.failed"), Description: backend.ErrorText(err)})
		u.model.invalidate()
	}()
}

// leaveChat readies session id's terminal: its next spawn resumes the
// conversation the chat left, with no prompt.
func (u *window) leaveChat(ctx context.Context, id string) {
	sv, ok := u.model.sessionViewOf(id)
	if !ok {
		return
	}
	u.gate.forget(id)
	u.gate.leaveChat(ctx, sv)
}

// enterChat lets the terminal of session id go: its process ended for the
// chat's, and a terminal shown again starts through leaveChat.
func (u *window) enterChat(id string) {
	u.terms.drop(id)
	u.gate.forget(id)
	u.toaster.Dismiss(exitToastID(id))
	u.model.invalidate()
}

// chatScreens is each session's chat view and selector state, by id.
// shortcut: entries outlive their sessions, a few widgets each; prune on
// session close if windows with thousands of sessions show up.
type chatScreens struct {
	panes map[string]*chatPane
	// views is the view each session was last drawn in.
	views map[string]chatShown
	// typed is what a reference injected from the Code tab, the editor or a
	// pull request handed each session's composer off the frame, waiting for
	// the frame that types it.
	mu    sync.Mutex
	typed map[string]string
	// attached is what a drop, the attach "+" or an artifact handed each
	// session's composer as cards, as typed is for text.
	attached map[string][]backend.Attachment
	// thumbs are the images the cards draw, by path, read once.
	thumbs map[string]*remote[*previewImage]
}

// typeIn queues text for session id's composer, from any goroutine.
func (c *chatScreens) typeIn(id, text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.typed == nil {
		c.typed = map[string]string{}
	}
	c.typed[id] += text
}

// takeTyped is the text queued for session id's composer, now taken.
func (c *chatScreens) takeTyped(id string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	text := c.typed[id]
	delete(c.typed, id)
	return text
}

func (c *chatScreens) pane(id string) *chatPane {
	if c.panes == nil {
		c.panes = map[string]*chatPane{}
	}
	p, ok := c.panes[id]
	if !ok {
		p = newChatPane()
		c.panes[id] = p
	}
	return p
}

// chatPane is one session's chat view and selector, kept across frames.
type chatPane struct {
	view      ui.Enum
	switchTip ui.Tooltip
	chatView
}
