package store

import (
	"github.com/lichdotdev/lich/api"
	"testing"
)

func TestRenameSessionReportsTheNewLabelOnceWritten(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s1", Label: "Session 1", Kind: "claude", NextSeq: 1})

	type rename struct{ id, label string }
	var seen []rename
	var stored string
	svc.SetSessionRenamed(func(id, label string) {
		seen = append(seen, rename{id, label})
		_ = svc.db.QueryRow(`SELECT label FROM sessions WHERE id = ?`, id).Scan(&stored)
	})

	if _, err := svc.RenameSession(t.Context(), api.LabelSessionOptions{SessionID: "s1", Label: "build"}); err != nil {
		t.Fatalf("RenameSession: %v", err)
	}
	if len(seen) != 1 || seen[0] != (rename{"s1", "build"}) {
		t.Fatalf("reported %+v, want the one rename", seen)
	}
	if stored != "build" {
		t.Errorf("label %q when reported, want the row already written", stored)
	}
}

func TestRenameSessionReportsNothingWhenTheWriteFails(t *testing.T) {
	svc := newTestStore(t)
	reported := false
	svc.SetSessionRenamed(func(string, string) { reported = true })
	_ = svc.db.Close()

	if _, err := svc.RenameSession(t.Context(), api.LabelSessionOptions{SessionID: "s1", Label: "build"}); err == nil {
		t.Fatal("RenameSession on a closed database succeeded")
	}
	if reported {
		t.Error("reported a rename that was never written")
	}
}
