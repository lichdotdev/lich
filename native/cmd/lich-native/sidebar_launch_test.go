package main

import (
	"context"
	"testing"

	"github.com/omartelo/lich/native/lichclient"
)

func TestReadEnabledLeavesOnlyClaudeOnByDefault(t *testing.T) {
	cases := []struct {
		id, value string
		want      bool
	}{
		{"claude", "", true}, {"codex", "", false}, {"codex", "1", true}, {"claude", "0", false}, {"codex", "yes", false},
	}
	for _, tc := range cases {
		if got := readEnabled(tc.id, tc.value); got != tc.want {
			t.Errorf("readEnabled(%q, %q) = %v", tc.id, tc.value, got)
		}
	}
}

func providerList() []launchProvider {
	return []launchProvider{
		{id: "claude", name: "Claude Code", installed: true, enabled: false},
		{id: "codex", name: "Codex", installed: true, enabled: true},
		{id: "opencode", name: "opencode", installed: false, enabled: true},
	}
}

func TestResolveProjectDefaultProvider(t *testing.T) {
	cases := []struct {
		name, global, project, want string
		list                        []launchProvider
	}{
		{"an enabled project default wins", "codex", "opencode", "opencode", providerList()},
		{"a disabled project default falls to the global", "codex", "claude", "codex", providerList()},
		{"a disabled global falls to the first enabled", "claude", "", "codex", providerList()},
		{"nothing loaded is Claude", "", "", "claude", nil},
	}
	for _, tc := range cases {
		if got := resolveProjectDefaultProvider(tc.list, tc.global, tc.project); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestImplicitKindIsTheShellWithNothingInstalled(t *testing.T) {
	bare := []launchProvider{{id: "claude", enabled: true}, {id: "codex", enabled: true}}
	if got := implicitSessionKind(bare, "codex", ""); got != "shell" {
		t.Fatalf("bare machine spawns %q", got)
	}
	if got := implicitSessionKind(nil, "", ""); got != "claude" {
		t.Fatalf("detection not answered yet spawns %q", got)
	}
	if got := implicitSessionKind(providerList(), "", ""); got != "codex" {
		t.Fatalf("installed machine spawns %q", got)
	}
}

func TestFolderLaunchHeading(t *testing.T) {
	one := []launchCheckout{{path: "/wt/a", label: "feat/a"}}
	if got := folderLaunchHeading("Apps", one); got != "New session in Apps, on feat/a" {
		t.Fatalf("one checkout: %q", got)
	}
	if got := folderLaunchHeading("Apps", append(one, launchCheckout{label: "repo"})); got != "New session in Apps" {
		t.Fatalf("two checkouts: %q", got)
	}
}

func TestModelLaunchMenuReadsDetectionFlagsAndDefaults(t *testing.T) {
	f, c := newSidebarBackend(t)
	f.detected = []lichclient.DetectedProvider{
		{ID: "claude", Name: "Claude Code", Installed: true},
		{ID: "codex", Name: "Codex", Installed: true},
		{ID: "crush", Name: "Crush"},
	}
	f.settings["provider.codex.enabled@"] = "1"
	f.settings["provider.claude.enabled@"] = "0"
	f.settings["provider.default@"] = "crush"
	f.settings["provider.default@p"] = "codex"
	m := newModel(c, func() {})
	m.projects = []lichclient.Project{{ID: "p"}}
	m.providers.noteProjects(m.projects)
	if err := m.providers.reload(context.Background()); err != nil {
		t.Fatal(err)
	}
	offered := m.launchProviders()
	if len(offered) != 1 || offered[0].id != "codex" || offered[0].name != "Codex" {
		t.Fatalf("offered %+v", offered)
	}
	if got := m.newSessionKind(); got != "codex" {
		t.Fatalf("implicit kind = %q", got)
	}
}
