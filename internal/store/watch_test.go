package store

import (
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
)

func TestSetSessionWatchReadsBackOnTheSessionRow(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s1", Label: "Session 1"})

	w := api.SessionWatch{On: api.WatchChecksFailed, Prompt: "  fix it\n", By: api.ScheduledByAgent, Runs: 2, PR: 9, Seen: "f:a"}
	if err := svc.SetSessionWatch("s1", w); err != nil {
		t.Fatalf("SetSessionWatch: %v", err)
	}
	w.Prompt = "fix it"
	if got := onlySession(t, svc).Watch; got != w {
		t.Fatalf("watch = %+v, want %+v", got, w)
	}

	if err := svc.SetSessionWatch("s1", api.SessionWatch{On: api.WatchReview, Prompt: strings.Repeat("x", schedulePromptLimit+1)}); err == nil {
		t.Fatal("a prompt over the limit was accepted")
	}
}

func TestScheduledByNamesWhoParkedThePrompt(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s1", Label: "Session 1"})

	for _, c := range []struct {
		write func() error
		want  string
	}{
		{func() error { return svc.SetAgentSchedule("s1", 1700000000, "mine") }, api.ScheduledByAgent},
		{func() error { return svc.SetSessionSchedule("s1", 1700000000, "theirs") }, ""},
		{func() error { return svc.SetSessionSchedule("s1", 1700000000, prompt.For(prompt.Spanish).ResumePrompt) }, api.ScheduledByLich},
	} {
		if err := c.write(); err != nil {
			t.Fatal(err)
		}
		if got := onlySession(t, svc).ScheduledBy; got != c.want {
			t.Errorf("ScheduledBy = %q, want %q", got, c.want)
		}
	}
}

func TestParkAndReopenCarryTheWatch(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "base", Label: "Session 1", Kind: "claude", NextSeq: 2})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "wt1", Label: "worker", Kind: "claude", Path: "/wt/foo"})
	w := api.SessionWatch{On: api.WatchReview, Prompt: "address it", Runs: 1, PR: 3, Seen: "r:x"}
	_ = svc.SetSessionWatch("wt1", w)
	_ = svc.SetAgentSchedule("wt1", 1700000000, "later")
	_, _ = svc.CloseSession(t.Context(), api.ParkSessionOptions{ProjectID: "p1", SessionID: "wt1", ActiveID: "base"})

	restored, err := svc.ReopenWorktreeSession(t.Context(), api.ReopenWorktreeSessionOptions{ProjectID: "p1", Path: "/wt/foo", NewSessionID: "wt2"})
	if err != nil || restored == nil {
		t.Fatalf("reopen = %+v, %v", restored, err)
	}
	if restored.Watch != w || restored.ScheduledBy != api.ScheduledByAgent {
		t.Fatalf("restored = %+v, want the watch and who parked the prompt", restored)
	}
	if got := sessionByID(t, svc, "wt2"); got.Watch != w {
		t.Fatalf("row = %+v, want the watch", got.Watch)
	}
}
