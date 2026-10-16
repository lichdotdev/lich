package main

import (
	"strings"
	"testing"

	"github.com/omartelo/lich/native/lichclient"
)

func TestWorktreeLeftByOnlyTheLastSessionOfAWorktree(t *testing.T) {
	m := newModel(nil, func() {})
	m.projects = []lichclient.Project{{ID: "p", Path: "/repo", Sessions: []lichclient.Session{
		{ID: "root"}, {ID: "a", Path: "/wt/a"}, {ID: "b1", Path: "/wt/b"}, {ID: "b2", Path: "/wt/b"},
	}}}
	for id, want := range map[string]string{"root": "", "a": "/wt/a", "b1": "", "gone": ""} {
		if p, got := m.worktreeLeftBy(id); got != want || (want != "" && p.ID != "p") {
			t.Errorf("worktreeLeftBy(%q) = %q in %q, want %q", id, got, p.ID, want)
		}
	}
}

func TestCloseWorktreeConfirmNamesWhatTheRemovalTakes(t *testing.T) {
	c := closeWorktreeConfirm{path: "/wt/a", adopted: true}
	if title, desc, action := c.copy(); title != closeWorktreeTitle || action != pullsRemoveWorktree || !strings.Contains(desc, "did not create") {
		t.Fatalf("adopted: %q / %q / %q", title, desc, action)
	}
	c.dirty = true
	title, desc, action := c.copy()
	if title != closeWorktreeDirtyTitle || action != closeWorktreeDiscard ||
		!strings.HasPrefix(desc, "lich did not create") || !strings.Contains(desc, "uncommitted changes") {
		t.Fatalf("adopted and dirty: %q / %q / %q", title, desc, action)
	}
	c.adopted = false
	if _, desc, _ := c.copy(); !strings.HasPrefix(desc, "The worktree at /wt/a") {
		t.Fatalf("dirty: %q", desc)
	}
}
