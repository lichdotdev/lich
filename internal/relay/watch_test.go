package relay

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/store"
)

func (f fakeSessions) SetAgentSchedule(sessionID string, at int64, prompt string) error {
	return f.SetSessionSchedule(sessionID, at, prompt)
}

func (f fakeSessions) SetSessionWatch(string, api.SessionWatch) error { return nil }

// watchRow is one open session on a store of a single mutable row, so a test
// can run several reads in a row against what the last one saved.
type watchRow struct {
	fakeSessions
	mu   sync.Mutex
	sess store.Session
}

func (r *watchRow) LoadState(context.Context, api.None) ([]store.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return []store.Project{{ID: "p", Path: "/repo", Sessions: []store.Session{r.sess}}}, nil
}

func (r *watchRow) SetSessionWatch(_ string, w api.SessionWatch) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sess.Watch = w
	return nil
}

func (r *watchRow) SetSessionSchedule(_ string, at int64, prompt string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sess.ScheduledAt, r.sess.ScheduledPrompt, r.sess.ScheduledBy = at, prompt, ""
	return nil
}

func (r *watchRow) SetLichSchedule(_ string, at int64, prompt string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sess.ScheduledAt, r.sess.ScheduledPrompt, r.sess.ScheduledBy = at, prompt, api.ScheduledByLich
	return nil
}

func (r *watchRow) SetAgentSchedule(_ string, at int64, prompt string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sess.ScheduledAt, r.sess.ScheduledPrompt, r.sess.ScheduledBy = at, prompt, api.ScheduledByAgent
	return nil
}

func (r *watchRow) watch() api.SessionWatch {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sess.Watch
}

func (r *watchRow) row() store.Session {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sess
}

func watching(w api.SessionWatch) *watchRow {
	return &watchRow{sess: store.Session{ID: "s1", Label: "one", Kind: "claude", Watch: w}}
}

func newWatchRelay(r *watchRow) (*Service, *fakeTerminal, *watchEvents) {
	term, evs := newFakeTerminal("s1"), &watchEvents{fakeEvents: &fakeEvents{}}
	svc := newRelay(r, term, evs)
	svc.now = at(1000)
	return svc, term, evs
}

func red(head string) api.PRRead {
	return api.PRRead{Path: "/repo", Number: 7, Open: true, Head: head, Checks: api.ChecksRollup{Failed: 1, Passed: 1, Total: 2}}
}

func green(head string) api.PRRead {
	return api.PRRead{Path: "/repo", Number: 7, Open: true, Head: head, Checks: api.ChecksRollup{Passed: 2, Total: 2}}
}

// watchEvents records the stop events fakeEvents has no list for.
type watchEvents struct {
	*fakeEvents
	mu    sync.Mutex
	stops []api.SessionWatchStopped
}

func (e *watchEvents) Emit(name string, data any) {
	if stop, ok := data.(api.SessionWatchStopped); ok && name == api.SessionWatchStoppedEvent.Name {
		e.mu.Lock()
		defer e.mu.Unlock()
		e.stops = append(e.stops, stop)
	}
	e.fakeEvents.Emit(name, data)
}

func (e *watchEvents) watchStops() []api.SessionWatchStopped {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]api.SessionWatchStopped(nil), e.stops...)
}

func setStarting(term *fakeTerminal, starting bool) {
	term.mu.Lock()
	defer term.mu.Unlock()
	term.starting["s1"] = starting
}

func typedAt(term *fakeTerminal) string {
	term.mu.Lock()
	defer term.mu.Unlock()
	return strings.Join(term.writes["s1"], "")
}

func waitTyped(t *testing.T, term *fakeTerminal, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(typedAt(term), want) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("typed %q, want it to contain %q", typedAt(term), want)
}

func TestWatchTypesThePromptOncePerFailingCommit(t *testing.T) {
	t.Parallel()
	r := watching(api.SessionWatch{On: api.WatchChecksFailed, Prompt: "fix it"})
	svc, term, _ := newWatchRelay(r)

	svc.ReadPullRequest(red("aaaaaaaaaaaaaaaa"))
	waitTyped(t, term, "fix it")
	waitTyped(t, term, "aaaaaaaaaaaa")
	svc.ReadPullRequest(red("aaaaaaaaaaaaaaaa"))

	if got := r.watch(); got.Runs != 1 || got.PR != 7 || got.On != api.WatchChecksFailed {
		t.Fatalf("watch = %+v, want one run and still armed", got)
	}
}

func TestWatchWaitsForEveryCheckToFinish(t *testing.T) {
	t.Parallel()
	r := watching(api.SessionWatch{On: api.WatchChecksFailed, Prompt: "fix it"})
	svc, _, _ := newWatchRelay(r)

	read := red("a")
	read.Checks.Pending, read.Checks.Total = 1, 3
	svc.ReadPullRequest(read)

	if got := r.watch(); got.Runs != 0 {
		t.Fatalf("fired with a check still running: %+v", got)
	}
}

func TestWatchLeavesABusySessionToTheNextRead(t *testing.T) {
	t.Parallel()
	r := watching(api.SessionWatch{On: api.WatchChecksFailed, Prompt: "fix it"})
	svc, term, _ := newWatchRelay(r)
	setStarting(term, true)

	svc.ReadPullRequest(red("a"))
	if got := r.watch(); got.Runs != 0 || got.Seen != "" {
		t.Fatalf("watch moved on a session that could not take it: %+v", got)
	}

	setStarting(term, false)
	svc.ReadPullRequest(red("a"))
	waitTyped(t, term, "fix it")
}

func TestWatchStopsOnlyWhenTheChecksFailAfterTheLastRun(t *testing.T) {
	t.Parallel()
	r := watching(api.SessionWatch{On: api.WatchChecksFailed, Prompt: "fix it"})
	svc, _, evs := newWatchRelay(r)

	for _, head := range []string{"a", "b", "c"} {
		svc.ReadPullRequest(red(head))
	}
	if got := r.watch(); got.Runs != api.WatchRunLimit || got.On == "" {
		t.Fatalf("after the limit's last run: %+v, want still armed", got)
	}
	if len(evs.watchStops()) != 0 {
		t.Fatal("told the person it stopped before anything failed again")
	}

	svc.ReadPullRequest(red("d"))
	if got := r.watch(); got.On != "" || got.Runs != api.WatchRunLimit {
		t.Fatalf("after failing again: %+v, want disarmed", got)
	}
	if got := evs.watchStops(); len(got) != 1 || got[0].Number != 7 || got[0].Runs != api.WatchRunLimit {
		t.Fatalf("stop events = %+v", got)
	}
}

func TestWatchRunsOnceForPassedChecks(t *testing.T) {
	t.Parallel()
	r := watching(api.SessionWatch{On: api.WatchChecksPassed, Prompt: "merge"})
	svc, term, _ := newWatchRelay(r)

	svc.ReadPullRequest(green("a"))
	waitTyped(t, term, "merge")

	if got := r.watch(); got.On != "" || got.Prompt != "" || got.Seen != "p:a" {
		t.Fatalf("watch = %+v, want disarmed with the commit remembered", got)
	}
}

func TestWatchTakesTheReviewsAlreadyThereAsTheBaseline(t *testing.T) {
	t.Parallel()
	r := watching(api.SessionWatch{On: api.WatchReview, Prompt: "address it"})
	svc, term, _ := newWatchRelay(r)
	read := api.PRRead{Path: "/repo", Number: 7, Open: true, Head: "a", Review: "R1"}

	svc.ReadPullRequest(read)
	if got := r.watch(); got.Runs != 0 || got.On == "" {
		t.Fatalf("fired on a review older than the watch: %+v", got)
	}
	svc.ReadPullRequest(read)
	if got := r.watch(); got.Runs != 0 {
		t.Fatalf("fired on the baseline review: %+v", got)
	}

	read.Review = "R2"
	svc.ReadPullRequest(read)
	waitTyped(t, term, "address it")
	if strings.Contains(typedAt(term), "R2") {
		t.Fatal("the notice named the review's id")
	}
}

func TestWatchDisarmsWhenThePullRequestCloses(t *testing.T) {
	t.Parallel()
	r := watching(api.SessionWatch{On: api.WatchChecksFailed, Prompt: "fix it", Runs: 2})
	svc, _, evs := newWatchRelay(r)

	svc.ReadPullRequest(api.PRRead{Path: "/repo", Number: 7})

	if got := r.watch(); got != (api.SessionWatch{}) {
		t.Fatalf("watch = %+v, want cleared", got)
	}
	if len(evs.watchStops()) != 0 {
		t.Fatal("a closed pull request is not a watch that gave up")
	}
}

func TestWatchStartsCountingAgainOnANewPullRequest(t *testing.T) {
	t.Parallel()
	r := watching(api.SessionWatch{On: api.WatchChecksFailed, Prompt: "fix it", Runs: api.WatchRunLimit, PR: 3, Seen: "f:z"})
	svc, _, evs := newWatchRelay(r)

	svc.ReadPullRequest(red("a"))

	if got := r.watch(); got.Runs != 1 || got.PR != 7 {
		t.Fatalf("watch = %+v, want one run on the new pull request", got)
	}
	if len(evs.watchStops()) != 0 {
		t.Fatal("stopped on a pull request it had never run on")
	}
}

func TestAgentCannotReplaceWhatThePersonScheduled(t *testing.T) {
	t.Parallel()
	opts := api.ScheduleSelfOptions{From: "s1", On: api.WatchChecksFailed, Prompt: "mine"}
	for name, r := range map[string]*watchRow{
		"a watch":  watching(api.SessionWatch{On: api.WatchReview, Prompt: "theirs"}),
		"a prompt": {sess: store.Session{ID: "s1", Kind: "claude", ScheduledAt: 5000, ScheduledPrompt: "theirs"}},
	} {
		svc, _, _ := newWatchRelay(r)
		if _, err := svc.ScheduleSelf(context.Background(), opts); err == nil {
			t.Errorf("%s: an agent replaced what the person set", name)
		}
		if _, err := svc.ScheduleSelf(context.Background(), api.ScheduleSelfOptions{From: "s1"}); err == nil {
			t.Errorf("%s: an agent cleared what the person set", name)
		}
	}
}

func TestAgentRearmingKeepsItsRunCountButAPersonResetsIt(t *testing.T) {
	t.Parallel()
	armed := api.SessionWatch{Runs: api.WatchRunLimit, PR: 7, Seen: "f:a", By: api.ScheduledByAgent}
	on := api.WatchChecksFailed

	r := watching(armed)
	svc, _, _ := newWatchRelay(r)
	if _, err := svc.ScheduleSelf(context.Background(), api.ScheduleSelfOptions{From: "s1", On: on, Prompt: "again"}); err != nil {
		t.Fatal(err)
	}
	if got := r.watch(); got.Runs != api.WatchRunLimit || got.Seen != "f:a" || got.By != api.ScheduledByAgent {
		t.Fatalf("agent re-arm: %+v, want the count kept", got)
	}

	if _, err := svc.SetSchedule(context.Background(), api.SetScheduleOptions{SessionID: "s1", On: on, Prompt: "mine"}); err != nil {
		t.Fatal(err)
	}
	if got := r.watch(); got.Runs != 0 || got.Seen != "" || got.By != "" {
		t.Fatalf("person's arm: %+v, want the count started over", got)
	}
}

func TestSettingATimeReplacesTheWatchAndTheOtherWayRound(t *testing.T) {
	t.Parallel()
	r := watching(api.SessionWatch{On: api.WatchReview, Prompt: "old"})
	svc, _, _ := newWatchRelay(r)
	ctx := context.Background()

	if _, err := svc.SetSchedule(ctx, api.SetScheduleOptions{SessionID: "s1", At: 5000, Prompt: "later"}); err != nil {
		t.Fatal(err)
	}
	if got := r.row(); got.Watch.On != "" || got.ScheduledPrompt != "later" {
		t.Fatalf("after a time: %+v", got)
	}

	if _, err := svc.SetSchedule(ctx, api.SetScheduleOptions{SessionID: "s1", On: api.WatchReview, Prompt: "new"}); err != nil {
		t.Fatal(err)
	}
	if got := r.row(); got.Watch.On != api.WatchReview || got.ScheduledPrompt != "" {
		t.Fatalf("after a watch: %+v", got)
	}
}

func TestArmingAWatchLeavesLichsContinuationParked(t *testing.T) {
	t.Parallel()
	r := &watchRow{sess: store.Session{ID: "s1", Kind: "claude", ScheduledAt: 5000, ScheduledPrompt: resumePrompt, ScheduledBy: api.ScheduledByLich}}
	svc, _, _ := newWatchRelay(r)

	if _, err := svc.SetSchedule(context.Background(), api.SetScheduleOptions{SessionID: "s1", On: api.WatchReview, Prompt: "new"}); err != nil {
		t.Fatal(err)
	}
	if got := r.row(); got.ScheduledPrompt != resumePrompt || got.Watch.On == "" {
		t.Fatalf("row = %+v, want both", got)
	}

	if _, err := svc.SetSchedule(context.Background(), api.SetScheduleOptions{SessionID: "s1"}); err != nil {
		t.Fatal(err)
	}
	if got := r.row(); got.Watch.On != "" || got.ScheduledPrompt != resumePrompt {
		t.Fatalf("clearing took the wrong one: %+v", got)
	}
}

func TestScheduleRefusesWhatCannotRun(t *testing.T) {
	t.Parallel()
	r := watching(api.SessionWatch{})
	svc, _, _ := newWatchRelay(r)
	for name, opts := range map[string]api.SetScheduleOptions{
		"both":       {SessionID: "s1", At: 5000, On: api.WatchReview, Prompt: "x"},
		"no prompt":  {SessionID: "s1", At: 5000},
		"past":       {SessionID: "s1", At: 999, Prompt: "x"},
		"unknown on": {SessionID: "s1", On: "deployed", Prompt: "x"},
		"no session": {SessionID: "gone", At: 5000, Prompt: "x"},
	} {
		if _, err := svc.SetSchedule(context.Background(), opts); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
