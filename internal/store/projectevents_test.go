package store

import (
	"reflect"
	"testing"
)

type projectEvent struct {
	name string
	data any
}

func recordProjectEvents(t *testing.T, svc *Service) *[]projectEvent {
	t.Helper()
	var got []projectEvent
	svc.SetProjectChanged(func(name string, data any) {
		got = append(got, projectEvent{name, data})
	})
	return &got
}

// The window reads the workspace back when it hears an event, so what it reads
// has to be the committed row, not the one before it.
func TestProjectEventsFollowTheCommit(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddProject("p2", "beta", "/tmp/beta")
	var seen [][]string
	svc.SetProjectChanged(func(string, any) {
		var ids []string
		for _, p := range mustLoad(t, svc) {
			ids = append(ids, p.ID)
		}
		seen = append(seen, ids)
	})

	_ = svc.ReorderProjects([]string{"p2", "p1"})
	_ = svc.CloseProject("p2")
	_ = svc.AddProject("p2", "beta", "/tmp/beta")

	want := [][]string{{"p2", "p1"}, {"p1"}, {"p2", "p1"}}
	if !reflect.DeepEqual(seen, want) {
		t.Errorf("open projects at each event = %v, want %v", seen, want)
	}
}

func TestAddProjectEventCarriesTheStoredIdentity(t *testing.T) {
	svc := newTestStore(t)
	got := recordProjectEvents(t, svc)

	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.CloseProject("p1")
	// A relocated checkout comes back under the same id at another path.
	_ = svc.AddProject("p1", "moved", "/tmp/moved")

	want := []projectEvent{
		{ProjectAddedEventName, ProjectAdded{ID: "p1", Name: "alpha", Path: "/tmp/alpha"}},
		{ProjectClosedEventName, ProjectClosed{ID: "p1"}},
		{ProjectAddedEventName, ProjectAdded{ID: "p1", Name: "moved", Path: "/tmp/moved"}},
	}
	if !reflect.DeepEqual(*got, want) {
		t.Errorf("events = %+v, want %+v", *got, want)
	}
}

func TestCloseProjectAnnouncesOnlyARowThatWasThere(t *testing.T) {
	svc := newTestStore(t)
	got := recordProjectEvents(t, svc)

	if err := svc.CloseProject("never-opened"); err != nil {
		t.Fatalf("CloseProject: %v", err)
	}
	if len(*got) != 0 {
		t.Errorf("closing an unknown project announced %+v", *got)
	}
}

func TestReorderProjectsAnnouncesTheStoredOrder(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	_ = svc.AddProject("p2", "beta", "/tmp/beta")
	got := recordProjectEvents(t, svc)

	if err := svc.ReorderProjects([]string{"p2", "p1"}); err != nil {
		t.Fatalf("ReorderProjects: %v", err)
	}
	want := []projectEvent{{ProjectsReorderedEventName, ProjectsReordered{IDs: []string{"p2", "p1"}}}}
	if !reflect.DeepEqual(*got, want) {
		t.Errorf("events = %+v, want %+v", *got, want)
	}
}

func TestAProjectMutationWithoutAListenerStillWrites(t *testing.T) {
	svc := newTestStore(t)
	_ = svc.AddProject("p1", "alpha", "/tmp/alpha")
	if err := svc.CloseProject("p1"); err != nil {
		t.Fatalf("CloseProject: %v", err)
	}
	if err := svc.ReorderProjects([]string{"p1"}); err != nil {
		t.Fatalf("ReorderProjects: %v", err)
	}
}

func mustLoad(t *testing.T, svc *Service) []Project {
	t.Helper()
	ps, err := svc.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	return ps
}
