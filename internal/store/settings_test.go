package store

import (
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

func TestSettingGlobalAndProjectScope(t *testing.T) {
	svc := newTestStore(t)

	if got, err := svc.GetSetting(t.Context(), api.GetSettingOptions{Key: claudeBinKey, ProjectID: globalScope}); err != nil || got != "" {
		t.Fatalf("missing setting = (%q, %v), want (\"\", nil)", got, err)
	}

	if _, err := svc.SetSetting(t.Context(), api.SetSettingOptions{Key: claudeBinKey, ProjectID: globalScope, Value: "/usr/bin/claude"}); err != nil {
		t.Fatalf("SetSetting global: %v", err)
	}
	if _, err := svc.SetSetting(t.Context(), api.SetSettingOptions{Key: claudeBinKey, ProjectID: "p1", Value: "/opt/claude-dev"}); err != nil {
		t.Fatalf("SetSetting project: %v", err)
	}

	// Global and project rows coexist under the same key.
	if got, _ := svc.GetSetting(t.Context(), api.GetSettingOptions{Key: claudeBinKey, ProjectID: globalScope}); got != "/usr/bin/claude" {
		t.Errorf("global = %q", got)
	}
	if got, _ := svc.GetSetting(t.Context(), api.GetSettingOptions{Key: claudeBinKey, ProjectID: "p1"}); got != "/opt/claude-dev" {
		t.Errorf("project = %q", got)
	}

	// Upsert overwrites in place rather than duplicating.
	if _, err := svc.SetSetting(t.Context(), api.SetSettingOptions{Key: claudeBinKey, ProjectID: globalScope, Value: "/usr/local/bin/claude"}); err != nil {
		t.Fatalf("SetSetting overwrite: %v", err)
	}
	if got, _ := svc.GetSetting(t.Context(), api.GetSettingOptions{Key: claudeBinKey, ProjectID: globalScope}); got != "/usr/local/bin/claude" {
		t.Errorf("overwritten global = %q", got)
	}
}

// The hotkey bindings are a JSON document rather than the flag or path every
// other setting holds (native/cmd/lich-native/hotkeys.go). Nothing may reshape
// the value on the way through: a binding that comes back re-quoted or trimmed
// is a shortcut that no longer fires, with nothing on screen saying why.
func TestSettingRoundTripsAJSONDocument(t *testing.T) {
	svc := newTestStore(t)

	const key = "hotkeys.bindings"
	const bindings = `{"newSession":{"mod":true,"shift":true,"alt":false,"key":"j"},` +
		`"closeSession":{"mod":false,"shift":false,"alt":false,"key":""}}`

	if _, err := svc.SetSetting(t.Context(), api.SetSettingOptions{Key: key, ProjectID: globalScope, Value: bindings}); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	got, err := svc.GetSetting(t.Context(), api.GetSettingOptions{Key: key, ProjectID: globalScope})
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if got != bindings {
		t.Errorf("round-tripped = %q, want %q", got, bindings)
	}

	// A rebind replaces the whole document; nothing of the old one survives.
	const rebound = `{"newSession":{"mod":true,"shift":true,"alt":false,"key":"y"}}`
	if _, err := svc.SetSetting(t.Context(), api.SetSettingOptions{Key: key, ProjectID: globalScope, Value: rebound}); err != nil {
		t.Fatalf("SetSetting rebind: %v", err)
	}
	if got, _ := svc.GetSetting(t.Context(), api.GetSettingOptions{Key: key, ProjectID: globalScope}); got != rebound {
		t.Errorf("rebound = %q, want %q", got, rebound)
	}
}

func TestClaudeProviderBinResolution(t *testing.T) {
	svc := newTestStore(t)

	// Nothing configured: empty, so the terminal applies its own default.
	if got := svc.ProviderBin(providers.Claude, "p1"); got != "" {
		t.Errorf("unconfigured = %q, want \"\"", got)
	}

	// Global only: every project resolves to it.
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: claudeBinKey, ProjectID: globalScope, Value: "global-claude"})
	if got := svc.ProviderBin(providers.Claude, "p1"); got != "global-claude" {
		t.Errorf("global fallback = %q, want global-claude", got)
	}

	// Project override wins over global for that project only.
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: claudeBinKey, ProjectID: "p1", Value: "p1-claude"})
	if got := svc.ProviderBin(providers.Claude, "p1"); got != "p1-claude" {
		t.Errorf("p1 override = %q, want p1-claude", got)
	}
	if got := svc.ProviderBin(providers.Claude, "p2"); got != "global-claude" {
		t.Errorf("p2 = %q, want global-claude", got)
	}
}

func TestGHAccountForPath(t *testing.T) {
	svc := newTestStore(t)
	if _, err := svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "lich", Path: "/repo/lich"}); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	// A worktree lives outside the project directory; only its session row ties
	// the path back to the project.
	if _, err := svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s1", Label: "fix", Kind: "claude", Path: "/data/worktrees/p1/fix", NextSeq: 2}); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	if _, err := svc.SetSetting(t.Context(), api.SetSettingOptions{Key: ghAccountKey, ProjectID: "p1", Value: "octocat"}); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{"project directory", "/repo/lich", "octocat"},
		{"worktree of the project", "/data/worktrees/p1/fix", "octocat"},
		{"unknown path", "/elsewhere", ""},
		{"empty path", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := svc.GHAccountForPath(tt.path); got != tt.want {
				t.Errorf("GHAccountForPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}

	// A project with no account configured resolves to "", which runs gh as its
	// own active account.
	if _, err := svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p2", Name: "other", Path: "/repo/other"}); err != nil {
		t.Fatalf("AddProject p2: %v", err)
	}
	if got := svc.GHAccountForPath("/repo/other"); got != "" {
		t.Errorf("unconfigured project = %q, want \"\"", got)
	}
}

func TestProviderBinResolution(t *testing.T) {
	svc := newTestStore(t)

	// A non-claude provider is keyed by "provider.<id>.bin", independent of the
	// legacy claude.bin key, with the same project-over-global fallback.
	if got := svc.ProviderBin("codex", "p1"); got != "" {
		t.Errorf("unconfigured codex = %q, want \"\"", got)
	}
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider.codex.bin", ProjectID: globalScope, Value: "global-codex"})
	if got := svc.ProviderBin("codex", "p1"); got != "global-codex" {
		t.Errorf("global codex = %q, want global-codex", got)
	}
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider.codex.bin", ProjectID: "p1", Value: "p1-codex"})
	if got := svc.ProviderBin("codex", "p1"); got != "p1-codex" {
		t.Errorf("p1 codex override = %q, want p1-codex", got)
	}

	// Claude routes through the legacy key, so both scopes read the same value.
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: claudeBinKey, ProjectID: globalScope, Value: "global-claude"})
	if got := svc.ProviderBin("claude", "p2"); got != "global-claude" {
		t.Errorf("ProviderBin claude = %q, want global-claude", got)
	}
}

// TestProviderBinParkedLayer pins what a switched-off layer does: it resolves as
// if it were unset, while the path it holds stays in the store — which is the
// whole point of the switch, since deleting the path was the old way to do this.
func TestProviderBinParkedLayer(t *testing.T) {
	svc := newTestStore(t)
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider.codex.bin", ProjectID: globalScope, Value: "global-codex"})
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider.codex.bin", ProjectID: "p1", Value: "p1-codex"})

	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider.codex.bin.off", ProjectID: "p1", Value: "true"})
	if got := svc.ProviderBin("codex", "p1"); got != "global-codex" {
		t.Errorf("parked project layer = %q, want global-codex", got)
	}
	if got, _ := svc.GetSetting(t.Context(), api.GetSettingOptions{Key: "provider.codex.bin", ProjectID: "p1"}); got != "p1-codex" {
		t.Errorf("parked path = %q, want it kept as p1-codex", got)
	}

	// Parking the global layer too leaves the provider's own default.
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider.codex.bin.off", ProjectID: globalScope, Value: "true"})
	if got := svc.ProviderBin("codex", "p1"); got != "" {
		t.Errorf("both layers parked = %q, want \"\"", got)
	}
	// A project scope parked while global runs still reads the global value.
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider.codex.bin.off", ProjectID: globalScope, Value: "false"})
	if got := svc.ProviderBin("codex", "p2"); got != "global-codex" {
		t.Errorf("unparked global = %q, want global-codex", got)
	}

	// Anything but the literal "true" is a layer nobody switched off — the key
	// is absent for every override configured before it existed.
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider.codex.bin.off", ProjectID: "p1", Value: "1"})
	if got := svc.ProviderBin("codex", "p1"); got != "p1-codex" {
		t.Errorf("non-\"true\" off value = %q, want p1-codex", got)
	}

	// Claude parks through the legacy key's own suffix.
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: claudeBinKey, ProjectID: globalScope, Value: "global-claude"})
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: claudeBinKey + ".off", ProjectID: globalScope, Value: "true"})
	if got := svc.ProviderBin("claude", "p1"); got != "" {
		t.Errorf("parked claude = %q, want \"\"", got)
	}
}

// TestSkipPermissionsIsScopedByCheckout pins the two scopes apart: the flag a
// session in the project's own directory reads is not the one a session in a
// worktree reads, so turning it on for throwaway checkouts leaves the main
// working tree prompting.
func TestSkipPermissionsIsScopedByCheckout(t *testing.T) {
	svc := newTestStore(t)
	if _, err := svc.AddProject(t.Context(), api.AddProjectOptions{ID: "p1", Name: "alpha", Path: "/repo/alpha"}); err != nil {
		t.Fatalf("AddProject: %v", err)
	}

	// Nothing configured: both scopes prompt.
	if svc.SkipPermissions(providers.Claude, "p1", "/repo/alpha") {
		t.Error("unconfigured project directory skips permissions, want prompts")
	}
	if svc.SkipPermissions(providers.Claude, "p1", "/repo/alpha-feature") {
		t.Error("unconfigured worktree skips permissions, want prompts")
	}

	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider.claude.skip-permissions.worktree", ProjectID: globalScope, Value: "true"})
	if !svc.SkipPermissions(providers.Claude, "p1", "/repo/alpha-feature/") {
		t.Error("worktree does not skip permissions, want it on")
	}
	if svc.SkipPermissions(providers.Claude, "p1", "/repo/alpha/") {
		t.Error("project directory follows the worktree flag, want its own")
	}

	// A project lich cannot place answers with the main-checkout flag, never the
	// worktree one it just read as true.
	if svc.SkipPermissions(providers.Claude, "gone", "/repo/alpha-feature") {
		t.Error("unknown project reads the worktree flag, want the main-checkout one")
	}

	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider.claude.skip-permissions", ProjectID: globalScope, Value: "true"})
	if !svc.SkipPermissions(providers.Claude, "p1", "/repo/alpha") {
		t.Error("project directory does not skip permissions, want it on")
	}

	// Only the literal "true" arms it — any other stored value prompts.
	_, _ = svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider.claude.skip-permissions", ProjectID: globalScope, Value: "1"})
	if svc.SkipPermissions(providers.Claude, "p1", "/repo/alpha") {
		t.Error("value \"1\" skips permissions, want only \"true\" to")
	}
}

func TestProviderEnabledReadsTheSwitchLikeTheWindows(t *testing.T) {
	svc := newTestStore(t)
	cases := []struct {
		provider, stored string
		want             bool
	}{
		{"claude", "", true},
		{"claude", "0", false},
		{"antigravity", "", false},
		{"antigravity", "1", true},
		{"codex", "0", false},
	}
	for _, c := range cases {
		if _, err := svc.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider." + c.provider + ".enabled", ProjectID: globalScope, Value: c.stored}); err != nil {
			t.Fatal(err)
		}
		if got, err := svc.ProviderEnabled(c.provider); err != nil || got != c.want {
			t.Errorf("%s stored %q: got %v, %v; want %v", c.provider, c.stored, got, err, c.want)
		}
	}
}
