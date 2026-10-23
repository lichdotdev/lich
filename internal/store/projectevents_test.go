package store

import (
	"reflect"
	"testing"

	"github.com/lichdotdev/lich/api"
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
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p2", Name: "beta", Path: "/tmp/beta"})
	var seen [][]string
	svc.SetProjectChanged(func(string, any) {
		var ids []string
		for _, p := range mustLoad(t, svc) {
			ids = append(ids, p.ID)
		}
		seen = append(seen, ids)
	})

	_, _ = svc.ReorderProjects(t.Context(), api.ReorderProjectsOptions{IDs: []string{"p2", "p1"}})
	_, _ = svc.CloseProject(t.Context(), api.ProjectIDOptions{ID: "p2"})
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p2", Name: "beta", Path: "/tmp/beta"})

	want := [][]string{{"p2", "p1"}, {"p1"}, {"p2", "p1"}}
	if !reflect.DeepEqual(seen, want) {
		t.Errorf("open projects at each event = %v, want %v", seen, want)
	}
}

func TestAddProjectEventCarriesTheStoredIdentity(t *testing.T) {
	svc := newTestStore(t)
	got := recordProjectEvents(t, svc)

	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.CloseProject(t.Context(), api.ProjectIDOptions{ID: "p1"})
	// A relocated checkout comes back under the same id at another path.
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "moved", Path: "/tmp/moved"})

	want := []projectEvent{
		{api.ProjectAddedEvent.Name, ProjectAdded{ID: "p1", Name: "alpha", Path: "/tmp/alpha"}},
		{api.ProjectClosedEvent.Name, ProjectClosed{ID: "p1"}},
		{api.ProjectAddedEvent.Name, ProjectAdded{ID: "p1", Name: "moved", Path: "/tmp/moved"}},
	}
	if !reflect.DeepEqual(*got, want) {
		t.Errorf("events = %+v, want %+v", *got, want)
	}
}

func TestCloseProjectAnnouncesOnlyARowThatWasThere(t *testing.T) {
	svc := newTestStore(t)
	got := recordProjectEvents(t, svc)

	if _, err := svc.CloseProject(t.Context(), api.ProjectIDOptions{ID: "never-opened"}); err != nil {
		t.Fatalf("CloseProject: %v", err)
	}
	if len(*got) != 0 {
		t.Errorf("closing an unknown project announced %+v", *got)
	}
}

func TestReorderProjectsAnnouncesTheStoredOrder(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p2", Name: "beta", Path: "/tmp/beta"})
	got := recordProjectEvents(t, svc)

	if _, err := svc.ReorderProjects(t.Context(), api.ReorderProjectsOptions{IDs: []string{"p2", "p1"}}); err != nil {
		t.Fatalf("ReorderProjects: %v", err)
	}
	want := []projectEvent{{api.ProjectsReorderedEvent.Name, ProjectsReordered{IDs: []string{"p2", "p1"}}}}
	if !reflect.DeepEqual(*got, want) {
		t.Errorf("events = %+v, want %+v", *got, want)
	}
}

func TestAProjectMutationWithoutAListenerStillWrites(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/tmp/alpha"})
	if _, err := svc.CloseProject(t.Context(), api.ProjectIDOptions{ID: "p1"}); err != nil {
		t.Fatalf("CloseProject: %v", err)
	}
	if _, err := svc.ReorderProjects(t.Context(), api.ReorderProjectsOptions{IDs: []string{"p1"}}); err != nil {
		t.Fatalf("ReorderProjects: %v", err)
	}
}

func mustLoad(t *testing.T, svc *Service) []Project {
	t.Helper()
	ps, err := svc.LoadState(t.Context(), api.None{})
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	return ps
}
