package main

import (
	"context"
	"log"
	"strconv"
	"sync"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
)

// The web keeps these two in localStorage (providers/settings.tsx); the
// native window keeps them in the backend under the same names.
const (
	desktopNotifyKey  = "lich.notifications.desktop"
	finishedNotifyKey = "lich.notifications.finishedTurn"
)

// desktopNotice is what the desktop channel does with a status report
// (session-events.ts DesktopNotice).
type desktopNotice uint8

const (
	noticeNone desktopNotice = iota
	noticeAsk
	noticeNotify
)

// optIn is the needs-input preference: unset until the opt-in is answered.
type optIn uint8

const (
	optInUnset optIn = iota
	optInNo
	optInYes
)

// notifyPrefs are session-events.ts DesktopNotifyPrefs.
type notifyPrefs struct {
	attention    optIn
	finishedTurn bool
}

// decideDesktopNotice is session-events.ts decideDesktopNotice: nothing while
// the window is in front, the opt-in's question while it is unanswered.
func decideDesktopNotice(focused bool, enabled optIn) desktopNotice {
	switch {
	case focused:
		return noticeNone
	case enabled == optInUnset:
		return noticeAsk
	case enabled == optInYes:
		return noticeNotify
	}
	return noticeNone
}

// decideStatusNotice is session-events.ts decideStatusNotice: every
// "waiting" report notifies, a second prompt being a second question; a
// "done" repeated with no run between, or closing a /compact, does not.
func decideStatusNotice(status, previous string, focused bool, prefs notifyPrefs) desktopNotice {
	switch {
	case status == statusWaiting:
		return decideDesktopNotice(focused, prefs.attention)
	case status == statusDone && previous != statusDone && previous != statusCompacting:
		if !prefs.finishedTurn {
			return noticeNone
		}
		return decideDesktopNotice(focused, optInYes)
	}
	return noticeNone
}

// statusReport is one session-status report as the desktop channel and the
// attention toast read it.
type statusReport struct {
	id, projectID    string
	status, previous string
	focused          bool
	// onScreen is the session being the one the window shows.
	onScreen       bool
	label, project string
	// reason is what a waiting session is blocked on, "" when unsaid.
	reason string
	// kind is the session's provider, branch the checkout's, "" unknown.
	kind, branch string
}

// notifier owns the desktop channel's preferences and the opt-in's open
// state; reports arrive from the /events goroutine, the switches from the
// frame's.
type notifier struct {
	mu     sync.Mutex
	prefs  notifyPrefs
	asking bool
	sink   noteSink
	// shown is the status each session's notification on screen says.
	shown map[string]string
	// store keeps one preference in the backend.
	store      func(key, value string)
	invalidate func()
}

// report raises what r calls for: the notification, or the opt-in's question.
// A notification that no longer says what the session is doing, or whose
// session is the one on screen in a focused window, comes down first.
func (n *notifier) report(r statusReport) {
	n.mu.Lock()
	notice := decideStatusNotice(r.status, r.previous, r.focused, n.prefs)
	if notice == noticeAsk {
		n.asking = true
	}
	shown, up := n.shown[r.id]
	stale := up && (shown != r.status || (r.focused && r.onScreen))
	if stale {
		delete(n.shown, r.id)
	}
	if notice == noticeNotify {
		if n.shown == nil {
			n.shown = map[string]string{}
		}
		n.shown[r.id] = r.status
	}
	n.mu.Unlock()
	if stale && notice != noticeNotify {
		n.sink.withdraw(r.id)
	}
	switch notice {
	case noticeAsk:
		n.invalidate()
	case noticeNotify:
		n.sink.show(noteFor(r, notifySummary(r.status, r.label)))
	}
}

// seen takes down id's notification: the window just showed the session.
func (n *notifier) seen(id string) {
	n.mu.Lock()
	_, up := n.shown[id]
	delete(n.shown, id)
	n.mu.Unlock()
	if up {
		n.sink.withdraw(id)
	}
}

// closed forgets id's notification, which the desktop took down itself.
func (n *notifier) closed(id string) {
	n.mu.Lock()
	delete(n.shown, id)
	n.mu.Unlock()
}

func notifySummary(status, label string) string {
	if status == statusWaiting {
		return i18n.T("shell.projectEvents.needsInput", i18n.Args{"label": label})
	}
	return i18n.T("shell.projectEvents.finished", i18n.Args{"label": label})
}

func (n *notifier) snapshot() (notifyPrefs, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.prefs, n.asking
}

// setAttention is the needs-input switch, and an answer to the opt-in,
// which closes it: the question is asked once.
func (n *notifier) setAttention(on bool) {
	n.mu.Lock()
	n.prefs.attention, n.asking = optInNo, false
	if on {
		n.prefs.attention = optInYes
	}
	n.mu.Unlock()
	n.store(desktopNotifyKey, strconv.FormatBool(on))
}

func (n *notifier) setFinishedTurn(on bool) {
	n.mu.Lock()
	n.prefs.finishedTurn = on
	n.mu.Unlock()
	n.store(finishedNotifyKey, strconv.FormatBool(on))
}

// dismiss closes the opt-in unanswered, so the next report asks again.
func (n *notifier) dismiss() {
	n.mu.Lock()
	n.asking = false
	n.mu.Unlock()
}

// parseNotifyPrefs is readDesktopNotifications and
// readFinishedTurnNotifications: only what writePref emits is an answer.
func parseNotifyPrefs(attention, finished string) notifyPrefs {
	p := notifyPrefs{finishedTurn: finished == "true"}
	switch attention {
	case "true":
		p.attention = optInYes
	case "false":
		p.attention = optInNo
	}
	return p
}

func loadNotifyPrefs(ctx context.Context, c *lichclient.Client) (notifyPrefs, error) {
	attention, err := c.GetSetting(ctx, desktopNotifyKey, "")
	if err != nil {
		return notifyPrefs{}, err
	}
	finished, err := c.GetSetting(ctx, finishedNotifyKey, "")
	return parseNotifyPrefs(attention, finished), err
}

// newNotifier is the window's desktop channel; sink raises what it decides.
func newNotifier(ctx context.Context, c *lichclient.Client, sink noteSink, invalidate func()) *notifier {
	return &notifier{
		invalidate: invalidate,
		sink:       sink,
		store: func(key, value string) {
			go func() {
				if err := c.SetSetting(ctx, key, "", value); err != nil {
					log.Printf("settings: save %s: %v", key, err)
				}
			}()
		},
	}
}
