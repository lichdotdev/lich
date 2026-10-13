package codexprofile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/omartelo/lich/internal/profilestore"
)

// spawnWith writes Personal's and the profile's config.toml, runs what a spawn
// into the profile runs, and returns the profile's config.toml after it.
func spawnWith(t *testing.T, personal, own string) string {
	t.Helper()
	dir := machine(t)
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "config.toml"), personal)
	mustWrite(t, filepath.Join(p.Dir, "config.toml"), own)
	if err := EnsureShared(p.Slug); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(p.Dir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "" {
		return ""
	}
	if err := EnsureShared(p.Slug); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(filepath.Join(p.Dir, "config.toml"))
	if string(again) != string(data) {
		t.Errorf("a second spawn changed config.toml again:\n%s\nthen\n%s", data, again)
	}
	return string(data)
}

func TestAProfileKeepsItsPluginSetAndTakesPersonalsSettings(t *testing.T) {
	personal := `model = "gpt-5"

[mcp_servers.lich]
command = "lich"

[marketplaces.home]
source_type = "git"
source = "https://example.com/home.git"

[plugins."home@home"]
enabled = true

[features]
goals = true
`
	own := `model = "o3"

[features]
goals = false

[marketplaces.work]
source_type = "git"
source = "https://example.com/work.git"
ref = "v1"

[plugins."tool@work"]
enabled = false
`
	want := `model = "gpt-5"

[mcp_servers.lich]
command = "lich"

[features]
goals = true

[marketplaces.work]
source_type = "git"
source = "https://example.com/work.git"
ref = "v1"

[plugins."tool@work"]
enabled = false
`
	if got := spawnWith(t, personal, own); got != want {
		t.Errorf("config.toml =\n%s\nwant\n%s", got, want)
	}
}

func TestTrustAnswersAreKeptFromBothSides(t *testing.T) {
	personal := `[projects."/shared"]
trust_level = "trusted"

[projects.'/both']
trust_level = "untrusted"

[hooks.state."lich@lich-plugin:hooks/codex-hooks.json:stop:0:0"]
trusted_hash = "sha256:a"
`
	own := `[projects."/mine"]
trust_level = "trusted"

[projects."/both"]
trust_level = "trusted"

[notice]
hide_full_access_warning = true

[[hooks.Stop]]
command = "mine"
`
	want := `[projects."/shared"]
trust_level = "trusted"

[projects.'/both']
trust_level = "untrusted"

[hooks.state."lich@lich-plugin:hooks/codex-hooks.json:stop:0:0"]
trusted_hash = "sha256:a"

[projects."/mine"]
trust_level = "trusted"

[notice]
hide_full_access_warning = true

`
	if got := spawnWith(t, personal, own); got != want {
		t.Errorf("config.toml =\n%s\nwant\n%s", got, want)
	}
}

func TestOnlyARealHeaderStartsATable(t *testing.T) {
	personal := `instructions = """
[plugins."not@aheader"]
still the string \"""
"""
args = [
  ["nested", "]"],
  "[plugins.x]",
]
note = 'a # not a comment [x]' # [plugins.y]

[plugins."real@m"]
enabled = true
`
	want := `instructions = """
[plugins."not@aheader"]
still the string \"""
"""
args = [
  ["nested", "]"],
  "[plugins.x]",
]
note = 'a # not a comment [x]' # [plugins.y]

`
	if got := spawnWith(t, personal, ""); got != want {
		t.Errorf("config.toml =\n%s\nwant\n%s", got, want)
	}
}

func TestNoConfigOnEitherSideWritesNone(t *testing.T) {
	if got := spawnWith(t, "", ""); got != "" {
		t.Errorf("config.toml = %q, want empty", got)
	}
}

func TestAProfileThatLinkedPersonalsConfigGetsItsOwn(t *testing.T) {
	personal := machine(t)
	shared := "model = \"x\"\n\n[plugins.\"home@m\"]\nenabled = true\n"
	mustWrite(t, filepath.Join(personal, "config.toml"), shared)
	mustWrite(t, filepath.Join(personal, "plugins", "cache", "m", "home", "file"), "x")
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(p.Dir, "config.toml")
	cache := filepath.Join(p.Dir, "plugins", "cache")
	if err := os.Remove(config); err != nil {
		t.Fatal(err)
	}
	mustMkdir(t, filepath.Dir(cache))
	if err := os.Symlink(filepath.Join(personal, "config.toml"), config); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	if err := os.Symlink(filepath.Join(personal, "plugins", "cache"), cache); err != nil {
		t.Fatal(err)
	}

	if err := EnsureShared(p.Slug); err != nil {
		t.Fatal(err)
	}
	if profilestore.IsLink(config) || profilestore.IsLink(cache) {
		t.Error("the profile still links Personal's config.toml or plugin cache")
	}
	if data, _ := os.ReadFile(config); string(data) != "model = \"x\"\n\n" {
		t.Errorf("config.toml = %q, want Personal's settings without its plugins", data)
	}
	if data, _ := os.ReadFile(filepath.Join(personal, "config.toml")); string(data) != shared {
		t.Errorf("Personal's config.toml changed to %q", data)
	}
	if _, err := os.Stat(filepath.Join(personal, "plugins", "cache", "m", "home", "file")); err != nil {
		t.Errorf("Personal's plugin cache lost a file: %v", err)
	}
}
