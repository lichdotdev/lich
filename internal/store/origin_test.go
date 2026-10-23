package store

import (
	"github.com/lichdotdev/lich/api"
	"testing"
)

// TestSessionOriginPersistsAndDefaults pins the two halves of the origin: the id
// of the session that asked for this one, and the label it was called at the
// time. Both come back from LoadState, and a session nobody delegated carries
// neither — absent is the normal case.
func TestSessionOriginPersistsAndDefaults(t *testing.T) {
	svc := newTestStore(t)
	if _, err := svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"}); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	if _, err := svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "parent", Label: "planner", Kind: "claude", NextSeq: 2}); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	if _, err := svc.AddSessionFrom(t.Context(), api.AddSessionFromOptions{ProjectID: "p1", SessionID: "child", Label: "worker", Kind: "claude", Path: "/wt/a", NextSeq: 3, OriginID: "parent", OriginLabel: "planner"}); err != nil {
		t.Fatalf("AddSessionFrom: %v", err)
	}

	projects, err := svc.LoadState(t.Context(), api.None{})
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if len(projects) != 1 || len(projects[0].Sessions) != 2 {
		t.Fatalf("state = %+v", projects)
	}
	if parent := projects[0].Sessions[0]; parent.OriginSessionID != "" || parent.OriginLabel != "" {
		t.Errorf("parent origin = %+v, want none", parent)
	}
	child := projects[0].Sessions[1]
	if child.OriginSessionID != "parent" || child.OriginLabel != "planner" {
		t.Errorf("child origin = (%q, %q), want (parent, planner)", child.OriginSessionID, child.OriginLabel)
	}
}

// TestSessionOriginSurvivesARenameOfTheParent proves the shape does what a
// single stored name could not: the id is what carries the rename, so the row
// keeps naming the parent no matter what the parent is called now. The label
// stored beside it is the fallback for a parent that is gone, and is deliberately
// the name the delegation happened under.
func TestSessionOriginSurvivesARenameOfTheParent(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "parent", Label: "planner", Kind: "claude", NextSeq: 2})
	_, _ = svc.AddSessionFrom(t.Context(), api.AddSessionFromOptions{ProjectID: "p1", SessionID: "child", Label: "worker", Kind: "claude", NextSeq: 3, OriginID: "parent", OriginLabel: "planner"})

	if _, err := svc.RenameSession(t.Context(), api.LabelSessionOptions{SessionID: "parent", Label: "the boss"}); err != nil {
		t.Fatalf("RenameSession: %v", err)
	}
	projects, err := svc.LoadState(t.Context(), api.None{})
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	sessions := projects[0].Sessions
	if sessions[0].Label != "the boss" {
		t.Errorf("parent label = %q, want the boss", sessions[0].Label)
	}
	if sessions[1].OriginSessionID != "parent" {
		t.Errorf("child origin id = %q, want it to still name the parent", sessions[1].OriginSessionID)
	}
}

// TestReopenWorktreeSessionCarriesTheOrigin proves a parked worktree session
// comes back knowing where it came from. The resume reinserts the row under a
// fresh id, so anything not copied across is lost — and the origin is identity,
// which must outlive a park exactly as the label and the conversation id do.
func TestReopenWorktreeSessionCarriesTheOrigin(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "parent", Label: "planner", Kind: "claude", NextSeq: 2})
	_, _ = svc.AddSessionFrom(t.Context(), api.AddSessionFromOptions{ProjectID: "p1", SessionID: "child", Label: "worker", Kind: "claude", Path: "/wt/a", NextSeq: 3, OriginID: "parent", OriginLabel: "planner"})
	_, _ = svc.CloseSession(t.Context(), api.ParkSessionOptions{ProjectID: "p1", SessionID: "child", ActiveID: "parent"})

	restored, err := svc.ReopenWorktreeSession(t.Context(), api.ReopenWorktreeSessionOptions{ProjectID: "p1", Path: "/wt/a", NewSessionID: "child2"})
	if err != nil {
		t.Fatalf("ReopenWorktreeSession: %v", err)
	}
	if restored == nil {
		t.Fatal("ReopenWorktreeSession = nil, want the parked session")
	}
	if restored.OriginSessionID != "parent" || restored.OriginLabel != "planner" {
		t.Errorf("restored origin = (%q, %q), want (parent, planner)",
			restored.OriginSessionID, restored.OriginLabel)
	}
	projects, err := svc.LoadState(t.Context(), api.None{})
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	var reloaded Session
	for _, s := range projects[0].Sessions {
		if s.ID == "child2" {
			reloaded = s
		}
	}
	if reloaded.OriginSessionID != "parent" || reloaded.OriginLabel != "planner" {
		t.Errorf("reloaded origin = %+v, want the parent it was opened from", reloaded)
	}
}

// TestMarkForkedRidesTheRowThroughAReopen: a fork is told apart from a
// delegate or a subagent, which carry an origin too, by its own mark, and a
// park and resume is not what makes a card stop being a fork.
func TestMarkForkedRidesTheRowThroughAReopen(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "keep", Label: "auth-refactor", Kind: "claude", NextSeq: 1})
	_, _ = svc.AddSessionFrom(t.Context(), api.AddSessionFromOptions{ProjectID: "p1", SessionID: "fork", Label: "auth-refactor 2", Kind: "claude", NextSeq: 2, OriginID: "keep", OriginLabel: "auth-refactor"})
	_, _ = svc.AddSessionFrom(t.Context(), api.AddSessionFromOptions{ProjectID: "p1", SessionID: "worker", Label: "worker", Kind: "claude", NextSeq: 3, OriginID: "keep", OriginLabel: "auth-refactor"})
	if _, err := svc.MarkForked(t.Context(), api.SessionIDOptions{ID: "fork"}); err != nil {
		t.Fatalf("MarkForked: %v", err)
	}

	forked := func() map[string]bool {
		projects, _ := svc.LoadState(t.Context(), api.None{})
		got := map[string]bool{}
		for _, s := range projects[0].Sessions {
			got[s.ID] = s.Forked
		}
		return got
	}
	if got := forked(); !got["fork"] || got["worker"] || got["keep"] {
		t.Fatalf("forked = %v, want only the fork marked", got)
	}
	_, _ = svc.CloseSession(t.Context(), api.ParkSessionOptions{ProjectID: "p1", SessionID: "fork", ActiveID: "keep"})
	restored, err := svc.ReopenSession(t.Context(), api.ReopenSessionOptions{SessionID: "fork", NewSessionID: "fork-again"})
	if err != nil || restored == nil || !restored.Forked {
		t.Fatalf("reopen = %+v, %v; want the mark carried", restored, err)
	}
	if !forked()["fork-again"] {
		t.Error("the reopened row lost its fork mark on reload")
	}
}
