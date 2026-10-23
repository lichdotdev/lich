package store

import (
	"github.com/lichdotdev/lich/api"
	"testing"
)

func TestSessionSubagentDepthRoundTripsAndSurvivesAResume(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "base", Label: "Session 1", Kind: "claude", NextSeq: 2})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "wt1", Label: "worker", Kind: "claude", Path: "/wt/foo", NextSeq: 3})

	if err := svc.SetSessionSubagentDepth("wt1", 2); err != nil {
		t.Fatalf("SetSessionSubagentDepth: %v", err)
	}
	if got := svc.SessionSubagentDepth("wt1"); got != 2 {
		t.Fatalf("SessionSubagentDepth(wt1) = %d, want 2", got)
	}
	if svc.SessionSubagentDepth("base") != 0 || svc.SessionSubagentDepth("ghost") != 0 {
		t.Fatal("want only the marked session read as a subagent")
	}
	_, _ = svc.CloseSession(t.Context(), api.ParkSessionOptions{ProjectID: "p1", SessionID: "wt1", ActiveID: "base"})
	if _, err := svc.ReopenWorktreeSession(t.Context(), api.ReopenWorktreeSessionOptions{ProjectID: "p1", Path: "/wt/foo", NewSessionID: "wt2"}); err != nil {
		t.Fatalf("ReopenWorktreeSession: %v", err)
	}
	if got := svc.SessionSubagentDepth("wt2"); got != 2 {
		t.Errorf("the resumed worker is %d deep, want 2", got)
	}
}

func TestSessionBranchReadsTheSessionsCheckout(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s1", Label: "Session 1", Kind: "claude", NextSeq: 2})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "wt1", Label: "worker", Kind: "claude", Path: "/wt/foo", NextSeq: 3})
	if got := svc.SessionBranch("s1"); got != "" {
		t.Errorf("unwired SessionBranch = %q, want empty", got)
	}
	svc.SetBranchOf(func(path string) string { return "branch-of:" + path })

	if got := svc.SessionBranch("wt1"); got != "branch-of:/wt/foo" {
		t.Errorf("SessionBranch(wt1) = %q, want the worktree's", got)
	}
	if got := svc.SessionBranch("s1"); got != "branch-of:/tmp/alpha" {
		t.Errorf("SessionBranch(s1) = %q, want the project directory's", got)
	}
}

// A fork of a subagent worker's conversation is a subagent as deep as the
// worker, found by the provider conversation it branches, so the copy opens
// cards exactly as the worker it was copied from would.
func TestInheritSubagentFollowsTheForkedConversation(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "worker", Label: "Session 1", Kind: "claude", NextSeq: 2})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "plain", Label: "Session 2", Kind: "claude", NextSeq: 3})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "fork1", Label: "Session 3", Kind: "claude", NextSeq: 4})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "fork2", Label: "Session 4", Kind: "claude", NextSeq: 5})
	_ = svc.SetProviderSession("worker", "conv-worker")
	_ = svc.SetProviderSession("plain", "conv-plain")
	_ = svc.SetSessionSubagentDepth("worker", 2)

	if err := svc.InheritSubagent("fork1", "conv-worker"); err != nil {
		t.Fatalf("InheritSubagent: %v", err)
	}
	if got := svc.SessionSubagentDepth("fork1"); got != 2 {
		t.Errorf("fork of a worker's conversation is %d deep, want the worker's 2", got)
	}
	if err := svc.InheritSubagent("fork2", "conv-plain"); err != nil {
		t.Fatalf("InheritSubagent: %v", err)
	}
	if svc.SessionSubagentDepth("fork2") != 0 {
		t.Error("fork of a plain conversation is marked a subagent, want it left alone")
	}
}
