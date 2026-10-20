package main

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework UserNotifications
#include <stdbool.h>
#include <stdlib.h>

bool lichNotesStart(const char *open, const char *dismiss);
void lichNotesShow(const char *id, const char *title, const char *body);
void lichNotesWithdraw(const char *id);
*/
import "C"

import (
	"context"
	"log"
	"sync"
	"unsafe"

	"github.com/lichdotdev/lich/native/i18n"
)

// Action keys lichNoteAnswered hands back (desktop_notes_darwin.m); any
// other is the note gone without either.
const (
	macNoteOpen    = "open"
	macNoteDismiss = "dismiss"
)

// macNotes posts through UNUserNotificationCenter, which answers only a
// process inside an app bundle: lich-native in Lich.app/Contents/MacOS. The
// session id is each note's identifier, so a newer one replaces it.
type macNotes struct {
	open func(projectID, sessionID, token string)
	gone func(sessionID string)

	mu    sync.Mutex
	notes map[string]desktopNote
}

// macNotesSink is the one macNotes the Objective-C delegate answers to.
var macNotesSink *macNotes

// newDesktopNotes is the Notification Center where lich-native runs from
// Lich.app, else fallback.
func newDesktopNotes(_ context.Context, fallback noteSink, open func(projectID, sessionID, token string), gone func(string)) noteSink {
	m := &macNotes{open: open, gone: gone, notes: map[string]desktopNote{}}
	macNotesSink = m
	openLabel, dismissLabel := C.CString(i18n.T("shell.projectEvents.open")), C.CString(i18n.T("shell.uncleanExitGate.dismiss"))
	defer C.free(unsafe.Pointer(openLabel))
	defer C.free(unsafe.Pointer(dismissLabel))
	if !C.lichNotesStart(openLabel, dismissLabel) {
		log.Printf("notify: not inside Lich.app, using the backend")
		macNotesSink = nil
		return fallback
	}
	return m
}

func (m *macNotes) show(n desktopNote) {
	m.mu.Lock()
	m.notes[n.sessionID] = n
	m.mu.Unlock()
	id, title, body := C.CString(n.sessionID), C.CString(n.title), C.CString(joinNonEmpty("\n", n.body, n.meta))
	defer C.free(unsafe.Pointer(id))
	defer C.free(unsafe.Pointer(title))
	defer C.free(unsafe.Pointer(body))
	C.lichNotesShow(id, title, body)
}

func (m *macNotes) withdraw(sessionID string) {
	id := C.CString(sessionID)
	defer C.free(unsafe.Pointer(id))
	C.lichNotesWithdraw(id)
}

func (m *macNotes) answered(sessionID, action string) {
	m.mu.Lock()
	n, ok := m.notes[sessionID]
	delete(m.notes, sessionID)
	m.mu.Unlock()
	if !ok {
		return
	}
	switch action {
	case macNoteOpen:
		m.open(n.projectID, n.sessionID, "")
		m.withdraw(sessionID)
	case macNoteDismiss:
		m.withdraw(sessionID)
	}
	m.gone(sessionID)
}

// lichNoteAnswered runs on the main thread, the one Window.Run waits on, so
// the answer goes off it before it can raise the window.
//
//export lichNoteAnswered
func lichNoteAnswered(id, action *C.char) {
	if m := macNotesSink; m != nil {
		go m.answered(C.GoString(id), C.GoString(action))
	}
}
