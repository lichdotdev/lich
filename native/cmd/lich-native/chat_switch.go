package main

import (
	"context"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icChatGUI = icons.Lucide("sparkles")
	icChatTUI = icons.Lucide("square-terminal")
	// chatSwitchItem is the selector's h-7 items, a size under the footer's
	// balloons: it covers the terminal's corner, not a strip of its own.
	chatSwitchItem = ui.Space(7)
	chatSwitchPadX = ui.Space(2)
)

// chatSwitch draws session sv's GUI | TUI selector floating in the top-right
// corner of gtx's area, over its terminal or its chat, for a provider that
// can be drawn as a chat. Its click is read before it is drawn, so the frame
// that took it already shows the switch on its way.
func (u *window) chatSwitch(gtx C, sv sessionView) {
	c := &u.model.chat
	shown, block := chatSwitchState(c.offers(sv.Kind), sv, c.isRunning(sv.ID), c.isSwitching(sv.ID))
	if !shown {
		return
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
	th := u.theme.On(u.theme.Sidebar)
	off := block != switchFree
	group := th.ToggleGroup(&s.view,
		ui.ToggleItem{Value: api.ViewChat, Text: i18n.T("chat.switch.gui"), Icon: icChatGUI, Disabled: off},
		ui.ToggleItem{Value: api.ViewTerminal, Text: i18n.T("chat.switch.tui"), Icon: icChatTUI, Disabled: off})
	group.Size, group.ItemHeight, group.ItemPadX, group.ItemText = ui.ToggleSizeSM, chatSwitchItem, chatSwitchPadX, ui.TextXS
	pill := func(gtx C) D {
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, u.theme.Sidebar, ui.RadiusMD) },
			group.Layout)
	}
	gtx.Constraints.Min = gtx.Constraints.Max
	layout.Inset{Top: balloonInset, Right: balloonInset}.Layout(gtx, func(gtx C) D {
		return layout.NE.Layout(gtx, func(gtx C) D {
			if !off {
				return pill(gtx)
			}
			reason := i18n.T("chat.switch.working")
			if block == switchPending {
				reason = i18n.T("chat.switch.pending")
			}
			return th.Tooltip(&u.root, &s.switchTip, reason).Layout(gtx, pill)
		})
	})
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
	view      widget.Enum
	switchTip ui.Tooltip
	chatView
}
