package main

import (
	"context"
	"testing"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

// launchRig is the window's providers wiring: a model, and the Settings
// screen whose writes have to reach it.
func launchRig(t *testing.T, f *sidebarBackend, c *backend.Client) (*model, *settingsScreen) {
	t.Helper()
	ctx := context.Background()
	m := newModel(c, func() {})
	s := newSettingsScreen(ctx, c, new(ui.Root), func() {})
	s.providers.book = m.providers
	if err := m.providers.reload(ctx); err != nil {
		t.Fatal(err)
	}
	return m, s
}

func claudeDetected(installed bool) []backend.DetectedProvider {
	return []backend.DetectedProvider{{ID: "claude", Name: "Claude Code", Installed: installed}}
}

func setDetected(f *sidebarBackend, list []backend.DetectedProvider) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.detected = list
}

func TestFixingABinaryPathMovesTheImplicitSessionKind(t *testing.T) {
	f, c := newSidebarBackend(t)
	setDetected(f, claudeDetected(false))
	m, s := launchRig(t, f, c)
	if got := m.newSessionKind(); got != "shell" {
		t.Fatalf("a machine with no binary spawns %q", got)
	}
	setDetected(f, claudeDetected(true))
	s.writeBin(s.providers.snapshot(), "claude", "", "/opt/claude")
	eventually(t, "the kind to follow the fixed path", func() bool { return m.newSessionKind() == "claude" })
}

func TestCheckingThePathAgainMovesTheImplicitSessionKind(t *testing.T) {
	f, c := newSidebarBackend(t)
	setDetected(f, claudeDetected(false))
	m, s := launchRig(t, f, c)
	if got := m.newSessionKind(); got != "shell" {
		t.Fatalf("a machine with no binary spawns %q", got)
	}
	setDetected(f, claudeDetected(true))
	s.providers.recheckPath(s)
	eventually(t, "the kind to follow the re-read $PATH", func() bool { return m.newSessionKind() == "claude" })
}

func twoInstalled(f *sidebarBackend) {
	setDetected(f, []backend.DetectedProvider{
		{ID: "claude", Name: "Claude Code", Installed: true},
		{ID: "codex", Name: "Codex", Installed: true},
	})
	f.mu.Lock()
	f.settings["provider.codex.enabled@"] = "1"
	f.mu.Unlock()
}

func TestAProjectDefaultIsOneValueEveryReaderSees(t *testing.T) {
	f, c := newSidebarBackend(t)
	twoInstalled(f)
	m, s := launchRig(t, f, c)
	m.mu.Lock()
	m.projects = []backend.Project{{ID: "p", Name: "p"}}
	m.providers.noteProjects(m.projects)
	m.mu.Unlock()
	s.scope = "p"
	s.setProviderDefault("codex")
	if got := m.newSessionKind(); got != "codex" {
		t.Fatalf("the launch kind before the backend answered: %q", got)
	}
	if got, ok := s.snapshot().set(providerDefaultKey, "p"); !ok || got != "codex" {
		t.Fatalf("Settings reads %q, %v", got, ok)
	}
	waitForSetting(t, f, "provider.default@p", "codex")
	if err := m.providers.reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := m.newSessionKind(); got != "codex" {
		t.Fatalf("the launch kind after a read: %q", got)
	}

	s.useGlobal(providerDefaultKey, "p")
	if got := m.newSessionKind(); got != "claude" {
		t.Fatalf("a project that follows Global spawns %q", got)
	}
	waitForSetting(t, f, "provider.default@p", "")
}

func TestSwitchingAProviderOffReachesTheLaunchMenuAndSandbox(t *testing.T) {
	f, c := newSidebarBackend(t)
	twoInstalled(f)
	m, s := launchRig(t, f, c)
	if n := len(m.launchProviders()); n != 2 {
		t.Fatalf("%d providers offered", n)
	}
	s.setProviderEnabled("codex", false)
	if n := len(s.providers.book.launchProviders()); n != 1 {
		t.Fatalf("Sandbox's rows list %d providers", n)
	}
	waitForSetting(t, f, "provider.codex.enabled@", "0")
	eventually(t, "the launch menu to drop it", func() bool { return len(m.launchProviders()) == 1 })
}

func TestAProjectOpenedAfterTheReadBringsItsDefault(t *testing.T) {
	f, c := newSidebarBackend(t)
	twoInstalled(f)
	m, _ := launchRig(t, f, c)
	m.mu.Lock()
	m.projects = []backend.Project{{ID: "late", DefaultProvider: "codex"}}
	m.activeProject = "late"
	m.providers.noteProjects(m.projects)
	m.mu.Unlock()
	if got := m.newSessionKind(); got != "codex" {
		t.Fatalf("the project's default is %q", got)
	}
}
