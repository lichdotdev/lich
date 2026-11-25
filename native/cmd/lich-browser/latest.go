package main

// latest is a drawing sent to a page one at a time: a drawing asked for
// while one is still running in the page waits, and a newer one takes its
// place. Sent at the window's frame rate to a page busy with its own
// scripts, every frame's drawing queued up in the page and the agent's
// cursor played seconds behind the window.
type latest struct {
	busy, queued bool
	next         string
}

// offer is whether script goes to the page now; else it waits for done.
func (l *latest) offer(script string) bool {
	if l.busy {
		l.next, l.queued = script, true
		return false
	}
	l.busy = true
	return true
}

// done is the drawing to send once the page ran the last one, false when none
// waits.
func (l *latest) done() (string, bool) {
	if !l.queued {
		l.busy = false
		return "", false
	}
	script := l.next
	l.next, l.queued = "", false
	return script, true
}
