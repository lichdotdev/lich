package main

import (
	"context"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

// childFrames is s1's chat with three sessions it opened: one waiting on a
// permission, one working, one done.
func childFrames(t *testing.T) (*window, *frameRig) {
	t.Helper()
	u, r, _ := chatFrames(t, api.ViewChat)
	u.model.mu.Lock()
	u.model.projects[0].Sessions = append(u.model.projects[0].Sessions,
		backend.Session{ID: "k1", Label: "Fix #421: stale Docker docs", Kind: "omp", OriginSessionID: "s1"},
		backend.Session{ID: "k2", Label: "Fix #412: relay handshake", Kind: "codex", OriginSessionID: "s1"},
		backend.Session{ID: "k3", Label: "Fix #417: menu bar icon", Kind: "opencode", OriginSessionID: "s1"},
		backend.Session{ID: "x", Label: "Someone else's", Kind: "claude"},
	)
	u.model.mu.Unlock()
	for _, ev := range []statusEvent{{ID: "k1", State: statusDone}, {ID: "k2", State: statusBusy}, {ID: "k3", State: statusWaiting, Reason: "Permission to run: go test ./..."}} {
		if err := u.model.applyStatus(context.Background(), statusEv(t, ev)); err != nil {
			t.Fatal(err)
		}
	}
	u.model.chat.history("s1", []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "fix the three bugs, one session each"}})
	r.settle(t)
	return u, r
}

// The composer counts the sessions this one opened and how many wait on the
// person; its list puts the one waiting first, with what it waits on, and a
// row opens its session.
func TestTheComposerListsTheSessionsThisOneOpened(t *testing.T) {
	u, r := childFrames(t)
	count, waiting := i18n.T("chat.children.count", i18n.Args{"count": 3}), "· "+i18n.T("chat.children.waiting", i18n.Args{"count": 1})
	writeShot(t, r, "children-chip.png")
	if !r.shows(count) || !r.shows(waiting) {
		t.Fatalf("no chip counting the children:\n%s", r.drawn())
	}
	r.click(t, r.at(t, count))
	r.settle(t)
	writeShot(t, r, "children-open.png")
	for _, s := range []string{"Fix #417: menu bar icon", "Fix #412: relay handshake", "Fix #421: stale Docker docs", "Permission to run: go test ./..."} {
		if !r.shows(s) {
			t.Errorf("the list lacks %q:\n%s", s, r.drawn())
		}
	}
	if r.shows("Someone else's") {
		t.Errorf("the list holds a session this one did not open")
	}
	if a, b := labelTop(t, r, "Fix #417: menu bar icon"), labelTop(t, r, "Fix #412: relay handshake"); a >= b {
		t.Errorf("the child waiting at y=%d is not over the one working at y=%d", a, b)
	}
	r.click(t, r.at(t, "Fix #412: relay handshake"))
	r.settle(t)
	if sv, ok := u.model.activeSession(); !ok || sv.ID != "k2" {
		t.Errorf("active session %q, want the row's k2", sv.ID)
	}
}

// A session that opened none has no chip.
func TestASessionThatOpenedNoneHasNoChip(t *testing.T) {
	u, r, _ := chatFrames(t, api.ViewChat)
	u.model.chat.history("s1", []backend.ChatItem{{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "hi"}})
	r.settle(t)
	if r.shows(i18n.T("chat.children.count", i18n.Args{"count": 0})) {
		t.Errorf("a chip with no children:\n%s", r.drawn())
	}
}
