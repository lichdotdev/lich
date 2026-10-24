package store

import (
	"context"
	"slices"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func addTestProject(t *testing.T, svc *Service, id string) {
	t.Helper()
	if _, err := svc.AddProject(context.Background(), api.AddProjectOptions{ID: id, Name: id + "-name", Path: "/tmp/" + id}); err != nil {
		t.Fatalf("add project %s: %v", id, err)
	}
}

func TestServicesRoundTripInInsertOrder(t *testing.T) {
	svc := newTestStore(t)
	addTestProject(t, svc, "p1")
	addTestProject(t, svc, "p2")
	want := []api.ServiceConfig{
		{ID: "a", ProjectID: "p1", Name: "tailscale", Command: "docker compose up tailscale", StartWithLich: true},
		{ID: "b", ProjectID: "p2", Name: "db", Command: "docker compose up db"},
		{ID: "c", ProjectID: "p1", Name: "aws", Command: "aws sso login"},
	}
	for _, c := range want {
		if err := svc.SaveService(c); err != nil {
			t.Fatal(err)
		}
	}
	got, err := svc.Services()
	if err != nil {
		t.Fatal(err)
	}
	order := []api.ServiceConfig{want[0], want[2], want[1]}
	if !slices.Equal(got, order) {
		t.Errorf("Services() = %+v, want %+v", got, order)
	}
}

func TestSaveServiceUpdatesInPlace(t *testing.T) {
	svc := newTestStore(t)
	addTestProject(t, svc, "p1")
	first := api.ServiceConfig{ID: "a", ProjectID: "p1", Name: "web", Command: "pnpm dev"}
	second := api.ServiceConfig{ID: "b", ProjectID: "p1", Name: "db", Command: "pg"}
	for _, c := range []api.ServiceConfig{first, second} {
		if err := svc.SaveService(c); err != nil {
			t.Fatal(err)
		}
	}
	edited := api.ServiceConfig{ID: "a", ProjectID: "p1", Name: "web", Command: "pnpm dev --host", StartWithLich: true}
	if err := svc.SaveService(edited); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Services()
	if err != nil {
		t.Fatal(err)
	}
	if want := []api.ServiceConfig{edited, second}; !slices.Equal(got, want) {
		t.Errorf("after edit Services() = %+v, want %+v", got, want)
	}
}

func TestSaveServiceRefusesATakenNameAndAMissingProject(t *testing.T) {
	svc := newTestStore(t)
	addTestProject(t, svc, "p1")
	if err := svc.SaveService(api.ServiceConfig{ID: "a", ProjectID: "p1", Name: "web", Command: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SaveService(api.ServiceConfig{ID: "b", ProjectID: "p1", Name: "web", Command: "y"}); err == nil {
		t.Error("a second service named web in p1 was saved")
	}
	if err := svc.SaveService(api.ServiceConfig{ID: "c", ProjectID: "nope", Name: "web", Command: "y"}); err == nil {
		t.Error("a service of a project that does not exist was saved")
	}
}

func TestServicesGoWithTheirProject(t *testing.T) {
	svc := newTestStore(t)
	addTestProject(t, svc, "p1")
	if err := svc.SaveService(api.ServiceConfig{ID: "a", ProjectID: "p1", Name: "web", Command: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteProject("p1"); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Services()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("services outlived their project: %+v", got)
	}
}

func TestDeleteServiceAndProjectName(t *testing.T) {
	svc := newTestStore(t)
	addTestProject(t, svc, "p1")
	if err := svc.SaveService(api.ServiceConfig{ID: "a", ProjectID: "p1", Name: "web", Command: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteService("a"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteService("a"); err != nil {
		t.Errorf("deleting a gone service: %v", err)
	}
	if got, _ := svc.Services(); len(got) != 0 {
		t.Errorf("Services() after delete = %+v", got)
	}
	if got := svc.ProjectName("p1"); got != "p1-name" {
		t.Errorf("ProjectName = %q", got)
	}
	if got := svc.ProjectName("nope"); got != "" {
		t.Errorf("ProjectName of a missing project = %q", got)
	}
}
