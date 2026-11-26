package library

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

func pluginNamed(list []api.LibraryItem, name string) (api.LibraryItem, bool) {
	for _, it := range list {
		if it.Name == name {
			return it, true
		}
	}
	return api.LibraryItem{}, false
}

// installClaude writes a Claude Code plugin install in Personal with its
// user switch.
func installClaude(t *testing.T, home, id string, on bool) {
	t.Helper()
	writeJSON(t, filepath.Join(home, ".claude", "plugins", "installed_plugins.json"), map[string]any{
		"plugins": map[string]any{id: []any{map[string]any{"scope": "user", "version": "3.5.5"}}},
	})
	writeJSON(t, filepath.Join(home, ".claude", "settings.json"), map[string]any{"enabledPlugins": map[string]bool{id: on, "lich@lich-plugin": true}})
}

// Each provider's plugins are listed in the user's scope with its switch;
// lich's own never is.
func TestPluginsPerProvider(t *testing.T) {
	for _, tc := range []struct {
		provider, id string
		setup        func(t *testing.T, home string)
		want         api.LibrarySwitch
	}{
		{providers.Claude, "addon@snk", func(t *testing.T, home string) { installClaude(t, home, "addon@snk", false) },
			api.LibrarySwitch{Provider: providers.Claude}},
		{providers.Codex, "ponytail@ponytail", func(t *testing.T, home string) {
			putFile(t, filepath.Join(home, ".codex", "config.toml"), "[plugins.\"ponytail@ponytail\"]\nenabled = true\n\n[plugins.\"lich@lich-plugin\"]\nenabled = true\n")
		}, api.LibrarySwitch{Provider: providers.Codex, On: true}},
		{providers.Antigravity, "notes", func(t *testing.T, home string) {
			putFile(t, filepath.Join(home, ".gemini", "config", "plugins", "notes", "plugin.json"), `{"version":"1.0.0"}`)
			putFile(t, filepath.Join(home, ".gemini", "config", "plugins", "lich", "plugin.json"), `{}`)
			putFile(t, filepath.Join(home, ".gemini", "config", "config.json"), `{"plugins":{"notes":{"enabled":false}}}`)
		}, api.LibrarySwitch{Provider: providers.Antigravity}},
		{providers.OMP, "@x/caveman", func(t *testing.T, home string) {
			putFile(t, filepath.Join(home, ".omp", "plugins", "omp-plugins.lock.json"), `{"plugins":{"@x/caveman":{"version":"1.0.4","enabled":true},"@x/gone":{"enabled":true}}}`)
			if err := os.MkdirAll(filepath.Join(home, ".omp", "plugins", "node_modules", "@x", "caveman"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, api.LibrarySwitch{Provider: providers.OMP, On: true}},
		{providers.OpenCode, "@x/ponytail", func(t *testing.T, home string) {
			putFile(t, filepath.Join(home, ".config", "opencode", "opencode.jsonc"), "{\n  // mine\n  \"plugin\": [\"@x/ponytail\"]\n}")
		}, api.LibrarySwitch{Provider: providers.OpenCode, On: true, Reason: api.LibraryNoSwitch}},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			r := newRig(t)
			tc.setup(t, r.home)
			lib, err := r.List(t.Context(), api.LibraryListOptions{})
			if err != nil {
				t.Fatal(err)
			}
			it, ok := pluginNamed(lib.Plugins, tc.id)
			if !ok || it.Kind != api.LibraryPluginKind || !slices.Equal(it.Switches, []api.LibrarySwitch{tc.want}) {
				t.Fatalf("plugins %+v, want %s with %+v", lib.Plugins, tc.id, tc.want)
			}
			for _, it := range lib.Plugins {
				if lichPlugin(it.Name) {
					t.Errorf("lich's own plugin is listed: %+v", it)
				}
			}
			if _, ok := pluginNamed(lib.Plugins, "@x/gone"); ok {
				t.Error("a plugin oh-my-pi's lock names but node_modules lacks is listed")
			}
		})
	}
}

// In a project, Claude Code's plugins are what its settings.json and then
// settings.local.json make of Personal's, behind one switch; one the project
// alone enables shows too.
func TestClaudePluginsInAProject(t *testing.T) {
	r := newRig(t)
	installClaude(t, r.home, "addon@snk", false)
	writeJSON(t, filepath.Join(r.home, ".claude", "plugins", "installed_plugins.json"), map[string]any{"plugins": map[string]any{
		"addon@snk": []any{map[string]any{"scope": "user"}}, "local@m": []any{map[string]any{"scope": "user"}},
	}})
	project := t.TempDir()
	writeJSON(t, filepath.Join(project, ".claude", "settings.json"), map[string]any{"enabledPlugins": map[string]bool{"addon@snk": true, "local@m": true}})
	writeJSON(t, filepath.Join(project, ".claude", "settings.local.json"), map[string]any{"enabledPlugins": map[string]bool{"local@m": false}})
	lib, _ := r.List(t.Context(), api.LibraryListOptions{Project: project})
	for id, on := range map[string]bool{"addon@snk": true, "local@m": false} {
		it, ok := pluginNamed(lib.Plugins, id)
		if !ok || !slices.Equal(it.Switches, []api.LibrarySwitch{{Provider: providers.Claude, On: on}}) {
			t.Errorf("%s = %+v, want one switch at %v", id, it, on)
		}
	}
}

// A project that commits settings.local.json shows Claude Code's switch as one
// lich cannot press, and refuses it.
func TestCommittedLocalSettingsAreReadOnly(t *testing.T) {
	r := newRig(t)
	installClaude(t, r.home, "addon@snk", true)
	project := t.TempDir()
	r.answers["git ls-files -- .claude/settings.local.json in "+project] = ".claude/settings.local.json\n"
	lib, _ := r.List(t.Context(), api.LibraryListOptions{Project: project})
	if it, _ := pluginNamed(lib.Plugins, "addon@snk"); len(it.Switches) != 1 || it.Switches[0].Reason != api.LibraryCommitted {
		t.Fatalf("addon@snk = %+v, want a committed switch", it)
	}
	*r.ran = nil
	if _, err := r.Set(t.Context(), api.LibrarySwitchOptions{Kind: api.LibraryPluginKind, Name: "addon@snk", Provider: providers.Claude, Project: project}); err == nil {
		t.Error("Set = nil, want a refusal")
	}
	for _, line := range *r.ran {
		if strings.HasPrefix(line, "claude") {
			t.Errorf("ran %q", line)
		}
	}
}

// A plugin's switch goes through its provider's own CLI, or Codex's
// config.toml, and tells the window once it is written; a provider with no
// switch refuses.
func TestSetPluginThroughTheProvider(t *testing.T) {
	project := filepath.Join(os.TempDir(), "proj")
	for _, tc := range []struct {
		name string
		opts api.LibrarySwitchOptions
		ran  string
	}{
		{"claude user", api.LibrarySwitchOptions{Provider: providers.Claude, Name: "addon@snk"}, "claude plugin disable addon@snk --scope user"},
		{"claude project", api.LibrarySwitchOptions{Provider: providers.Claude, Name: "addon@snk", On: true, Project: project}, "claude plugin enable addon@snk --scope local in " + project},
		{"antigravity", api.LibrarySwitchOptions{Provider: providers.Antigravity, Name: "notes", On: true}, "antigravity plugin enable notes"},
		{"omp", api.LibrarySwitchOptions{Provider: providers.OMP, Name: "@x/caveman"}, "omp plugin disable @x/caveman"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newRig(t)
			tc.opts.Kind = api.LibraryPluginKind
			if _, err := r.Set(t.Context(), tc.opts); err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(*r.ran, tc.ran) {
				t.Errorf("ran %q, want %q", *r.ran, tc.ran)
			}
			if !slices.Contains(*r.events, api.LibraryChangedEvent.Name) {
				t.Error("no library.changed after the switch")
			}
		})
	}
	t.Run("codex", func(t *testing.T) {
		r := newRig(t)
		path := filepath.Join(r.home, ".codex", "config.toml")
		putFile(t, path, "[plugins.\"ponytail@ponytail\"]\nenabled = true\n")
		if _, err := r.Set(t.Context(), api.LibrarySwitchOptions{Kind: api.LibraryPluginKind, Provider: providers.Codex, Name: "ponytail@ponytail"}); err != nil {
			t.Fatal(err)
		}
		if got, _ := os.ReadFile(path); !strings.Contains(string(got), "enabled = false") {
			t.Errorf("config.toml = %q, want it off", got)
		}
	})
	for _, p := range []string{providers.OpenCode, providers.Crush, providers.Cursor, providers.Kiro} {
		t.Run(p, func(t *testing.T) {
			r := newRig(t)
			if _, err := r.Set(t.Context(), api.LibrarySwitchOptions{Kind: api.LibraryPluginKind, Provider: p, Name: "x"}); err == nil {
				t.Error("Set = nil, want a refusal")
			}
		})
	}
}
