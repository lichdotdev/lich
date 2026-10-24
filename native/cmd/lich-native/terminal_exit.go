package main

import (
	"context"
	"fmt"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

type sessionExit = api.SessionExit

func readSessionExit(ev backend.Event) sessionExit {
	id, _ := api.TerminalExitEvents.ID(ev.Name)
	e, err := backend.Decode(ev, api.TerminalExitEvents.Of(id))
	if err != nil {
		return sessionExit{}
	}
	return e
}

// exitMarker is the scrollback line where the
// process died, which outlives the exit after a restart.
func exitMarker(e sessionExit) string {
	if e.Code == nil {
		return "\r\n[process exited]\r\n"
	}
	return fmt.Sprintf("\r\n[process exited: %d]\r\n", *e.Code)
}

// exited marks session id's terminal as holding a process that is gone. A
// session never shown here has no terminal: showing it starts a new process.
func (ts *terminals) exited(id string, e sessionExit) {
	ts.mu.Lock()
	t := ts.byID[id]
	if t != nil {
		t.exit = &e
	}
	ts.mu.Unlock()
	if t == nil {
		return
	}
	if t.kind == serviceTermKind {
		// A service's bar already says how it ended; its console only stops
		// looking like it waits for input.
		t.vt.Write([]byte(hideCursor))
	} else {
		t.vt.Write([]byte(exitMarker(e)))
	}
	ts.invalidate()
}

// exitOf is the exit session id's terminal is showing, if any.
func (ts *terminals) exitOf(id string) (sessionExit, bool) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	t := ts.byID[id]
	if t == nil || t.exit == nil {
		return sessionExit{}, false
	}
	return *t.exit, true
}

// restart starts the same kind in the same directory into the same terminal
// and its scrollback, reopening conversation resume, or a conversation of
// its own for "". It reports whether the process started.
func (ts *terminals) restart(ctx context.Context, s sessionView, resume string) bool {
	ts.mu.Lock()
	t := ts.byID[s.ID]
	ts.mu.Unlock()
	if t == nil {
		return false
	}
	cols, rows := t.vt.Size()
	if err := ts.start(ctx, s, resume, cols, rows); err != nil {
		ts.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("terminal.view.restartFailed", i18n.Args{"error": backend.ErrorText(err)})})
		return false
	}
	ts.running(s.ID)
	return true
}

// running clears the exit of session id's terminal: a process runs in its
// PTY again.
func (ts *terminals) running(id string) {
	ts.mu.Lock()
	if t := ts.byID[id]; t != nil {
		t.exit = nil
	}
	ts.mu.Unlock()
	ts.invalidate()
}
