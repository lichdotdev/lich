package library

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// memSettings is the store's settings, in memory.
type memSettings map[string]string

func (m memSettings) GetSetting(_ context.Context, opts api.GetSettingOptions) (string, error) {
	return m[opts.Key], nil
}

func (m memSettings) SetSetting(_ context.Context, opts api.SetSettingOptions) (api.None, error) {
	m[opts.Key] = opts.Value
	return api.None{}, nil
}

// rig is a library over a temporary home and config directory, every
// provider's own config under that home.
type rig struct {
	*Service
	home, config string
	events       *[]string
}

func newRig(t *testing.T) rig {
	t.Helper()
	home, config := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	for _, v := range []string{"CLAUDE_CONFIG_DIR", "CODEX_HOME", "OMP_PROFILE", "PI_CODING_AGENT_DIR", "KIRO_HOME"} {
		t.Setenv(v, "")
	}
	var events []string
	s := New(memSettings{}, func(name string, _ any) { events = append(events, name) }, nil, nil)
	s.configDir = func() (string, error) { return config, nil }
	s.home = func() (string, error) { return home, nil }
	return rig{Service: s, home: home, config: config, events: &events}
}

func writeSkill(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + name + "\ndescription: Does " + name + ".\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(path, skillFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, _ := json.Marshal(v)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// One skill found in two providers is one row naming both; lich's own and the
// copies lich wrote are never offered.
func TestFoundSkillsAreOneRowPerName(t *testing.T) {
	r := newRig(t)
	writeSkill(t, filepath.Join(r.home, ".claude", "skills"), "pdf")
	writeSkill(t, filepath.Join(r.home, ".gemini", "skills"), "pdf")
	writeSkill(t, filepath.Join(r.home, ".claude", "skills"), "lich-theme")
	ours := writeSkill(t, filepath.Join(r.home, ".gemini", "skills"), "deploy")
	if err := os.WriteFile(filepath.Join(ours, deliveredMarker), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	lib, err := r.List(t.Context(), api.None{})
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Found) != 1 {
		t.Fatalf("found %+v, want the pdf skill alone", lib.Found)
	}
	f := lib.Found[0]
	// Crush reads ~/.claude/skills too, so it holds the skill as well.
	if f.Name != "pdf" || !f.New || !slices.Contains(f.Providers, providers.Claude) || !slices.Contains(f.Providers, providers.Antigravity) {
		t.Errorf("found %+v, want pdf from Claude Code and Antigravity, new", f)
	}
	if _, err := r.Seen(t.Context(), api.LibrarySeenOptions{Kind: api.LibrarySkill}); err != nil {
		t.Fatal(err)
	}
	lib, _ = r.List(t.Context(), api.None{})
	if lib.Found[0].New {
		t.Error("a skill seen on the last visit is still new")
	}
}

// Move in copies the skill into the library, turns it off where it was
// without deleting a thing, and turns it on in lich for those providers.
func TestMoveInASkill(t *testing.T) {
	r := newRig(t)
	claude := writeSkill(t, filepath.Join(r.home, ".claude", "skills"), "pdf")
	agents := filepath.Join(r.home, ".agents", "skills")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(agents, "pdf")
	if err := os.Symlink(claude, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if _, err := r.MoveIn(t.Context(), api.LibraryItemOptions{Kind: api.LibrarySkill, Name: "pdf"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(r.config, "lich", "library", "skills", "pdf", skillFile)); err != nil {
		t.Errorf("the skill is not in the library: %v", err)
	}
	if _, err := os.Stat(filepath.Join(claude, skillFile+movedSuffix)); err != nil {
		t.Errorf("the folder's SKILL.md was not set aside: %v", err)
	}
	if _, err := os.Lstat(link + movedSuffix); err != nil {
		t.Errorf("the link was not set aside: %v", err)
	}
	if _, err := os.Stat(filepath.Join(claude, "SKILL.md")); !os.IsNotExist(err) {
		t.Error("Claude Code would still load the skill where it was")
	}
	for _, p := range []string{providers.Claude, providers.Codex, providers.Crush} {
		if !r.On(api.LibrarySkill, "pdf", p, "") {
			t.Errorf("not on for %s, which had it", p)
		}
	}
	if r.On(api.LibrarySkill, "pdf", providers.Kiro, "") {
		t.Error("on for Kiro CLI, which never had it")
	}
	if !slices.Contains(*r.events, api.LibraryChangedEvent.Name) {
		t.Error("no library.changed after the move")
	}
	lib, _ := r.List(t.Context(), api.None{})
	if len(lib.Found) != 0 || len(lib.Skills) != 1 {
		t.Errorf("found %+v, skills %+v, want the skill in the library alone", lib.Found, lib.Skills)
	}
}

// An MCP server moves into the library and out of the key its provider
// reads, kept beside it.
func TestMoveInAnMCPServer(t *testing.T) {
	r := newRig(t)
	path := filepath.Join(r.home, ".cursor", "mcp.json")
	writeJSON(t, path, map[string]any{"mcpServers": map[string]any{
		"postgres": map[string]any{"command": "npx", "args": []string{"pg"}},
		"lich":     map[string]any{"command": "/usr/bin/lich", "args": []string{"mcp"}},
	}})
	if _, err := r.MoveIn(t.Context(), api.LibraryItemOptions{Kind: api.LibraryMCP, Name: "postgres"}); err != nil {
		t.Fatal(err)
	}
	doc, _ := readDoc(path)
	servers, _ := doc["mcpServers"].(map[string]any)
	moved, _ := doc[movedKey("mcpServers")].(map[string]any)
	if _, ok := servers["postgres"]; ok || moved["postgres"] == nil || servers["lich"] == nil {
		t.Errorf("document = %v, want postgres set aside and lich's own left", doc)
	}
	lib, _ := r.List(t.Context(), api.None{})
	if len(lib.MCP) != 1 || lib.MCP[0].Command != "npx pg" || lib.MCP[0].Transport != "stdio" {
		t.Errorf("library mcp = %+v", lib.MCP)
	}
}

// The provider's switch speaks for every profile until a profile's own is
// set, which then speaks for that profile alone, either way.
func TestSwitchesPerProviderAndProfile(t *testing.T) {
	r := newRig(t)
	set := func(provider, profile string, on bool) {
		if _, err := r.Set(t.Context(), api.LibrarySwitchOptions{Kind: api.LibrarySkill, Name: "pdf", Provider: provider, Profile: profile, On: on}); err != nil {
			t.Fatal(err)
		}
	}
	on := func(profile string) bool { return r.On(api.LibrarySkill, "pdf", providers.Claude, profile) }
	if on("") || on("work") {
		t.Error("on before any switch")
	}
	set(providers.Claude, "", true)
	if !on("work") || !on("personal") {
		t.Error("the provider's switch did not reach its profiles")
	}
	set(providers.Claude, "work", false)
	if on("work") || !on("personal") {
		t.Error("turning Work off reached Personal, or left Work on")
	}
	set(providers.Claude, "", false)
	set(providers.Claude, "work", true)
	if !on("work") || on("personal") {
		t.Error("Work's own switch did not keep it on alone")
	}
	if _, err := r.Set(t.Context(), api.LibrarySwitchOptions{Kind: api.LibrarySkill, Name: "pdf", Provider: "shell", On: true}); err == nil {
		t.Error("a switch for no provider was stored")
	}
}

// Each provider with profiles says whether its delivery keeps a switch per
// profile, kind by kind: the ones lich writes one file or folder for share
// one switch.
func TestOneSwitchWhereProfilesShareTheFile(t *testing.T) {
	want := map[string][2]string{
		providers.Claude:      {"", ""},
		providers.Codex:       {api.LibrarySharedFile, ""},
		providers.Antigravity: {"", ""},
		providers.OpenCode:    {"", ""},
		providers.OMP:         {"", api.LibrarySharedFile},
		providers.Crush:       {"", ""},
		providers.Cursor:      {"", api.LibrarySharedFile},
		providers.Kiro:        {api.LibrarySharedFile, ""},
	}
	r := newRig(t)
	r.profiles = func(string) []api.Profile { return []api.Profile{{Slug: "personal"}, {Slug: "work"}} }
	lib, err := r.List(t.Context(), api.None{})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range lib.Providers {
		if got := [2]string{p.OneSkills, p.OneMCP}; got != want[p.ID] {
			t.Errorf("%s one switch = %v, want %v", p.ID, got, want[p.ID])
		}
	}
}

// A provider whose profiles share the file takes the provider's switch for
// every profile, whatever a profile's own says.
func TestSharedDeliveryFollowsTheProvidersSwitch(t *testing.T) {
	r := newRig(t)
	writeSkill(t, filepath.Join(r.config, "lich", "library", "skills"), "pdf")
	for _, sw := range []api.LibrarySwitchOptions{
		{Kind: api.LibrarySkill, Name: "pdf", Provider: providers.Codex, On: true},
		{Kind: api.LibrarySkill, Name: "pdf", Provider: providers.Codex, Profile: "work", On: false},
	} {
		if _, err := r.Set(t.Context(), sw); err != nil {
			t.Fatal(err)
		}
	}
	codex := filepath.Join(r.home, ".codex")
	r.Deliver(providers.Codex, "work", codex)
	if _, err := os.Stat(filepath.Join(codex, "skills", "pdf", deliveredMarker)); err != nil {
		t.Errorf("Codex's shared skills folder lost the skill to a profile's switch: %v", err)
	}
}

// Every provider is handed an on skill by its own route, and nothing for an
// item that is off.
func TestDeliverEveryProvider(t *testing.T) {
	for _, p := range providers.Registry {
		t.Run(p.ID, func(t *testing.T) {
			if p.ID == providers.Crush && runtime.GOOS == "windows" {
				t.Skip("Crush's composed folder needs symlinks, which Windows hands out to few accounts (docs/ceilings.md)")
			}
			r := newRig(t)
			writeSkill(t, filepath.Join(r.config, "lich", "library", "skills"), "pdf")
			writeSkill(t, filepath.Join(r.config, "lich", "library", "skills"), "unused")
			writeJSON(t, filepath.Join(r.config, "lich", "library", mcpFile), map[string]any{"mcpServers": map[string]any{
				"postgres": map[string]any{"command": "npx", "args": []string{"pg"}},
			}})
			for _, item := range [][2]string{{api.LibrarySkill, "pdf"}, {api.LibraryMCP, "postgres"}} {
				if _, err := r.Set(t.Context(), api.LibrarySwitchOptions{Kind: item[0], Name: item[1], Provider: p.ID, On: true}); err != nil {
					t.Fatal(err)
				}
			}

			d := r.Deliver(p.ID, "", filepath.Join(r.home, ".codex"))

			if got := skillsHanded(t, r, p.ID, d); !slices.Equal(got, []string{"pdf"}) {
				t.Errorf("skills handed %v, want pdf alone", got)
			}
			if got := serversHanded(t, r, p.ID, d); noMCP(p.ID) == "" && !slices.Equal(got, []string{"postgres"}) {
				t.Errorf("servers handed %v, want postgres", got)
			} else if noMCP(p.ID) != "" && len(got) != 0 {
				t.Errorf("servers handed %v to a provider lich says takes none", got)
			}
		})
	}
}

// skillsHanded are the skill folder names provider's session would load.
func skillsHanded(t *testing.T, r rig, provider string, d Delivery) []string {
	t.Helper()
	var dir string
	switch provider {
	case providers.Claude, providers.Cursor, providers.OpenCode, providers.OMP:
		dir = d.SkillsDir
		if provider == providers.OMP {
			data, err := os.ReadFile(d.OMPConfig)
			_, encoded, ok := strings.Cut(string(data), "    - ")
			var configuredDir string
			decodeErr := json.Unmarshal([]byte(strings.TrimSpace(encoded)), &configuredDir)
			if err != nil || !ok || decodeErr != nil || configuredDir != d.SkillsDir {
				t.Errorf("omp overlay %q (read err %v, decode err %v) does not name %s", data, err, decodeErr, d.SkillsDir)
			}
		}
		if provider == providers.Claude || provider == providers.Cursor {
			if _, err := os.Stat(filepath.Join(d.PluginDir, ".claude-plugin", "plugin.json")); err != nil {
				t.Errorf("no plugin manifest in %s", d.PluginDir)
			}
		}
	case providers.Crush:
		dir = d.CrushSkills
	default:
		dir = r.deliveryDir(provider, filepath.Join(r.home, ".codex"))
	}
	var names []string
	for _, sk := range readSkills(dir) {
		names = append(names, sk.name)
	}
	return names
}

// serversHanded are the server names provider's session would get.
func serversHanded(t *testing.T, r rig, provider string, d Delivery) []string {
	t.Helper()
	var names []string
	switch provider {
	case providers.OMP, providers.Cursor:
		for _, doc := range r.mcpDocs(provider) {
			config, _ := readDoc(doc.path)
			for name := range config[doc.key].(map[string]any) {
				names = append(names, name)
			}
		}
	default:
		for name := range d.MCP {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

// A skill turned off is taken back out of the folders lich wrote into, and a
// folder the person made under that name is never touched.
func TestWrittenSkillsFollowTheSwitch(t *testing.T) {
	r := newRig(t)
	writeSkill(t, filepath.Join(r.config, "lich", "library", "skills"), "pdf")
	writeSkill(t, filepath.Join(r.config, "lich", "library", "skills"), "mine")
	theirs := writeSkill(t, filepath.Join(r.home, ".gemini", "skills"), "mine")
	for _, name := range []string{"pdf", "mine"} {
		if _, err := r.Set(t.Context(), api.LibrarySwitchOptions{Kind: api.LibrarySkill, Name: name, Provider: providers.Antigravity, On: true}); err != nil {
			t.Fatal(err)
		}
	}
	r.Deliver(providers.Antigravity, "", "")
	dir := filepath.Join(r.home, ".gemini", "skills")
	if _, err := os.Stat(filepath.Join(dir, "pdf", deliveredMarker)); err != nil {
		t.Fatalf("pdf not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(theirs, deliveredMarker)); !os.IsNotExist(err) {
		t.Error("lich wrote over a skill the person made")
	}
	if _, err := r.Set(t.Context(), api.LibrarySwitchOptions{Kind: api.LibrarySkill, Name: "pdf", Provider: providers.Antigravity}); err != nil {
		t.Fatal(err)
	}
	r.Deliver(providers.Antigravity, "", "")
	if _, err := os.Stat(filepath.Join(dir, "pdf")); !os.IsNotExist(err) {
		t.Error("a skill turned off is still written")
	}
}

// The servers lich wrote into a document follow the switch; the person's own
// entry of the same name stays theirs.
func TestWrittenServersFollowTheSwitch(t *testing.T) {
	r := newRig(t)
	writeJSON(t, filepath.Join(r.config, "lich", "library", mcpFile), map[string]any{"mcpServers": map[string]any{
		"postgres": map[string]any{"command": "npx"}, "notes": map[string]any{"command": "notes"},
	}})
	path := filepath.Join(r.home, ".cursor", "mcp.json")
	writeJSON(t, path, map[string]any{"mcpServers": map[string]any{"notes": map[string]any{"command": "theirs"}}})
	for _, name := range []string{"postgres", "notes"} {
		if _, err := r.Set(t.Context(), api.LibrarySwitchOptions{Kind: api.LibraryMCP, Name: name, Provider: providers.Cursor, On: true}); err != nil {
			t.Fatal(err)
		}
	}
	r.Deliver(providers.Cursor, "", "")
	doc, _ := readDoc(path)
	servers := doc["mcpServers"].(map[string]any)
	if servers["postgres"] == nil || servers["notes"].(map[string]any)["command"] != "theirs" {
		t.Fatalf("document %v, want postgres added and their notes kept", servers)
	}
	if _, err := r.Set(t.Context(), api.LibrarySwitchOptions{Kind: api.LibraryMCP, Name: "postgres", Provider: providers.Cursor}); err != nil {
		t.Fatal(err)
	}
	r.Deliver(providers.Cursor, "", "")
	doc, _ = readDoc(path)
	servers = doc["mcpServers"].(map[string]any)
	if servers["postgres"] != nil || servers["notes"] == nil {
		t.Errorf("document %v, want postgres gone and their notes kept", servers)
	}
}

// lich's own plugin is the release sessions run and what it brings.
func TestPluginCountsWhatTheReleaseBrings(t *testing.T) {
	r := newRig(t)
	if p := r.plugin(); p.Version != "" {
		t.Errorf("plugin with no release = %+v", p)
	}
	dir := filepath.Join(t.TempDir(), "0.23.0")
	writeJSON(t, filepath.Join(dir, "hooks", "hooks.json"), map[string]any{"hooks": map[string]any{
		"SessionStart": []any{map[string]any{"hooks": []any{map[string]any{"command": "a"}}}},
		"PreToolUse":   []any{map[string]any{"hooks": []any{map[string]any{"command": "b"}, map[string]any{"command": "c"}}}},
	}})
	writeSkill(t, filepath.Join(dir, "skills"), "theme")
	r.pluginDir = func() string { return dir }
	if p := r.plugin(); p != (api.LibraryPlugin{Version: "0.23.0", Hooks: 3, Skills: 1, MCP: 1}) {
		t.Errorf("plugin = %+v", p)
	}
}

// What the providers hold changes the fingerprint the watcher compares, and
// nothing else does.
func TestFingerprintFollowsWhatTheProvidersHold(t *testing.T) {
	r := newRig(t)
	before := r.fingerprint()
	if r.fingerprint() != before {
		t.Fatal("the fingerprint moved with nothing changed")
	}
	writeSkill(t, filepath.Join(r.home, ".cursor", "skills"), "pdf")
	if r.fingerprint() == before {
		t.Error("a skill installed in Cursor did not change the fingerprint")
	}
}

// What cannot be moved in or switched is refused, and nothing is written.
func TestRefusals(t *testing.T) {
	r := newRig(t)
	for _, opts := range []api.LibraryItemOptions{{Kind: api.LibrarySkill, Name: "absent"}, {Kind: api.LibraryMCP, Name: "absent"}, {Kind: "plugin", Name: "x"}} {
		if _, err := r.MoveIn(t.Context(), opts); err == nil {
			t.Errorf("MoveIn(%+v) = nil, want an error", opts)
		}
	}
	if _, err := r.Set(t.Context(), api.LibrarySwitchOptions{Kind: "plugin", Name: "x", Provider: providers.Claude}); err == nil {
		t.Error("a switch for no kind was stored")
	}
	writeSkill(t, filepath.Join(r.home, ".cursor", "skills"), "pdf")
	writeSkill(t, filepath.Join(r.config, "lich", "library", "skills"), "pdf")
	if lib, _ := r.List(t.Context(), api.None{}); len(lib.Found) != 0 {
		t.Errorf("found %+v, want a name the library has left out", lib.Found)
	}
}
