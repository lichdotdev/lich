package claudeprofile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/internal/profilestore"
)

func readSettingsFile(t *testing.T, path string) map[string]any {
	t.Helper()
	var settings map[string]any
	readJSON(t, path, &settings)
	return settings
}

func TestAProfileKeepsItsOwnDefaultModelAndPlugins(t *testing.T) {
	personal := machine(t)
	mustWrite(t, filepath.Join(personal, "settings.json"), `{"model":"opus","effortLevel":"high","theme":"dark"}`)
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	// What `/model`, `/effort` and `claude plugin install` run in the profile
	// leave there.
	mustWrite(t, filepath.Join(p.Dir, "settings.json"), `{
	  "model": "sonnet", "effortLevel": "low", "theme": "dark",
	  "enabledPlugins": {"work@corp": true}, "extraKnownMarketplaces": {"corp": {}}
	}`)
	mustWrite(t, filepath.Join(personal, "settings.json"), `{
	  "model": "haiku", "modelSettings": {"opus": {"effortLevel": "max"}},
	  "hooks": {"Stop": [{"hooks": [{"type": "command", "command": "a && b > c"}]}]},
	  "enabledPlugins": {"mine@home": true}
	}`)

	if err := EnsureShared(p.Slug); err != nil {
		t.Fatal(err)
	}
	got := readSettingsFile(t, filepath.Join(p.Dir, "settings.json"))
	if got["model"] != "sonnet" || got["effortLevel"] != "low" {
		t.Errorf("the profile's own model and effort were replaced: %v", got)
	}
	if _, ok := got["modelSettings"]; ok {
		t.Errorf("Personal's per-model effort reached the profile: %v", got)
	}
	if plugins, _ := got["enabledPlugins"].(map[string]any); len(plugins) != 1 || plugins["work@corp"] != true {
		t.Errorf("enabledPlugins %v, want the profile's own", plugins)
	}
	if markets, _ := got["extraKnownMarketplaces"].(map[string]any); markets["corp"] == nil {
		t.Errorf("the profile's marketplace was dropped: %v", got)
	}
	if _, ok := got["theme"]; ok {
		t.Error("a key Personal removed stayed in the profile")
	}
	if _, ok := got["hooks"]; !ok {
		t.Error("Personal's hooks did not reach the profile")
	}
	if data, _ := os.ReadFile(filepath.Join(p.Dir, "settings.json")); !strings.Contains(string(data), "a && b > c") {
		t.Errorf("a hook command was rewritten: %s", data)
	}
	if mine := readSettingsFile(t, filepath.Join(personal, "settings.json")); mine["model"] != "haiku" {
		t.Errorf("syncing the profile changed Personal's settings: %v", mine)
	}
}

func TestASettingsFileLinkedToPersonalsBecomesTheProfilesOwn(t *testing.T) {
	personal := machine(t)
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	shared := filepath.Join(personal, "settings.json")
	mustWrite(t, shared, `{"model":"opus","enabledPlugins":{"mine@home":true}}`)
	path := filepath.Join(p.Dir, "settings.json")
	if err := os.Symlink(shared, path); err != nil {
		t.Skipf("no file links here: %v", err)
	}

	if err := EnsureShared(p.Slug); err != nil {
		t.Fatal(err)
	}
	if profilestore.IsLink(path) {
		t.Fatal("settings.json is still linked to Personal's")
	}
	if got := readSettingsFile(t, path); got["model"] != "opus" || got["enabledPlugins"] == nil {
		t.Errorf("a profile moved off the shared file lost what it ran on: %v", got)
	}
	mustWrite(t, path, `{"model":"sonnet"}`)
	if got := readSettingsFile(t, shared); got["model"] != "opus" {
		t.Errorf("a profile's /model still reaches Personal: %v", got)
	}
}

func TestWithoutPersonalsSettingsAProfileKeepsItsOwnFile(t *testing.T) {
	personal := machine(t)
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(p.Dir, "settings.json")
	mustWrite(t, path, `{"own":true}`)
	if err := EnsureShared(p.Slug); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); string(data) != `{"own":true}` {
		t.Errorf("the profile's settings.json was rewritten: %s", data)
	}
	if _, err := os.Stat(filepath.Join(personal, "settings.json")); !os.IsNotExist(err) {
		t.Errorf("a profile's sync created Personal's settings.json: %v", err)
	}
}

func TestABrokenSettingsFileFailsTheSync(t *testing.T) {
	personal := machine(t)
	p, err := Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(personal, "settings.json"), `{"model":`)
	if err := EnsureShared(p.Slug); err == nil {
		t.Error("a settings.json that is not JSON synced")
	}
}
