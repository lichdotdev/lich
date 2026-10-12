package main

import (
	"flag"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui"
)

// ponytail: English until the native window has i18n (shell.notificationsOptIn).
const (
	optInTitle    = "Notify you when a session needs you?"
	optInDesc     = "A session just asked for your input while you were somewhere else. lich can raise a desktop notification when that happens, naming the session and its project, so you can start something and walk away from the window. Sessions you are looking at never notify. You can change this any time in Settings › General."
	optInNoThanks = "No thanks"
	optInNotifyMe = "Notify me"
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
	no := on.Button(&o.no, optInNoThanks)
	no.Variant = ui.VariantGhost
	d := th.Dialog(&o.dialog, asking, optInTitle, optInDesc)
	d.MaxWidth = optInWidth
	d.Actions = []layout.Widget{no.Layout, on.Button(&o.yes, optInNotifyMe).Layout}
	if d.Layout(gtx) {
		u.notifier.dismiss()
	}
}
