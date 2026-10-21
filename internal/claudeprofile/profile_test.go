package claudeprofile

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/internal/profilestore"
)

// machine points every directory this package resolves at a scratch home: the
// user's config dir (XDG on Linux, APPDATA on Windows, the home on macOS) and
// Personal's ~/.claude under it. It returns Personal's directory.
func machine(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData"))
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", "")
	t.Setenv("LICH_DEV", "")
	personal := filepath.Join(home, ".claude")
	mustMkdir(t, personal)
	return personal
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, data string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readJSON(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

const globalConfig = `{
  "oauthAccount": {"emailAddress": "me@example.com"},
  "userID": "u1",
  "cachedGrowthBookFeatures": {},
  "passesEligibilityCache": {},
  "hasCompletedOnboarding": true,
  "theme": "dark",
  "mcpServers": {"tool": {"command": "tool"}},
  "projects": {
    "/work": {
      "hasTrustDialogAccepted": true,
      "allowedTools": ["Bash"],
      "enabledMcpjsonServers": ["db"],
      "lastSessionId": "abc",
      "lastCost": 1.5,
      "exampleFiles": ["a.go"]
    }
  }
}`

func TestCreateLaysTheProfileOutAgainstPersonal(t *testing.T) {
	personal := machine(t)
	home := filepath.Dir(personal)
	mustWrite(t, filepath.Join(personal, "settings.json"), `{"model":"opus"}`)
	mustMkdir(t, filepath.Join(personal, "agents"))
	mustWrite(t, filepath.Join(personal, ".credentials.json"), `{"secret":true}`)
	mustWrite(t, filepath.Join(home, ".claude.json"), globalConfig)
	mustWrite(t, filepath.Join(personal, "plugins", "known_marketplaces.json"), `{
	  "official": {"source": {"source": "github"}, "installLocation": "`+filepath.ToSlash(filepath.Join(home, "elsewhere", "plugins", "marketplaces", "official"))+`"},
	  "mine": {"source": {"source": "directory"}, "installLocation": "`+filepath.ToSlash(filepath.Join(home, "src", "mine"))+`"}
	}`)

	p, err := Create("Work Account")
	if err != nil {
		t.Fatal(err)
	}
	if p.Slug != "work-account" || p.Name != "Work Account" {
		t.Fatalf("created %+v", p)
	}
	for _, name := range []string{"agents", filepath.Join("plugins", "marketplaces")} {
		if !profilestore.IsLink(filepath.Join(p.Dir, name)) {
			t.Errorf("%s is not linked to Personal", name)
		}
	}
	if cache := filepath.Join(p.Dir, "plugins", "cache"); profilestore.IsLink(cache) || !isDir(cache) {
		t.Error("the plugin cache is not a directory of the profile's own")
	}
	if slices.Contains(p.Copied, "settings.json") {
		// A Windows without Developer Mode: the documented fallback is a copy.
		if _, err := os.Stat(filepath.Join(p.Dir, "settings.json")); err != nil {
			t.Errorf("settings.json neither linked nor copied: %v", err)
		}
	} else {
		assertWritesThrough(t, filepath.Join(p.Dir, "settings.json"), filepath.Join(personal, "settings.json"))
	}

	var global map[string]any
	readJSON(t, filepath.Join(p.Dir, ".claude.json"), &global)
	for _, gone := range []string{"oauthAccount", "userID", "cachedGrowthBookFeatures", "passesEligibilityCache"} {
		if _, ok := global[gone]; ok {
			t.Errorf("global config kept %s", gone)
		}
	}
	for _, kept := range []string{"hasCompletedOnboarding", "theme", "mcpServers"} {
		if _, ok := global[kept]; !ok {
			t.Errorf("global config dropped %s", kept)
		}
	}
	project := global["projects"].(map[string]any)["/work"].(map[string]any)
	var keys []string
	for k := range project {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	if want := []string{"allowedTools", "enabledMcpjsonServers", "hasTrustDialogAccepted"}; !slices.Equal(keys, want) {
		t.Errorf("project keys %v, want %v", keys, want)
	}

	var markets map[string]struct {
		InstallLocation string `json:"installLocation"`
	}
	readJSON(t, filepath.Join(p.Dir, "plugins", "known_marketplaces.json"), &markets)
	if got, want := markets["official"].InstallLocation, filepath.Join(p.Dir, "plugins", "marketplaces", "official"); got != want {
		t.Errorf("clone location %q, want %q", got, want)
	}
	if got, want := markets["mine"].InstallLocation, filepath.ToSlash(filepath.Join(home, "src", "mine")); got != want {
		t.Errorf("local marketplace moved to %q, want %q", got, want)
	}
	if len(installedPlugins(filepath.Join(p.Dir, "plugins"))) != 0 {
		t.Error("a new profile has plugins installed")
	}

	profiles, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 || !profiles[0].Personal || profiles[0].Dir != personal || profiles[1].Slug != p.Slug {
		t.Errorf("listed %+v", profiles)
	}
}

// With CLAUDE_CODE_PLUGIN_CACHE_DIR set, Personal's marketplaces are under it,
// so a new profile seeds from and links to those rather than to a plugins
// directory under Personal's config dir that Personal no longer reads.
func TestCreateSeedsFromPersonalsMovedPlugins(t *testing.T) {
	personal := machine(t)
	moved := t.TempDir()
	t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", moved)
	mustMkdir(t, filepath.Join(moved, "marketplaces", "official"))
	mustWrite(t, filepath.Join(moved, "known_marketplaces.json"), `{
	  "official": {"source": {"source": "github"}, "installLocation": "`+filepath.ToSlash(filepath.Join(moved, "marketplaces", "official"))+`"}
	}`)

	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(p.Dir, "plugins", "marketplaces")
	mustWrite(t, filepath.Join(moved, "marketplaces", "probe"), "moved")
	if got, err := os.ReadFile(filepath.Join(link, "probe")); err != nil || string(got) != "moved" {
		t.Errorf("read through the marketplaces link: %q, %v; want the moved marketplaces", got, err)
	}
	var markets map[string]struct {
		InstallLocation string `json:"installLocation"`
	}
	readJSON(t, filepath.Join(p.Dir, "plugins", "known_marketplaces.json"), &markets)
	if got, want := markets["official"].InstallLocation, filepath.Join(link, "official"); got != want {
		t.Errorf("clone location %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(personal, "plugins")); !os.IsNotExist(err) {
		t.Errorf("a plugins directory Personal does not read was created under its config dir: %v", err)
	}
}

func TestSlugsAreFreeAndNeverPersonal(t *testing.T) {
	machine(t)
	cases := []struct{ name, want string }{
		{"Work", "work"},
		{"work", "work-2"},
		{"Personal", "personal-2"},
		{"!!!", "profile"},
		{strings.Repeat("a", 50), strings.Repeat("a", 32)},
	}
	for _, c := range cases {
		p, err := Create(c.name)
		if err != nil {
			t.Fatal(err)
		}
		if p.Slug != c.want {
			t.Errorf("Create(%q) slug %q, want %q", c.name, p.Slug, c.want)
		}
	}
	if _, err := Create("   "); err == nil {
		t.Error("a blank name was accepted")
	}
}

func TestRenameKeepsTheDirectory(t *testing.T) {
	machine(t)
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Update(p.Slug, profilestore.Update{Name: "Client"}); err != nil {
		t.Fatal(err)
	}
	profiles, _ := List()
	if profiles[1].Name != "Client" || profiles[1].Dir != p.Dir {
		t.Errorf("after rename %+v, want name Client at %s", profiles[1], p.Dir)
	}
	if _, err := Update(p.Slug, profilestore.Update{Name: " "}); err == nil {
		t.Error("a blank name was accepted")
	}
}

func TestRenamingPersonalNeverTouchesItsConfig(t *testing.T) {
	personal := machine(t)
	if _, err := Update(Personal, profilestore.Update{Name: "Mine"}); err != nil {
		t.Fatal(err)
	}
	profiles, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if profiles[0].Name != "Mine" || !profiles[0].Personal || profiles[0].Dir != personal {
		t.Errorf("after rename %+v, want Personal named Mine at %s", profiles[0], personal)
	}
	if _, err := os.Stat(personal + ".json"); !os.IsNotExist(err) {
		t.Errorf("the rename wrote beside Personal's config: %v", err)
	}
	if _, err := Update(Personal, profilestore.Update{Name: " "}); err == nil {
		t.Error("a blank name was accepted")
	}
}

func TestDeleteNeverReachesPersonal(t *testing.T) {
	personal := machine(t)
	mustWrite(t, filepath.Join(personal, "settings.json"), `{}`)
	mustWrite(t, filepath.Join(personal, "plugins", "cache", "p", "file"), "x")
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	if err := Delete(p.Slug); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.Dir); !os.IsNotExist(err) {
		t.Errorf("profile dir still there: %v", err)
	}
	for _, kept := range []string{"settings.json", filepath.Join("plugins", "cache", "p", "file")} {
		if _, err := os.Stat(filepath.Join(personal, kept)); err != nil {
			t.Errorf("deleting the profile took Personal's %s: %v", kept, err)
		}
	}
	if _, err := ConfigDir(p.Slug); !errors.Is(err, ErrProfileGone) {
		t.Errorf("ConfigDir of a deleted profile: %v, want ErrProfileGone", err)
	}
	if err := Delete(Personal); err == nil {
		t.Error("Personal was deleted")
	}
}

func TestDirRefusesAnythingButASlug(t *testing.T) {
	machine(t)
	for _, slug := range []string{"../x", "a/b", "", "Upper", Personal, "-x"} {
		if _, err := Dir(slug); err == nil {
			t.Errorf("Dir(%q) accepted", slug)
		}
	}
}

func TestEnsureSharedLinksWhatPersonalGainedLater(t *testing.T) {
	personal := machine(t)
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(personal, "CLAUDE.md"), "rules")
	mustWrite(t, filepath.Join(p.Dir, "settings.json"), `{"own":true}`)
	mustWrite(t, filepath.Join(personal, "settings.json"), `{}`)
	if err := EnsureShared(p.Slug); err != nil {
		t.Fatal(err)
	}
	if !profilestore.IsLink(filepath.Join(p.Dir, "CLAUDE.md")) {
		t.Error("CLAUDE.md created after the profile was not linked")
	}
	if data, _ := os.ReadFile(filepath.Join(p.Dir, "settings.json")); string(data) != `{"own":true}` {
		t.Errorf("the profile's own settings.json was replaced: %s", data)
	}
	if err := EnsureShared(Personal); err != nil {
		t.Errorf("Personal: %v", err)
	}
	if err := EnsureShared("gone"); !errors.Is(err, ErrProfileGone) {
		t.Errorf("missing profile: %v", err)
	}
}

func TestEnvHandsTheProfileItsOwnLogin(t *testing.T) {
	got := Env([]string{
		"PATH=/bin", "CLAUDE_CODE_OAUTH_TOKEN=t", "ANTHROPIC_API_KEY=k", "ANTHROPIC_AUTH_TOKEN=a",
		"CLAUDE_SECURESTORAGE_CONFIG_DIR=s", "CLAUDE_CONFIG_DIR=/old",
	}, "/new")
	if want := []string{"PATH=/bin", "CLAUDE_CONFIG_DIR=/new"}; !slices.Equal(got, want) {
		t.Errorf("Env = %v, want %v", got, want)
	}
}

// CLAUDE_CODE_PLUGIN_CACHE_DIR moves Claude Code's whole plugins directory
// (installed set, marketplaces, cache), so one inherited by a profile's spawn
// would hand every profile Personal's plugins.
func TestEnvKeepsTheProfilesOwnPlugins(t *testing.T) {
	got := Env([]string{"PATH=/bin", "CLAUDE_CODE_PLUGIN_CACHE_DIR=/shared"}, "/new")
	if want := []string{"PATH=/bin", "CLAUDE_CONFIG_DIR=/new"}; !slices.Equal(got, want) {
		t.Errorf("Env = %v, want %v", got, want)
	}
}

func TestPersonalPluginsFollowLichsEnvironment(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for _, c := range []struct{ name, value, want string }{
		{"unset", "", filepath.Join("/cfg", "plugins")},
		{"absolute", "/elsewhere", "/elsewhere"},
		{"home", "~/p", filepath.Join(home, "p")},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("CLAUDE_CODE_PLUGIN_CACHE_DIR", c.value)
			if got := PluginsDir(Personal, "/cfg"); got != c.want {
				t.Errorf("PluginsDir = %q, want %q", got, c.want)
			}
			if got, want := PluginsDir("work", "/work"), filepath.Join("/work", "plugins"); got != want {
				t.Errorf("a profile's PluginsDir = %q, want %q", got, want)
			}
		})
	}
}

func TestPersonalFollowsLichsEnvironment(t *testing.T) {
	machine(t)
	custom := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", custom)
	dir, err := ConfigDir("")
	if err != nil || dir != custom {
		t.Errorf("ConfigDir(\"\") = %q, %v, want %q", dir, err, custom)
	}
	file, err := personalGlobalFile()
	if err != nil || file != filepath.Join(custom, ".claude.json") {
		t.Errorf("global file %q, %v", file, err)
	}
}

func TestDevRigKeepsItsOwnProfiles(t *testing.T) {
	machine(t)
	real, err := Root()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("LICH_DEV", "1")
	dev, err := Root()
	if err != nil {
		t.Fatal(err)
	}
	if real == dev {
		t.Errorf("a dev rig shares %s with the installed lich", dev)
	}
}

// assertWritesThrough checks that a write to link lands in target, the link
// being what keeps an edit made under one account visible under the others.
func assertWritesThrough(t *testing.T, link, target string) {
	t.Helper()
	if !profilestore.IsLink(link) {
		t.Errorf("%s is not a link", link)
		return
	}
	if err := os.WriteFile(link, []byte(`{"model":"haiku"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(target); string(data) != `{"model":"haiku"}` {
		t.Errorf("a write through %s left %s at %s", link, target, data)
	}
}
