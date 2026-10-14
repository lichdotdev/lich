package main

import (
	"context"
	"log"
	"strings"

	"github.com/omartelo/lich/native/lichclient"
)

// desktopNote is one session's desktop notification. A session has at most
// one on screen: a newer report replaces it, and it is taken down once it no
// longer says what the session is doing or the session was seen in the window.
type desktopNote struct {
	sessionID, projectID string
	kind                 string
	status               string
	title                string
	// body is what the session asks, or what it said at the end of its turn,
	// "" when neither is known.
	body string
	// meta is "project · branch".
	meta string
}

// noteSink is where desktop notifications go: the desktop's own notification
// server where the window talks to it, the backend's system.Notify elsewhere.
type noteSink interface {
	show(n desktopNote)
	// withdraw takes sessionID's notification down, if it has one up.
	withdraw(sessionID string)
}

// noteRecapLimit bounds the turn's closing words in a body, in runes: about a
// line and a half in a notification bubble.
const noteRecapLimit = 140

func noteFor(r statusReport, title string) desktopNote {
	n := desktopNote{sessionID: r.id, projectID: r.projectID, kind: r.kind, status: r.status, title: title}
	if r.status == statusWaiting {
		n.body = r.reason
	}
	n.meta = joinNonEmpty(" · ", r.project, r.branch)
	return n
}

// recapLine is the turn's closing words folded onto one line and cut to
// noteRecapLimit.
func recapLine(said string) string {
	line := strings.Join(strings.Fields(said), " ")
	runes := []rune(line)
	if len(runes) <= noteRecapLimit {
		return line
	}
	return strings.TrimSpace(string(runes[:noteRecapLimit-1])) + "…"
}

// recapping fills a finished turn's body with what the agent said last, read
// off the frame, before next raises it.
type recapping struct {
	next noteSink
	said func(id string) (string, error)
}

func (r recapping) show(n desktopNote) {
	if n.status != statusDone || n.body != "" {
		r.next.show(n)
		return
	}
	go func() {
		said, err := r.said(n.sessionID)
		if err != nil {
			log.Printf("notify: last turn of %s: %v", n.sessionID, err)
		}
		n.body = recapLine(said)
		r.next.show(n)
	}()
}

func (r recapping) withdraw(id string) { r.next.withdraw(id) }

// backendNotes raises notifications through the backend's system.Notify, the
// RPC the web uses: no click, no replacing, nothing to take down.
type backendNotes struct {
	ctx    context.Context
	client *lichclient.Client
}

func (b backendNotes) show(n desktopNote) {
	go func() {
		if err := b.client.Notify(b.ctx, n.title, joinNonEmpty("\n", n.body, n.meta)); err != nil {
			log.Printf("notify: %v", err)
		}
	}()
}

func (backendNotes) withdraw(string) {}
