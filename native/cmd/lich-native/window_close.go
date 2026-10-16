package main

import (
	"fmt"
	"log"
	"os"
	"sync"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui"
)

// ponytail: English until the native window has i18n (shell.closeDialog).
const (
	closeDialogTitle  = "Keep lich running?"
	closeRunningOne   = "%d session is running. In the background it keeps working, and opening lich again brings it back on screen."
	closeRunningOther = "%d sessions are running. In the background they keep working, and opening lich again brings them back on screen."
	closeDontAsk      = "Don't ask again"
	closeInSettings   = "· change it in Settings › General"
	closeKeepLabel    = "Keep running"
	closeQuitLabel    = "Quit lich"
	closeFailed       = "Could not quit lich: %v"
	closeCountFailed  = "Could not count the running sessions: %v"
)

// closePrompt is CloseDialog: what closing the window does with the
// sessions, asked when no answer is kept. The window closing on its own is
// lich running on in the background, since the backend outlives it.
type closePrompt struct {
	mu      sync.Mutex
	running int  // the sessions the open dialog counts, 0 while closed
	leaving bool // the window is on its way out: a close request passes

	remember widget.Bool
	dialog   ui.DialogState
	keep     widget.Clickable
	quit     widget.Clickable
}

// onClosing answers the window manager's close. Keep running lets it
// through; Ask and Quit hold it until the running sessions are counted.
func (u *window) onClosing(e *app.ClosingEvent) {
	c := &u.closer
	c.mu.Lock()
	leaving := c.leaving
	c.mu.Unlock()
	action := u.settings.look.prefs.closeAction
	if leaving || action == closeBackground {
		return
	}
	e.Abort()
	go u.askOrQuit(action)
}

// askOrQuit is CloseDialog's close request: quit outright with nothing
// running or quit kept as the answer, ask otherwise. A count that fails
// leaves the window open rather than guess.
func (u *window) askOrQuit(action string) {
	live, err := u.client.LiveCount(u.ctx)
	if err != nil {
		u.toaster.Show(failure(closeCountFailed, err))
		return
	}
	if closeQuits(action, live) {
		u.quitLich()
		return
	}
	c := &u.closer
	c.mu.Lock()
	c.running = live
	c.mu.Unlock()
	c.remember.Value = false
	u.win.Invalidate()
}

func (u *window) quitLich() {
	if err := u.client.Quit(u.ctx); err != nil {
		u.toaster.Show(failure(closeFailed, err))
		return
	}
	u.leave()
}

// leave closes the window for good, past onClosing, and ends the process.
// It exits rather than wait for the DestroyEvent: on macOS Gio sends that
// from the view's dealloc, which AppKit runs when its event loop next wakes,
// so a window closed while nothing touches the machine stayed alive.
func (u *window) leave() {
	c := &u.closer
	c.mu.Lock()
	c.leaving, c.running = true, 0
	c.mu.Unlock()
	log.Print("closing the window")
	u.win.Perform(system.ActionClose)
	os.Exit(0)
}

// layoutClosePrompt draws CloseDialog while a close is held.
func (u *window) layoutClosePrompt(gtx C, th *ui.Theme) {
	c := &u.closer
	c.mu.Lock()
	running := c.running
	c.mu.Unlock()
	answer := func(choice string, act func()) {
		if c.remember.Value {
			p := u.settings.look.prefs
			p.closeAction = choice
			u.settings.commitAppearance(p, closeActionKey, choice)
		}
		go act()
	}
	if c.keep.Clicked(gtx) && running > 0 {
		answer(closeBackground, u.leave)
	}
	if c.quit.Clicked(gtx) && running > 0 {
		answer(closeQuit, u.quitLich)
	}
	on := th.On(th.Popover)
	quit := on.Button(&c.quit, closeQuitLabel)
	quit.Variant = ui.VariantGhost
	keep := on.Button(&c.keep, closeKeepLabel)
	d := th.Dialog(&c.dialog, running > 0, closeDialogTitle, closeRunning(running))
	d.MaxWidth = ui.DialogWidthXL
	d.Body = func(gtx C) D { return closeRemember(gtx, on, &c.remember) }
	d.Actions = []layout.Widget{quit.Layout, keep.Layout}
	if d.Layout(gtx) {
		c.mu.Lock()
		c.running = 0
		c.mu.Unlock()
	}
}

func closeRunning(n int) string {
	return fmt.Sprintf(pluralFormat(n, closeRunningOne, closeRunningOther), n)
}

// closeRemember is the checkbox and its label, the hint of where the
// answer can be changed muted after it.
func closeRemember(gtx C, th *ui.Theme, remember *widget.Bool) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(th.Checkbox(remember).Layout),
		ui.Gap(2),
		layout.Rigid(th.Text(ui.TextSM, closeDontAsk).Layout),
		ui.Gap(1),
		layout.Rigid(th.Text(ui.TextSM, closeInSettings).Muted().Layout),
	)
}
