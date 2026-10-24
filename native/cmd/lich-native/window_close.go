package main

import (
	"log"
	"os"
	"sync"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// closePrompt is CloseDialog: what closing the window does with the
// sessions, asked when no answer is kept. The window closing on its own is
// lich running on in the background, since the backend outlives it.
type closePrompt struct {
	mu       sync.Mutex
	running  int  // the sessions the open dialog counts, 0 while closed
	services int  // the running services it counts beside them
	leaving  bool // the window is on its way out: a close request passes

	remember widget.Bool
	dialog   ui.DialogState
	keep     widget.Clickable
	quit     widget.Clickable
}

// onClosing answers the window manager's close: let through, or held until
// the running sessions are counted (closeIsHeld).
func (u *window) onClosing(e *app.ClosingEvent) {
	if !u.closeIsHeld() {
		return
	}
	e.Abort()
	go u.askOrQuit(u.settings.look.prefs.closeAction)
}

// closeIsHeld reports whether a close waits on lich counting its sessions,
// which Ask and Quit do. Keep running lets it through, and so does a window
// lich dropped: nobody is there to count, and holding
// the close kept the window open for good, Windows' installer included.
func (u *window) closeIsHeld() bool {
	c := &u.closer
	c.mu.Lock()
	leaving := c.leaving
	c.mu.Unlock()
	return !leaving && !u.model.connectionLost() && u.settings.look.prefs.closeAction != closeBackground
}

// askOrQuit is CloseDialog's close request: quit outright with nothing
// running or quit kept as the answer, ask otherwise. A count that fails
// leaves the window open rather than guess.
func (u *window) askOrQuit(action string) {
	live, err := u.client.LiveCount(u.ctx)
	if err != nil {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("shell.windowClose.countFailed", i18n.Args{"error": backend.ErrorText(err)})})
		return
	}
	services := u.model.services.running()
	if closeQuits(action, live+services) {
		u.quitLich()
		return
	}
	c := &u.closer
	c.mu.Lock()
	c.running, c.services = live, services
	c.mu.Unlock()
	c.remember.Value = false
	u.win.Invalidate()
}

func (u *window) quitLich() {
	if err := u.client.Quit(u.ctx); err != nil {
		u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("shell.windowClose.quitFailed", i18n.Args{"error": backend.ErrorText(err)})})
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
	c.leaving, c.running, c.services = true, 0, 0
	c.mu.Unlock()
	log.Print("closing the window")
	u.win.Perform(system.ActionClose)
	os.Exit(0)
}

// layoutClosePrompt draws CloseDialog while a close is held.
func (u *window) layoutClosePrompt(gtx C, th *ui.Theme) {
	c := &u.closer
	c.mu.Lock()
	sessions, services := c.running, c.services
	c.mu.Unlock()
	running := sessions + services
	if running == 0 && !c.dialog.Animating(gtx) {
		return
	}
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
	quit := on.Button(&c.quit, i18n.T("shell.closeDialog.quit"))
	quit.Variant = ui.VariantGhost
	keep := on.Button(&c.keep, i18n.T("shell.closeDialog.keep"))
	d := th.Dialog(&c.dialog, running > 0, i18n.T("shell.closeDialog.title"), closeRunning(sessions, services))
	d.MaxWidth = ui.DialogWidthXL
	d.Body = func(gtx C) D { return closeRemember(gtx, on, &c.remember) }
	d.Actions = []layout.Widget{quit.Layout, keep.Layout}
	if d.Layout(gtx) {
		c.mu.Lock()
		c.running, c.services = 0, 0
		c.mu.Unlock()
	}
}

// closeRunning is the dialog's sentence for what quitting would end: the
// sessions, the services, or both.
func closeRunning(sessions, services int) string {
	switch {
	case services == 0:
		return i18n.T("shell.closeDialog.running", i18n.Args{"count": sessions})
	case sessions == 0:
		return i18n.T("shell.closeDialog.services", i18n.Args{"count": services})
	}
	return i18n.T("shell.closeDialog.running", i18n.Args{"count": sessions}) + " " +
		i18n.T("shell.closeDialog.servicesToo", i18n.Args{"count": services})
}

// closeRemember is the checkbox and its label, the hint of where the
// answer can be changed muted after it.
func closeRemember(gtx C, th *ui.Theme, remember *widget.Bool) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(th.Checkbox(remember).Layout),
		ui.Gap(2),
		layout.Rigid(th.Text(ui.TextSM, i18n.T("shell.closeDialog.dontAsk")).Layout),
		ui.Gap(1),
		layout.Rigid(th.Text(ui.TextSM, i18n.T("shell.windowClose.changeInSettings")).Muted().Layout),
	)
}
