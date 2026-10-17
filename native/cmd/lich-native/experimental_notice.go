package main

import (
	"flag"
	"log"
	"sync/atomic"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

var flagExperimentalNotice = flag.Bool("experimental-notice", false, "open the experimental-window notice at start, for -shot")

// experimentalNoticeSeenKey is set once the notice was closed, a global
// setting: the notice greets the first native window a user opens, not every
// release that switches them to it.
const experimentalNoticeSeenKey = "native.experimentalNoticeSeen"

// experimentalNotice tells a user the native window is experimental and asks
// for an issue on anything that breaks.
type experimentalNotice struct {
	open          atomic.Bool
	dialog        ui.DialogState
	gotIt, report widget.Clickable
}

// checkExperimentalNotice opens the notice when it was never closed.
func (u *window) checkExperimentalNotice() {
	seen, err := u.client.GetSetting(u.ctx, experimentalNoticeSeenKey, "")
	if err != nil {
		log.Printf("experimental notice: %v", err)
		return
	}
	if seen == "" {
		u.beta.open.Store(true)
		u.win.Invalidate()
	}
}

func (u *window) closeExperimentalNotice() {
	u.beta.open.Store(false)
	go func() {
		u.logErr("experimental notice: record", u.client.SetSetting(u.ctx, experimentalNoticeSeenKey, "", "1"))
	}()
}

func (u *window) layoutExperimentalNotice(gtx C, th *ui.Theme) {
	n := &u.beta
	if n.gotIt.Clicked(gtx) {
		u.closeExperimentalNotice()
	}
	if n.report.Clicked(gtx) {
		u.closeExperimentalNotice()
		go func() {
			d, err := u.client.Diagnostics(u.ctx)
			if err != nil {
				log.Printf("experimental notice: diagnostics: %v", err)
				return
			}
			u.logErr("experimental notice: open bug report", u.client.OpenExternal(u.ctx, bugReportURL(d)))
		}()
	}
	on := th.On(th.Popover)
	gotIt := on.Button(&n.gotIt, i18n.T("shell.experimentalNotice.gotIt"))
	gotIt.Variant = ui.VariantGhost
	d := th.Dialog(&n.dialog, n.open.Load(), i18n.T("shell.experimentalNotice.title"), i18n.T("shell.experimentalNotice.description"))
	d.MaxWidth = optInWidth
	d.Actions = []layout.Widget{gotIt.Layout, on.Button(&n.report, i18n.T("shell.experimentalNotice.reportIssue")).Layout}
	if d.Layout(gtx) {
		u.closeExperimentalNotice()
	}
}
