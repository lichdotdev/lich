package main

import (
	"flag"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

var flagNotifyOptIn = flag.Bool("notify-opt-in", false, "open the desktop-notification opt-in at start, for -shot")

// optInWidth is the dialog's sm:max-w-lg.
const optInWidth unit.Dp = 512

// notifyOptIn is NotificationsOptIn's dialog state.
type notifyOptIn struct {
	dialog ui.DialogState
	no     widget.Clickable
	yes    widget.Clickable
}

// layoutNotifyOptIn draws NotificationsOptIn while the notifier asks: either
// button is an answer, stored; Escape or the backdrop closes it unanswered.
func (u *window) layoutNotifyOptIn(gtx C, th *ui.Theme) {
	o := &u.optIn
	if o.no.Clicked(gtx) {
		u.notifier.setAttention(false)
	}
	if o.yes.Clicked(gtx) {
		u.notifier.setAttention(true)
	}
	_, asking := u.notifier.snapshot()
	on := th.On(th.Popover)
	no := on.Button(&o.no, i18n.T("shell.notificationsOptIn.noThanks"))
	no.Variant = ui.VariantGhost
	d := th.Dialog(&o.dialog, asking, i18n.T("shell.notificationsOptIn.title"), i18n.T("shell.notifyPrompt.description"))
	d.MaxWidth = optInWidth
	d.Actions = []layout.Widget{no.Layout, on.Button(&o.yes, i18n.T("shell.notificationsOptIn.notifyMe")).Layout}
	if d.Layout(gtx) {
		u.notifier.dismiss()
	}
}
