package store

import (
	"github.com/lichdotdev/lich/api"
	"slices"
	"testing"
)

// folderOf reads one session's folder straight off the row, so a test asserting
// where a session was filed never depends on how the sidebar groups them.
func folderOf(t *testing.T, svc *Service, projectID, sessionID string) string {
	t.Helper()
	sessions, err := svc.sessionsOf(projectID)
	if err != nil {
		t.Fatalf("sessionsOf: %v", err)
	}
	for _, s := range sessions {
		if s.ID == sessionID {
			return s.Folder
		}
	}
	t.Fatalf("session %q not in project %q", sessionID, projectID)
	return ""
}

// TestSetSessionFolderFilesAndUnfiles: a folder is an assignment on the session,
// so filing one is a write and taking it out is the same write with an empty
// name — never a second concept the sidebar would have to tell apart.
func TestSetSessionFolderFilesAndUnfiles(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s1", Label: "Session 1", NextSeq: 2})

	if _, err := svc.SetSessionFolder(t.Context(), api.SessionFolderOptions{SessionID: "s1", Folder: "Design system"}); err != nil {
		t.Fatalf("SetSessionFolder: %v", err)
	}
	if got := folderOf(t, svc, "p1", "s1"); got != "Design system" {
		t.Errorf("folder = %q, want %q", got, "Design system")
	}
	if _, err := svc.SetSessionFolder(t.Context(), api.SessionFolderOptions{SessionID: "s1"}); err != nil {
		t.Fatalf("SetSessionFolder(unfile): %v", err)
	}
	if got := folderOf(t, svc, "p1", "s1"); got != "" {
		t.Errorf("folder after unfiling = %q, want empty", got)
	}
}

// TestSetSessionFolderLeavesTheDragOrder: filing a card must not move it. The
// sidebar lifts a filed card into its block at render time, exactly as it does
// a pinned one, so the stored order is what an unfiled card falls back into.
func TestSetSessionFolderLeavesTheDragOrder(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s1", Label: "Session 1", NextSeq: 2})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s2", Label: "Session 2", NextSeq: 3})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s3", Label: "Session 3", NextSeq: 4})
	_, _ = svc.ReorderSessions(t.Context(), api.ReorderSessionsOptions{ProjectID: "p1", IDs: []string{"s3", "s1", "s2"}})

	if _, err := svc.SetSessionFolder(t.Context(), api.SessionFolderOptions{SessionID: "s1", Folder: "Apps"}); err != nil {
		t.Fatalf("SetSessionFolder: %v", err)
	}
	if got := sessionIDs(t, svc, "p1"); !equalIDs(got, []string{"s3", "s1", "s2"}) {
		t.Errorf("order after filing = %v, want [s3 s1 s2]", got)
	}
}

// TestRenameFolderRewritesItsSessionsOnly: the name is the folder's identity, so
// a rename is a write across the sessions carrying it — and it stops at the
// project, because two projects can hold folders of the same name.
func TestRenameFolderRewritesItsSessionsOnly(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p2", Name: "beta", Path: "/tmp/beta"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s1", Label: "Session 1", NextSeq: 2})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s2", Label: "Session 2", NextSeq: 3})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p2", SessionID: "o1", Label: "Other 1", NextSeq: 2})
	_, _ = svc.SetSessionFolder(t.Context(), api.SessionFolderOptions{SessionID: "s1", Folder: "Apps"})
	_, _ = svc.SetSessionFolder(t.Context(), api.SessionFolderOptions{SessionID: "s2", Folder: "Infra"})
	_, _ = svc.SetSessionFolder(t.Context(), api.SessionFolderOptions{SessionID: "o1", Folder: "Apps"})

	if _, err := svc.RenameFolder(t.Context(), api.RenameProjectFolderOptions{ProjectID: "p1", From: "Apps", To: "Applications"}); err != nil {
		t.Fatalf("RenameFolder: %v", err)
	}
	if got := folderOf(t, svc, "p1", "s1"); got != "Applications" {
		t.Errorf("renamed folder = %q, want %q", got, "Applications")
	}
	if got := folderOf(t, svc, "p1", "s2"); got != "Infra" {
		t.Errorf("other folder = %q, want %q", got, "Infra")
	}
	if got := folderOf(t, svc, "p2", "o1"); got != "Apps" {
		t.Errorf("other project's folder = %q, want %q", got, "Apps")
	}
}

// TestRenameFolderToNothingUngroups: taking a folder apart is the rename with an
// empty target — one write, and every card falls back to its checkout's block.
func TestRenameFolderToNothingUngroups(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s1", Label: "Session 1", NextSeq: 2})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s2", Label: "Session 2", NextSeq: 3})
	_, _ = svc.SetSessionFolder(t.Context(), api.SessionFolderOptions{SessionID: "s1", Folder: "Apps"})
	_, _ = svc.SetSessionFolder(t.Context(), api.SessionFolderOptions{SessionID: "s2", Folder: "Apps"})

	if _, err := svc.RenameFolder(t.Context(), api.RenameProjectFolderOptions{ProjectID: "p1", From: "Apps"}); err != nil {
		t.Fatalf("RenameFolder: %v", err)
	}
	for _, id := range []string{"s1", "s2"} {
		if got := folderOf(t, svc, "p1", id); got != "" {
			t.Errorf("session %q folder = %q, want empty", id, got)
		}
	}
}

// TestRenameFolderRefusesTheEmptyName: without the guard, renaming "" would
// sweep every unfiled session in the project into a folder nobody asked for.
func TestRenameFolderRefusesTheEmptyName(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s1", Label: "Session 1", NextSeq: 2})

	if _, err := svc.RenameFolder(t.Context(), api.RenameProjectFolderOptions{ProjectID: "p1", To: "Apps"}); err == nil {
		t.Fatal("RenameFolder with no source folder = nil, want an error")
	}
	if got := folderOf(t, svc, "p1", "s1"); got != "" {
		t.Errorf("unfiled session folder = %q, want empty", got)
	}
}

// TestResumeKeepsTheFolder: where a session was filed is not something closing
// it undoes. A resume that dropped the folder would land the card back among its
// checkout's own — the pile the folder was made to break up.
func TestResumeKeepsTheFolder(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s1", Label: "Session 1", NextSeq: 2})
	_, _ = svc.SetSessionFolder(t.Context(), api.SessionFolderOptions{SessionID: "s1", Folder: "Design system"})
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
	if restored.Folder != "Design system" {
		t.Errorf("resumed folder = %q, want %q", restored.Folder, "Design system")
	}
	if got := folderOf(t, svc, "p1", "s1-new"); got != "Design system" {
		t.Errorf("stored folder after resume = %q, want %q", got, "Design system")
	}
}

// TestRenameFolderReachesParkedSessions: a parked row is resumed into the folder
// it is filed under, so a rename while it is away has to find it — otherwise the
// session comes back into a folder the user renamed out of existence.
func TestRenameFolderReachesParkedSessions(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s1", Label: "Session 1", NextSeq: 2})
	_, _ = svc.SetSessionFolder(t.Context(), api.SessionFolderOptions{SessionID: "s1", Folder: "Apps"})
	_, _ = svc.CloseSession(t.Context(), api.ParkSessionOptions{ProjectID: "p1", SessionID: "s1"})

	if _, err := svc.RenameFolder(t.Context(), api.RenameProjectFolderOptions{ProjectID: "p1", From: "Apps", To: "Applications"}); err != nil {
		t.Fatalf("RenameFolder: %v", err)
	}
	restored, err := svc.ReopenSession(t.Context(), api.ReopenSessionOptions{SessionID: "s1", NewSessionID: "s1-new"})
	if err != nil {
		t.Fatalf("ReopenSession: %v", err)
	}
	if restored == nil {
		t.Fatal("ReopenSession returned no session")
	}
	if restored.Folder != "Applications" {
		t.Errorf("resumed folder = %q, want %q", restored.Folder, "Applications")
	}
}

// TestRenameFolderReturnsEveryRowItMoved: the ids come from the write, parked
// rows included, so a caller announcing the move names exactly what it rewrote
// and never a card it read as filed a moment earlier.
func TestRenameFolderReturnsEveryRowItMoved(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s1", Label: "Session 1", NextSeq: 2})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s2", Label: "Session 2", NextSeq: 3})
	_, _ = svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s3", Label: "Session 3", NextSeq: 4})
	_, _ = svc.SetSessionFolder(t.Context(), api.SessionFolderOptions{SessionID: "s1", Folder: "Apps"})
	_, _ = svc.SetSessionFolder(t.Context(), api.SessionFolderOptions{SessionID: "s2", Folder: "Apps"})
	_, _ = svc.SetSessionFolder(t.Context(), api.SessionFolderOptions{SessionID: "s3", Folder: "Infra"})
	_, _ = svc.CloseSession(t.Context(), api.ParkSessionOptions{ProjectID: "p1", SessionID: "s2"})

	moved, err := svc.RenameFolder(t.Context(), api.RenameProjectFolderOptions{ProjectID: "p1", From: "Apps", To: "Applications"})
	if err != nil {
		t.Fatalf("RenameFolder: %v", err)
	}
	slices.Sort(moved)
	if !slices.Equal(moved, []string{"s1", "s2"}) {
		t.Errorf("moved = %v, want the open and the parked session", moved)
	}
}

// TestRenameFolderOfNothingMovesNothing: a name no row carries matches nothing,
// and the answer is an empty list rather than an error.
func TestRenameFolderOfNothingMovesNothing(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})

	moved, err := svc.RenameFolder(t.Context(), api.RenameProjectFolderOptions{ProjectID: "p1", From: "Apps", To: "Applications"})
	if err != nil {
		t.Fatalf("RenameFolder: %v", err)
	}
	if moved == nil || len(moved) != 0 {
		t.Errorf("moved = %#v, want an empty list", moved)
	}
}

// TestAddSessionFilesTheRowItInserts: a session opened into a folder carries it
// from the insert, with no filing write after it, so no crash can land between
// the row and its folder and bring the session back unfiled.
func TestAddSessionFilesTheRowItInserts(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})

	if _, err := svc.AddSessionInFolder(t.Context(), api.AddSessionInFolderOptions{ProjectID: "p1", SessionID: "s1", Label: "Session 1", NextSeq: 2, Folder: "Apps"}); err != nil {
		t.Fatalf("AddSessionInFolder: %v", err)
	}
	if _, err := svc.AddSessionFrom(t.Context(), api.AddSessionFromOptions{ProjectID: "p1", SessionID: "s2", Label: "worker", NextSeq: 3, OriginID: "s1", OriginLabel: "Session 1", Folder: "Apps"}); err != nil {
		t.Fatalf("AddSessionFrom: %v", err)
	}
	for _, id := range []string{"s1", "s2"} {
		if got := folderOf(t, svc, "p1", id); got != "Apps" {
			t.Errorf("%s folder = %q, want %q", id, got, "Apps")
		}
	}
}
