package store

import (
	"github.com/lichdotdev/lich/api"
	"testing"
)

// colorOf reads one session's card colour straight off the hydration, the same
// read the window paints from.
func colorOf(t *testing.T, svc *Service, projectID, sessionID string) string {
	t.Helper()
	sessions, err := svc.sessionsOf(projectID)
	if err != nil {
		t.Fatalf("sessionsOf: %v", err)
	}
	for _, s := range sessions {
		if s.ID == sessionID {
			return s.Color
		}
	}
	t.Fatalf("session %q not in project %q", sessionID, projectID)
	return ""
}

func TestSetSessionColorPaintsAndClears(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s1", Label: "Session 1", NextSeq: 2})

	if _, err := svc.SetSessionColor(t.Context(), api.SessionColorOptions{SessionID: "s1", Color: "blue"}); err != nil {
		t.Fatalf("SetSessionColor: %v", err)
	}
	if got := colorOf(t, svc, "p1", "s1"); got != "blue" {
		t.Errorf("color = %q, want %q", got, "blue")
	}
	if _, err := svc.SetSessionColor(t.Context(), api.SessionColorOptions{SessionID: "s1"}); err != nil {
		t.Fatalf("SetSessionColor(clear): %v", err)
	}
	if got := colorOf(t, svc, "p1", "s1"); got != "" {
		t.Errorf("color after clearing = %q, want empty", got)
	}
}

// TestResumeKeepsTheColor: the colour is the user's mark on the card, and
// closing a session is not a reason to take it off.
func TestResumeKeepsTheColor(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s1", Label: "Session 1", NextSeq: 2})
	_, _ = svc.SetSessionColor(t.Context(), api.SessionColorOptions{SessionID: "s1", Color: "green"})
	if _, err := svc.CloseSession(t.Context(), api.ParkSessionOptions{ProjectID: "p1", SessionID: "s1"}); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}

	restored, err := svc.ReopenSession(t.Context(), api.ReopenSessionOptions{SessionID: "s1", NewSessionID: "s1-new"})
	if err != nil {
		t.Fatalf("ReopenSession: %v", err)
	}
	if restored == nil {
		t.Fatal("ReopenSession returned no session")
	}
	if restored.Color != "green" {
		t.Errorf("resumed color = %q, want %q", restored.Color, "green")
	}
	if got := colorOf(t, svc, "p1", "s1-new"); got != "green" {
		t.Errorf("stored color after resume = %q, want %q", got, "green")
	}
}

// TestColorFolderPaintsItsSessionsOnly: painting a folder is one write over the
// cards filed under it, in that project alone, and answers which ones it hit.
func TestColorFolderPaintsItsSessionsOnly(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p2", Name: "beta", Path: "/tmp/beta"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s1", Label: "Session 1", NextSeq: 2})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s2", Label: "Session 2", NextSeq: 3})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p2", SessionID: "o1", Label: "Other 1", NextSeq: 2})
	_, _ = svc.SetSessionFolder(t.Context(), api.SessionFolderOptions{SessionID: "s1", Folder: "Apps"})
	_, _ = svc.SetSessionFolder(t.Context(), api.SessionFolderOptions{SessionID: "o1", Folder: "Apps"})

	painted, err := svc.ColorFolder("p1", "Apps", "teal")
	if err != nil {
		t.Fatalf("ColorFolder: %v", err)
	}
	if len(painted) != 1 || painted[0] != "s1" {
		t.Errorf("painted = %v, want [s1]", painted)
	}
	if got := colorOf(t, svc, "p1", "s1"); got != "teal" {
		t.Errorf("filed session color = %q, want teal", got)
	}
	if got := colorOf(t, svc, "p1", "s2"); got != "" {
		t.Errorf("unfiled session color = %q, want empty", got)
	}
	if got := colorOf(t, svc, "p2", "o1"); got != "" {
		t.Errorf("other project's session color = %q, want empty", got)
	}
}

// TestColorFolderRefusesTheEmptyName: "" is every unfiled session, which no
// folder colour was ever meant to reach.
func TestColorFolderRefusesTheEmptyName(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s1", Label: "Session 1", NextSeq: 2})

	if _, err := svc.ColorFolder("p1", "", "red"); err == nil {
		t.Fatal("ColorFolder with no folder = nil, want an error")
	}
	if got := colorOf(t, svc, "p1", "s1"); got != "" {
		t.Errorf("unfiled session color = %q, want empty", got)
	}
}
