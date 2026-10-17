package main

import (
	"context"
	"testing"

	"github.com/lichdotdev/lich/native/lichclient"
)

func homeModel() *model {
	m := newModel(nil, func() {})
	m.homeID = "home"
	m.projects = []lichclient.Project{
		{ID: "home", Name: "me", Path: "/home/me", ActiveSessionID: "h1", Sessions: []lichclient.Session{{ID: "h1"}}},
		{ID: "a", Name: "lich", Path: "/src/lich", ActiveSessionID: "a1", Sessions: []lichclient.Session{{ID: "a1"}}},
	}
	m.activeProject, m.active = "a", "a1"
	return m
}

func TestHomeIsTheHomeDirectorysProject(t *testing.T) {
	m := homeModel()
	m.showHome()
	p, ok := m.project()
	if !ok || p.ID != "home" {
		t.Fatalf("project on Home = %q, %v; want the home project, so New Session has somewhere to go", p.ID, ok)
	}
	if !m.onHome() {
		t.Fatal("onHome is false with Home chosen")
	}
	if m.active != "h1" {
		t.Fatalf("active session on Home = %q, want the home project's h1", m.active)
	}
	tabs, active := m.projectTabs()
	if active != "" || len(tabs) != 1 || tabs[0].ID != "a" {
		t.Fatalf("tabs %v active %q, want only the lich tab and none active: Home is the brand, not a tab", tabs, active)
	}
	if !m.setActiveProject("a") || m.onHome() {
		t.Fatal("choosing a tab did not leave Home")
	}
}

func TestHomeCannotBeClosed(t *testing.T) {
	m := homeModel()
	if _, err := m.askClose(context.Background(), "home"); err == nil {
		t.Fatal("closing the home project went through; Home is permanent")
	}
}

func TestAFreshWorkspaceOpensOnHomeWithAShell(t *testing.T) {
	var ps []lichclient.Project
	m := newModel(fakeState(t, &ps, ""), func() {})
	if err := m.reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	p, ok := m.project()
	if !ok || p.ID != fakeHome.ID || len(p.Sessions) != 1 || p.Sessions[0].Kind != "shell" {
		t.Fatalf("fresh workspace shows %+v, %v; want Home with one shell", p, ok)
	}
}
