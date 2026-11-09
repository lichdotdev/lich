package store

import (
	"errors"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func gatedStore(t *testing.T) (*Service, error) {
	t.Helper()
	svc := newTestStore(t)
	if _, err := svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "old", Label: "Session 1", Kind: "claude", NextSeq: 2}); err != nil {
		t.Fatal(err)
	}
	refusal := errors.New("lich 0.10.0 is a required security update")
	svc.SetSessionGate(func() error { return refusal })
	return svc, refusal
}

func TestEveryInsertOfAnAgentSessionIsRefusedByTheGate(t *testing.T) {
	svc, refusal := gatedStore(t)
	inserts := map[string]func(id string) error{
		"AddSession": func(id string) error {
			_, err := svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: id, Label: id, Kind: "codex", NextSeq: 3})
			return err
		},
		"AddSession with no kind, which is Claude Code": func(id string) error {
			_, err := svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: id, Label: id, NextSeq: 3})
			return err
		},
		"AddSessionInFolder": func(id string) error {
			_, err := svc.AddSessionInFolder(t.Context(), api.AddSessionInFolderOptions{AddSessionOptions: api.AddSessionOptions{ProjectID: "p1", SessionID: id, Label: id, Kind: "claude", NextSeq: 3}, Folder: "f"})
			return err
		},
		"AddSessionFrom": func(id string) error {
			_, err := svc.AddSessionFrom(t.Context(), api.AddSessionFromOptions{ProjectID: "p1", SessionID: id, Label: id, Kind: "claude", NextSeq: 3, OriginID: "old", OriginLabel: "Session 1"})
			return err
		},
	}
	for name, insert := range inserts {
		t.Run(name, func(t *testing.T) {
			if err := insert("new"); !errors.Is(err, refusal) {
				t.Fatalf("err = %v, want the gate's refusal", err)
			}
		})
	}
	projects, err := svc.LoadState(t.Context(), api.None{})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(projects[0].Sessions); got != 1 || projects[0].Sessions[0].ID != "old" || projects[0].NextSeq != 2 {
		t.Fatalf("after the refusals: %d sessions, next_seq %d; want only the one that was there, untouched", got, projects[0].NextSeq)
	}
}

// The update is installed from a shell session, so the gate must not close it.
func TestAShellSessionOpensThroughTheGate(t *testing.T) {
	svc, _ := gatedStore(t)
	if _, err := svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "sh", Label: "Shell 2", Kind: "shell", NextSeq: 3}); err != nil {
		t.Fatalf("AddSession(shell) = %v, want it open", err)
	}
	if err := svc.SessionRefused("shell"); err != nil {
		t.Fatalf("SessionRefused(shell) = %v", err)
	}
}

func TestNothingIsRefusedOnceTheGateLifts(t *testing.T) {
	svc, _ := gatedStore(t)
	svc.SetSessionGate(func() error { return nil })
	if _, err := svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "new", Label: "Session 2", Kind: "claude", NextSeq: 3}); err != nil {
		t.Fatalf("AddSession = %v, want it open", err)
	}
}

// A session lich already holds is the person's work: the gate stops new ones,
// not a parked one coming back.
func TestAParkedSessionComesBackThroughTheGate(t *testing.T) {
	svc, _ := gatedStore(t)
	if _, err := svc.CloseSession(t.Context(), api.ParkSessionOptions{ProjectID: "p1", SessionID: "old"}); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}
	restored, err := svc.ReopenSession(t.Context(), api.ReopenSessionOptions{SessionID: "old", NewSessionID: "back"})
	if err != nil || restored == nil {
		t.Fatalf("ReopenSession = %v, %v; want the parked row back", restored, err)
	}
}
