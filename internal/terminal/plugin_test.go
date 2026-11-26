package terminal

import (
	"encoding/json"
	"github.com/lichdotdev/lich/internal/library"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/internal/prompt"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/relay"
)

// Every provider is answered: the four that take lich's release per session
// get it the way each was measured to load it, and the four that run the
// plugin installed in their config get nothing on their argv.
func TestPluginArgsForEveryProvider(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "0.23.0")
	want := map[string][]string{
		providers.Claude:      {"--plugin-dir=" + dir},
		providers.Cursor:      {"--plugin-dir=" + dir},
		providers.OMP:         {"-e", filepath.Join(dir, "omp", "lich.js")},
		providers.Codex:       nil,
		providers.Antigravity: nil,
		providers.OpenCode:    nil,
		providers.Crush:       nil,
		providers.Kiro:        nil,
	}
	for _, p := range providers.Registry {
		w, ok := want[p.ID]
		if !ok {
			t.Errorf("%s is not decided here", p.ID)
			continue
		}
		if got := pluginArgs(p.ID, dir); !slices.Equal(got, w) {
			t.Errorf("pluginArgs(%s) = %v, want %v", p.ID, got, w)
		}
		if got := pluginArgs(p.ID, ""); got != nil {
			t.Errorf("pluginArgs(%s) with no release = %v, want nothing", p.ID, got)
		}
	}
}

// Claude Code's --mcp-config takes a list and stays last; the plugin rides
// in one token before it, so neither swallows the other.
func TestProviderArgsLoadsThePluginBeforeTheMCPConfig(t *testing.T) {
	args := providerArgs(providers.Claude, "", "", "", "", "/usr/bin/lich", "", "/p/0.23.0", library.Delivery{}, false, false, false, relay.RouteCards, prompt.English)
	plugin := slices.Index(args, "--plugin-dir=/p/0.23.0")
	mcp := slices.Index(args, claudeMCPFlag)
	if plugin < 0 || mcp < 0 || plugin > mcp || mcp != len(args)-2 {
		t.Fatalf("args = %v, want the plugin before --mcp-config, which stays last", args)
	}
}

// oh-my-pi's chat is its ACP server, whose root takes -e before the subcommand.
func TestChatArgsLoadsTheOMPExtension(t *testing.T) {
	got := chatArgs(spawnArgs{kind: providers.OMP, plugin: "/p/0.23.0"}, ChatLaunch{})
	want := []string{"-e", filepath.Join("/p/0.23.0", "omp", "lich.js"), acpSubcmd}
	if !slices.Equal(got, want) {
		t.Fatalf("chatArgs(omp) = %v, want %v", got, want)
	}
	if got := chatArgs(spawnArgs{kind: providers.Cursor, plugin: "/p/0.23.0"}, ChatLaunch{}); !slices.Equal(got, []string{acpSubcmd}) {
		t.Errorf("chatArgs(cursor) = %v, want the bare ACP server, which runs no plugin hooks", got)
	}
}

// opencode is handed the module through its inline config, beside the
// plugins the person lists there themselves.
func TestPluginEnvAddsTheModuleToOpencodesInlineConfig(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "0.23.0")
	module := fileURL(filepath.Join(dir, "opencode", "lich.js"))
	env := []string{"A=1", opencodeConfigContentVar + `={"plugin":["mine@1"],"theme":"x"}`}
	got := pluginEnv(env, providers.OpenCode, dir)
	var config map[string]any
	if err := json.Unmarshal([]byte(lastEnvValue(got, opencodeConfigContentVar)), &config); err != nil {
		t.Fatal(err)
	}
	plugins, _ := config["plugin"].([]any)
	if len(plugins) != 2 || plugins[0] != "mine@1" || plugins[1] != module || config["theme"] != "x" {
		t.Errorf("inline config = %v, want the person's plugin and setting kept and %q added", config, module)
	}
	for _, kind := range []string{providers.Claude, providers.OMP} {
		if got := pluginEnv(env, kind, dir); !slices.Equal(got, env) {
			t.Errorf("pluginEnv(%s) = %v, want the environment untouched", kind, got)
		}
	}
	if got := pluginEnv(env, providers.OpenCode, ""); !slices.Equal(got, env) {
		t.Errorf("pluginEnv with no release = %v, want the environment untouched", got)
	}
}

// An inline config that is not a JSON object is the person's to fix: the
// session goes without the plugin rather than without its spawn.
func TestPluginEnvLeavesAnInlineConfigItCannotExtend(t *testing.T) {
	env := []string{opencodeConfigContentVar + "=not json"}
	if got := pluginEnv(env, providers.OpenCode, "/p"); !slices.Equal(got, env) {
		t.Errorf("pluginEnv = %v, want it untouched", got)
	}
}

func TestFileURL(t *testing.T) {
	if got := fileURL("/home/u/lich.js"); got != "file:///home/u/lich.js" {
		t.Errorf("fileURL = %q", got)
	}
	if got := fileURL(`C:\Users\u\lich.js`); !strings.HasPrefix(got, "file:///C:") {
		t.Errorf("fileURL of a drive path = %q, want file:///C:...", got)
	}
}
