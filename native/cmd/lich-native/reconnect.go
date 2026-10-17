package main

import (
	"context"
	"log"
	"os"
	"time"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
	"github.com/lichdotdev/lich/native/ui"
)

// reconnectWait bounds waiting for a restarted lich to answer. Its successor
// is launched before it stops and binds the port the moment it is free;
// `lich native` gives a cold start 15s (internal/cli/native.go). A var so a
// test can let the wait run out without sleeping through it.
var reconnectWait = 15 * time.Second

const reconnectPoll = 250 * time.Millisecond

// backendDropped is /events ending under the open window. A backend that
// still answers gave its sockets to another window, and this one is lost. A
// backend that is gone quit or restarted: the runtime file names a successor
// once it is up, and the window moves onto it as the web page does.
func (u *window) backendDropped() {
	if u.client.Answers(u.ctx) {
		u.model.markLost()
		return
	}
	u.reconnecting.Store(true)
	u.win.Invalidate()
	defer func() {
		u.reconnecting.Store(false)
		u.win.Invalidate()
	}()
	rt, ok := u.awaitSuccessor()
	if !ok {
		u.model.markLost()
		return
	}
	u.client.Retarget(rt)
	if err := u.reattach(); err != nil {
		log.Printf("reconnect to the restarted lich: %v", err)
		u.model.markLost()
	}
}

// awaitSuccessor polls the runtime file until it names a lich that answers,
// up to reconnectWait.
func (u *window) awaitSuccessor() (lichclient.Runtime, bool) {
	path := *flagRuntime
	if path == "" {
		var err error
		if path, err = lichclient.RuntimePath(os.Getenv); err != nil {
			log.Printf("reconnect: %v", err)
			return lichclient.Runtime{}, false
		}
	}
	deadline := time.Now().Add(reconnectWait)
	for time.Now().Before(deadline) {
		if rt, err := lichclient.ReadRuntime(path); err == nil && lichclient.New(rt).Answers(u.ctx) {
			return rt, true
		}
		select {
		case <-u.ctx.Done():
			return lichclient.Runtime{}, false
		case <-time.After(reconnectPoll):
		}
	}
	return lichclient.Runtime{}, false
}

// reattach is connect's sockets and state again, on the backend the client
// now points at: every terminal attaches again, the spawn gate asks about the
// sessions the restart stopped, and the asks a new release brings run again.
// The sockets belong to a context of their own, ended when reattach fails: the
// window is lost then, and a socket left open would hold the backend's one
// /events client for a window that shows nothing.
func (u *window) reattach() (err error) {
	ctx, cancel := context.WithCancel(u.ctx)
	defer func() {
		if err != nil {
			cancel()
		}
	}()
	events, err := u.client.Events(ctx)
	if err != nil {
		return err
	}
	stream, _, err := u.client.Terminal(ctx, func(id string, data []byte) { u.terms.onOutput(id, data) })
	if err != nil {
		return err
	}
	u.terms.reattach(stream)
	u.gate.forgetAll()
	if err := u.model.reload(u.ctx); err != nil {
		return err
	}
	if err := u.model.hydrateTurnStates(u.ctx); err != nil {
		return err
	}
	u.toaster.Dismiss(appUpdateToastID)
	go u.model.follow(u.ctx, events)
	go u.settings.startupPatchNotes()
	go u.checkPlugin()
	return nil
}

// reconnectingNotice stands in for the window while it waits for the
// restarted lich.
func (u *window) reconnectingNotice(gtx C) D {
	gtx.Constraints.Min = gtx.Constraints.Max
	return layout.Center.Layout(gtx, u.theme.Text(ui.TextSM, i18n.T("shell.reconnect.reconnecting")).Weight(font.Medium).Layout)
}
