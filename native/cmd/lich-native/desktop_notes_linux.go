package main

import (
	"context"
	"html"
	"log"
	"slices"
	"sync"

	"github.com/godbus/dbus/v5"

	"github.com/lichdotdev/lich/native/i18n"
)

// The freedesktop notification server (Desktop Notifications Specification
// 1.2): GNOME Shell, KDE Plasma, mako, dunst and swaync all answer it.
const (
	notesService     = "org.freedesktop.Notifications"
	notesPath        = "/org/freedesktop/Notifications"
	notesIface       = "org.freedesktop.Notifications"
	notesApp         = "lich"
	notesSignalQueue = 16
	// notesJobQueue holds calls waiting their turn: shows and withdrawals go
	// out in the order they were asked, off the caller's goroutine.
	notesJobQueue = 32
)

// Action keys. "default" is the click on the bubble itself; servers that draw
// buttons (GNOME, KDE) show the others, mako lists them in `makoctl menu`.
const (
	noteActionDefault = "default"
	noteActionOpen    = "open"
	noteActionDismiss = "dismiss"
)

// Expiry in milliseconds as the spec reads it: a waiting session's note stays
// up until lich takes it down, a finished turn's follows the server's default.
const (
	noteNeverExpires  = int32(0)
	noteServerExpires = int32(-1)
	noteUrgencyNormal = byte(1)
)

// dbusNotes talks to the desktop's notification server on a private session
// bus connection, so its signal matches never reach another reader's.
type dbusNotes struct {
	conn   *dbus.Conn
	markup bool
	// open puts a session on screen and brings the window up with token,
	// the activation token the server handed over ("" when it sent none).
	open func(projectID, sessionID, token string)
	// gone is told the server took a session's note down on its own.
	gone func(sessionID string)
	jobs chan func()

	mu     sync.Mutex
	ids    map[string]uint32 // session → notification id
	notes  map[uint32]desktopNote
	tokens map[uint32]string
}

// newDesktopNotes is the notification server where there is one, else
// fallback.
func newDesktopNotes(ctx context.Context, fallback noteSink, open func(projectID, sessionID, token string), gone func(string)) noteSink {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		log.Printf("notify: no session bus, using the backend: %v", err)
		return fallback
	}
	var caps []string
	if err := conn.Object(notesService, notesPath).Call(notesIface+".GetCapabilities", 0).Store(&caps); err != nil {
		log.Printf("notify: no notification server, using the backend: %v", err)
		conn.Close()
		return fallback
	}
	d := &dbusNotes{conn: conn, markup: slices.Contains(caps, "body-markup"), open: open, gone: gone,
		ids: map[string]uint32{}, notes: map[uint32]desktopNote{}, tokens: map[uint32]string{},
		jobs: make(chan func(), notesJobQueue)}
	go d.follow(ctx)
	go d.work(ctx)
	return d
}

func (d *dbusNotes) work(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-d.jobs:
			job()
		}
	}
}

func (d *dbusNotes) show(n desktopNote) { d.jobs <- func() { d.notify(n) } }

func (d *dbusNotes) withdraw(sessionID string) { d.jobs <- func() { d.close(sessionID) } }

func (d *dbusNotes) notify(n desktopNote) {
	d.mu.Lock()
	replaces := d.ids[n.sessionID]
	d.mu.Unlock()
	expire := noteServerExpires
	if n.status == statusWaiting {
		expire = noteNeverExpires
	}
	open, dismiss := i18n.T("shell.projectEvents.open"), i18n.T("shell.uncleanExitGate.dismiss")
	actions := []string{noteActionDefault, open, noteActionOpen, open, noteActionDismiss, dismiss}
	var id uint32
	err := d.conn.Object(notesService, notesPath).Call(notesIface+".Notify", 0,
		notesApp, replaces, noteAppIcon(n), n.title, d.body(n), actions, d.hints(), expire).Store(&id)
	if err != nil {
		log.Printf("notify: %v", err)
		return
	}
	d.mu.Lock()
	if replaces != 0 && replaces != id {
		delete(d.notes, replaces)
		delete(d.tokens, replaces)
	}
	d.ids[n.sessionID], d.notes[id] = id, n
	d.mu.Unlock()
}

func (d *dbusNotes) close(sessionID string) {
	d.mu.Lock()
	id, ok := d.ids[sessionID]
	d.mu.Unlock()
	if !ok {
		return
	}
	if err := d.conn.Object(notesService, notesPath).Call(notesIface+".CloseNotification", 0, id).Err; err != nil {
		log.Printf("notify: close %s: %v", sessionID, err)
	}
}

// body is what the session asks or said, then project and branch, escaped
// where the server reads markup.
func (d *dbusNotes) body(n desktopNote) string {
	text := joinNonEmpty("\n", n.body, n.meta)
	if d.markup {
		return html.EscapeString(text)
	}
	return text
}

func (d *dbusNotes) hints() map[string]dbus.Variant {
	return map[string]dbus.Variant{"urgency": dbus.MakeVariant(noteUrgencyNormal)}
}

// noteAppIcon is the note's icon file, "" (the server's own) when it could
// not be drawn.
func noteAppIcon(n desktopNote) string {
	path, err := noteIconFile(n.kind, n.status)
	if err != nil {
		log.Printf("notify: icon: %v", err)
		return ""
	}
	return path
}

func (d *dbusNotes) follow(ctx context.Context) {
	if err := d.conn.AddMatchSignal(dbus.WithMatchInterface(notesIface)); err != nil {
		log.Printf("notify: can't follow clicks: %v", err)
		return
	}
	signals := make(chan *dbus.Signal, notesSignalQueue)
	d.conn.Signal(signals)
	for {
		select {
		case <-ctx.Done():
			return
		case s, ok := <-signals:
			if !ok {
				return
			}
			d.signal(s)
		}
	}
}

func (d *dbusNotes) signal(s *dbus.Signal) {
	if len(s.Body) < 2 {
		return
	}
	id, ok := s.Body[0].(uint32)
	if !ok {
		return
	}
	switch s.Name {
	case notesIface + ".ActivationToken":
		if token, ok := s.Body[1].(string); ok {
			d.mu.Lock()
			d.tokens[id] = token
			d.mu.Unlock()
		}
	case notesIface + ".ActionInvoked":
		key, _ := s.Body[1].(string)
		d.invoked(id, key)
	case notesIface + ".NotificationClosed":
		d.closed(id)
	}
}

// invoked runs a click. The spec sends ActivationToken just before
// ActionInvoked, so the token is already here.
func (d *dbusNotes) invoked(id uint32, key string) {
	d.mu.Lock()
	n, ok := d.notes[id]
	token := d.tokens[id]
	d.mu.Unlock()
	if !ok {
		return
	}
	switch key {
	case noteActionDefault, noteActionOpen:
		d.open(n.projectID, n.sessionID, token)
		d.withdraw(n.sessionID)
	case noteActionDismiss:
		d.withdraw(n.sessionID)
	}
}

func (d *dbusNotes) closed(id uint32) {
	d.mu.Lock()
	n, ok := d.notes[id]
	if ok {
		delete(d.notes, id)
		delete(d.tokens, id)
		if d.ids[n.sessionID] == id {
			delete(d.ids, n.sessionID)
		}
	}
	d.mu.Unlock()
	if ok {
		d.gone(n.sessionID)
	}
}
