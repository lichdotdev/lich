package main

import (
	"context"
	"slices"
	"testing"

	"github.com/omartelo/lich/native/lichclient"
)

type sentNotice struct{ summary, detail string }

// fakeNotes is the desktop: what was raised, as headline and the line under
// it, and which sessions' notes were taken down.
type fakeNotes struct {
	sent      *[]sentNotice
	withdrawn *[]string
}

func (f fakeNotes) show(n desktopNote) {
	*f.sent = append(*f.sent, sentNotice{n.title, n.meta})
}

func (f fakeNotes) withdraw(id string) { *f.withdrawn = append(*f.withdrawn, id) }

// notifyRig is a model over one open project whose status reports reach a
// notifier with a fake desktop and a fake store.
type notifyRig struct {
	m         *model
	n         *notifier
	sent      []sentNotice
	withdrawn []string
	stored    map[string]string
}

func newNotifyRig(prefs notifyPrefs) *notifyRig {
	r := &notifyRig{stored: map[string]string{}}
	r.n = &notifier{prefs: prefs, invalidate: func() {},
		sink:  fakeNotes{sent: &r.sent, withdrawn: &r.withdrawn},
		store: func(key, value string) { r.stored[key] = value }}
	r.m = newModel(nil, func() {})
	r.m.projects = []lichclient.Project{{ID: "p", Name: "lich", Sessions: []lichclient.Session{
		{ID: "a", Label: "fix the build"}, {ID: "b"},
	}}}
	r.m.notify = r.n.report
	r.m.seen = r.n.seen
	return r
}

func (r *notifyRig) report(t *testing.T, id, state string) {
	t.Helper()
	if err := r.m.applyStatus(context.Background(), statusEv(t, statusEvent{ID: id, State: state})); err != nil {
		t.Fatal(err)
	}
}

func (r *notifyRig) asking() bool {
	_, asking := r.n.snapshot()
	return asking
}

func TestNothingNotifiesWhileTheWindowIsInFront(t *testing.T) {
	r := newNotifyRig(notifyPrefs{attention: optInYes, finishedTurn: true})
	r.m.setFocused(context.Background(), true)
	r.report(t, "a", statusWaiting)
	r.report(t, "a", statusBusy)
	r.report(t, "a", statusDone)
	if len(r.sent) != 0 || r.asking() {
		t.Fatalf("focused window raised %v, asking %v", r.sent, r.asking())
	}
}

func TestEveryWaitingReportNotifiesNamingTheSessionAndProject(t *testing.T) {
	r := newNotifyRig(notifyPrefs{attention: optInYes})
	r.report(t, "a", statusWaiting)
	r.report(t, "a", statusWaiting)
	r.report(t, "b", statusWaiting)
	want := []sentNotice{
		{"fix the build needs your input", "lich"},
		{"fix the build needs your input", "lich"},
		{"A session needs your input", "lich"},
	}
	if !slices.Equal(r.sent, want) {
		t.Fatalf("sent %v, want %v", r.sent, want)
	}
}

func TestAFinishedTurnNotifiesOncePerRun(t *testing.T) {
	r := newNotifyRig(notifyPrefs{attention: optInNo, finishedTurn: true})
	for _, state := range []string{statusBusy, statusDone, statusDone, statusCompacting, statusDone, statusBusy, statusDone} {
		r.report(t, "a", state)
	}
	if len(r.sent) != 2 || r.sent[0].summary != "fix the build has finished working" {
		t.Fatalf("sent %v, want one per run, none for the repeat or the /compact", r.sent)
	}
}

func TestEachChannelAnswersToItsOwnSwitch(t *testing.T) {
	r := newNotifyRig(notifyPrefs{attention: optInYes})
	r.report(t, "a", statusDone)
	if len(r.sent) != 0 || r.asking() {
		t.Fatalf("finished turn is off by default, yet %v / asking %v", r.sent, r.asking())
	}
	r = newNotifyRig(notifyPrefs{attention: optInNo, finishedTurn: true})
	r.report(t, "a", statusWaiting)
	if len(r.sent) != 0 || r.asking() {
		t.Fatalf("needs input answered no, yet %v / asking %v", r.sent, r.asking())
	}
}

func TestUnansweredOptInAsksInsteadOfNotifying(t *testing.T) {
	r := newNotifyRig(notifyPrefs{finishedTurn: true})
	r.report(t, "a", statusDone)
	if r.asking() || len(r.sent) != 1 {
		t.Fatalf("a finished turn never asks: asking %v, sent %v", r.asking(), r.sent)
	}
	r.report(t, "a", statusWaiting)
	if !r.asking() || len(r.sent) != 1 {
		t.Fatalf("first waiting while unset: asking %v, sent %v", r.asking(), r.sent)
	}
	r.n.dismiss()
	r.report(t, "a", statusWaiting)
	if !r.asking() {
		t.Fatal("a dismissed opt-in is not asked again")
	}
	r.n.setAttention(true)
	if r.asking() || r.stored[desktopNotifyKey] != "true" {
		t.Fatalf("answer: asking %v, stored %v", r.asking(), r.stored)
	}
	r.report(t, "a", statusWaiting)
	if len(r.sent) != 2 || r.asking() {
		t.Fatalf("after yes: sent %v, asking %v", r.sent, r.asking())
	}
}

func TestReportsOnASessionNoOpenProjectHoldsAreDropped(t *testing.T) {
	r := newNotifyRig(notifyPrefs{attention: optInYes})
	r.report(t, "gone", statusWaiting)
	if len(r.sent) != 0 {
		t.Fatalf("sent %v for a session with nowhere to route", r.sent)
	}
}

func TestNotifyPrefsReadOnlyWhatTheWebWrites(t *testing.T) {
	cases := []struct {
		attention, finished string
		want                notifyPrefs
	}{
		{"", "", notifyPrefs{}},
		{"true", "true", notifyPrefs{attention: optInYes, finishedTurn: true}},
		{"false", "false", notifyPrefs{attention: optInNo}},
		{"1", "yes", notifyPrefs{}},
	}
	for _, c := range cases {
		if got := parseNotifyPrefs(c.attention, c.finished); got != c.want {
			t.Errorf("%q %q = %+v, want %+v", c.attention, c.finished, got, c.want)
		}
	}
	r := newNotifyRig(notifyPrefs{})
	r.n.setFinishedTurn(true)
	r.n.setAttention(false)
	if r.stored[finishedNotifyKey] != "true" || r.stored[desktopNotifyKey] != "false" {
		t.Fatalf("stored %v", r.stored)
	}
}

func TestNotifyPrefsLoadFromTheBackend(t *testing.T) {
	f, client := newSidebarBackend(t)
	f.settings[desktopNotifyKey+"@"], f.settings[finishedNotifyKey+"@"] = "true", "true"
	p, err := loadNotifyPrefs(context.Background(), client)
	if err != nil || p != (notifyPrefs{attention: optInYes, finishedTurn: true}) {
		t.Fatalf("%+v, %v", p, err)
	}
}

func TestANoteComesDownOnceItsSessionMovesOn(t *testing.T) {
	r := newNotifyRig(notifyPrefs{attention: optInYes, finishedTurn: true})
	r.report(t, "a", statusWaiting)
	r.report(t, "a", statusWaiting)
	if len(r.withdrawn) != 0 {
		t.Fatalf("a second prompt took the note down: %v", r.withdrawn)
	}
	r.report(t, "a", statusBusy)
	if !slices.Equal(r.withdrawn, []string{"a"}) {
		t.Fatalf("withdrawn %v after the session went back to work, want [a]", r.withdrawn)
	}
	r.report(t, "a", statusBusy)
	if len(r.withdrawn) != 1 {
		t.Fatalf("withdrawn %v, want no second take-down with nothing up", r.withdrawn)
	}
}

func TestAFinishedNoteGivesWayToTheNextPromptWithoutATakeDown(t *testing.T) {
	r := newNotifyRig(notifyPrefs{attention: optInYes, finishedTurn: true})
	r.report(t, "a", statusBusy)
	r.report(t, "a", statusDone)
	r.report(t, "a", statusWaiting)
	if len(r.withdrawn) != 0 || len(r.sent) != 2 {
		t.Fatalf("sent %v withdrawn %v, want the prompt to replace the finished note in place", r.sent, r.withdrawn)
	}
}

func TestSeeingTheSessionTakesItsNoteDown(t *testing.T) {
	r := newNotifyRig(notifyPrefs{attention: optInYes})
	r.report(t, "a", statusWaiting)
	r.n.seen("b")
	r.n.seen("a")
	r.n.seen("a")
	if !slices.Equal(r.withdrawn, []string{"a"}) {
		t.Fatalf("withdrawn %v, want [a] once", r.withdrawn)
	}
}

func TestANoteTheDesktopClosedIsNotTakenDownAgain(t *testing.T) {
	r := newNotifyRig(notifyPrefs{attention: optInYes})
	r.report(t, "a", statusWaiting)
	r.n.closed("a")
	r.report(t, "a", statusBusy)
	if len(r.withdrawn) != 0 {
		t.Fatalf("withdrawn %v, want nothing: the desktop already took it down", r.withdrawn)
	}
}
