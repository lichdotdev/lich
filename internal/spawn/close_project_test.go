package spawn

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/internal/store"
)

// projectClosable is a workspace where the caller (s1) runs in one project and
// another, holding two sessions, is the one to close.
func projectClosable() []store.Project {
	return []store.Project{
		{ID: "p1", Name: "lich", Path: "/src/lich", Sessions: []store.Session{
			{ID: "s1", Label: "Session 1", Kind: "claude"},
		}},
		{ID: "p2", Name: "revu", Path: "/src/revu", Sessions: []store.Session{
			{ID: "s2", Label: "Session 1", Kind: "claude"},
			{ID: "s3", Label: "fix", Kind: "codex", Path: "/wt/fix"},
		}},
	}
}

func projectCloser(t *testing.T, projects []store.Project) (*Service, *fakeSessions, *fakeTerminal) {
	t.Helper()
	sessions := &fakeSessions{projects: projects}
	term := &fakeTerminal{}
	return New(sessions, &fakeWorktrees{}, term, &fakeEvents{}), sessions, term
}

func TestCloseProjectStopsEverySessionAndKeepsTheRows(t *testing.T) {
	svc, sessions, term := projectCloser(t, projectClosable())

	closed, err := svc.CloseProject(t.Context(), CloseProjectOptions{From: "s1", Project: "revu"})
	if err != nil {
		t.Fatalf("CloseProject: %v", err)
	}
	if closed.ID != "p2" || closed.Name != "revu" || closed.Path != "/src/revu" || closed.Sessions != 2 {
		t.Errorf("closed = %+v, want p2 with its two sessions", closed)
	}
	if !slices.Equal(sessions.closedProjects, []string{"p2"}) {
		t.Errorf("closed projects = %v, want p2", sessions.closedProjects)
	}
	if !slices.Equal(term.closed, []string{"s2", "s3"}) {
		t.Errorf("closed terminals = %v, want both of the project's sessions", term.closed)
	}
	for _, closerID := range term.closers {
		if closerID != "s1" {
			t.Errorf("a terminal close named %q as its closer, want s1", closerID)
		}
	}
	if len(sessions.parked) != 0 || len(sessions.deleted) != 0 {
		t.Errorf("parked %v, deleted %v: a project close keeps its sessions' rows as they are",
			sessions.parked, sessions.deleted)
	}
}

func TestCloseProjectFindsAProjectByItsPath(t *testing.T) {
	dir := t.TempDir()
	projects := projectClosable()
	projects[1].Path = dir
	svc, sessions, _ := projectCloser(t, projects)

	if _, err := svc.CloseProject(t.Context(), CloseProjectOptions{From: "s1", Project: dir}); err != nil {
		t.Fatalf("CloseProject: %v", err)
	}
	if !slices.Equal(sessions.closedProjects, []string{"p2"}) {
		t.Errorf("closed projects = %v, want the project at %s", sessions.closedProjects, dir)
	}
}

// Closing the caller's own project stops the agent asking, halfway through.
func TestCloseProjectRefusesTheCallersOwnProject(t *testing.T) {
	svc, sessions, term := projectCloser(t, projectClosable())

	_, err := svc.CloseProject(t.Context(), CloseProjectOptions{From: "s3", Project: "revu"})
	if err == nil || !strings.Contains(err.Error(), "runs in") {
		t.Fatalf("err = %v, want the caller's own project refused", err)
	}
	if len(sessions.closedProjects) != 0 || len(term.closed) != 0 {
		t.Error("wrote something for a close that was refused")
	}
}

// The window keeps Home open for good; a close from outside it cannot take it
// away either.
func TestCloseProjectRefusesHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory: %v", err)
	}
	projects := projectClosable()
	projects[1].Path = home
	svc, sessions, term := projectCloser(t, projects)

	_, err = svc.CloseProject(t.Context(), CloseProjectOptions{From: "s1", Project: "revu"})
	if err == nil || !strings.Contains(err.Error(), "home") {
		t.Fatalf("err = %v, want the home project refused", err)
	}
	if len(sessions.closedProjects) != 0 || len(term.closed) != 0 {
		t.Error("wrote something for a close that was refused")
	}
}

// The project is never inherited: the caller's own is the one it would be.
func TestCloseProjectNeedsAProject(t *testing.T) {
	svc, sessions, _ := projectCloser(t, projectClosable())

	if _, err := svc.CloseProject(t.Context(), CloseProjectOptions{From: "s1", Project: "  "}); err == nil {
		t.Fatal("closed a project nobody named")
	}
	if len(sessions.closedProjects) != 0 {
		t.Error("wrote something for a close that was refused")
	}
}

func TestCloseProjectRefusesAnUnknownProject(t *testing.T) {
	svc, sessions, _ := projectCloser(t, projectClosable())

	if _, err := svc.CloseProject(t.Context(), CloseProjectOptions{From: "s1", Project: "nope"}); err == nil {
		t.Fatal("closed a project the workspace does not hold")
	}
	if len(sessions.closedProjects) != 0 {
		t.Error("wrote something for a close that was refused")
	}
}

// The row goes first: a store that refuses the close leaves every session
// running, rather than a project on screen whose agents were all stopped.
func TestCloseProjectLeavesTheSessionsRunningWhenTheStoreRefuses(t *testing.T) {
	svc, sessions, term := projectCloser(t, projectClosable())
	sessions.closeProjErr = errors.New("disk full")

	if _, err := svc.CloseProject(t.Context(), CloseProjectOptions{From: "s1", Project: "revu"}); err == nil {
		t.Fatal("reported a close the store refused")
	}
	if len(term.closed) != 0 {
		t.Errorf("closed terminals %v for a project that is still open", term.closed)
	}
}

func TestCloseProjectRefusesAnUnreadableWorkspace(t *testing.T) {
	svc, sessions, _ := projectCloser(t, projectClosable())
	sessions.loadErr = errors.New("locked")

	if _, err := svc.CloseProject(t.Context(), CloseProjectOptions{From: "s1", Project: "revu"}); err == nil {
		t.Fatal("closed a project without reading the workspace")
	}
}

// The window logs a terminal that would not close and carries on; so does
// this, since the row is already closed.
func TestCloseProjectReportsTheCloseWhenAPTYRefuses(t *testing.T) {
	svc, sessions, term := projectCloser(t, projectClosable())
	term.closeErr = errors.New("gone")

	if _, err := svc.CloseProject(t.Context(), CloseProjectOptions{From: "s1", Project: "revu"}); err != nil {
		t.Fatalf("CloseProject: %v", err)
	}
	if len(sessions.closedProjects) != 1 || len(term.closed) != 2 {
		t.Errorf("closed projects %v, terminals %v, want the close carried through",
			sessions.closedProjects, term.closed)
	}
}
