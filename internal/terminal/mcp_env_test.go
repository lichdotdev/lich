package terminal

import (
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/internal/providers"
)

func TestMCPEnvRegistersLichWithOpencode(t *testing.T) {
	t.Parallel()
	env := mcpEnv([]string{"HOME=/h"}, providers.OpenCode, "/usr/bin/lich")

	want := `{"mcp":{"lich":{"command":["/usr/bin/lich","mcp"],"type":"local"}}}`
	if got := lastEnvValue(env, opencodeConfigContentVar); got != want {
		t.Errorf("%s = %s, want %s", opencodeConfigContentVar, got, want)
	}
	if lastEnvValue(env, "HOME") != "/h" {
		t.Error("the rest of the environment was not kept")
	}
	if strings.Contains(lastEnvValue(env, opencodeConfigContentVar), "token") {
		t.Error("a secret reached the registration")
	}
}

func TestMCPEnvLeavesEveryOtherSpawnAlone(t *testing.T) {
	t.Parallel()
	base := []string{"HOME=/h"}
	cases := []struct{ kind, bin string }{
		{providers.OpenCode, ""},
		{providers.Claude, "/usr/bin/lich"},
		{providers.Crush, "/usr/bin/lich"},
		{KindShell, "/usr/bin/lich"},
	}
	for _, c := range cases {
		if env := mcpEnv(slices.Clone(base), c.kind, c.bin); !slices.Equal(env, base) {
			t.Errorf("%s bin=%q: env = %v, want it untouched", c.kind, c.bin, env)
		}
	}
}

// The user's own servers and keys ride the same variable, and the child keeps
// only its last value, so they have to be merged into lich's, not shadowed by it.
func TestMCPEnvExtendsTheUsersOwnInlineConfig(t *testing.T) {
	t.Parallel()
	own := opencodeConfigContentVar + `={"model":"m","mcp":{"mine":{"type":"remote","url":"http://x"}}}`
	unfolded, err := pasteUnfoldEnv([]string{own}, providers.OpenCode, true)
	if err != nil {
		t.Fatalf("pasteUnfoldEnv: %v", err)
	}
	config := opencodeConfig(t, mcpEnv(unfolded, providers.OpenCode, "/usr/bin/lich"))

	if config["model"] != "m" || config["experimental"] == nil {
		t.Errorf("config = %v, want the user's model and the paste switch kept", config)
	}
	servers := config["mcp"].(map[string]any)
	if servers["mine"] == nil || servers["lich"] == nil {
		t.Errorf("mcp = %v, want the user's server beside lich's", servers)
	}
}

// An inline config lich cannot extend costs the session its tools, never its
// spawn: the registration is lich's addition, and the user's config is theirs.
func TestMCPEnvSkipsAnInlineConfigItCannotExtend(t *testing.T) {
	t.Parallel()
	for _, own := range []string{`not json`, `{"mcp":true}`} {
		base := []string{opencodeConfigContentVar + "=" + own}
		if env := mcpEnv(slices.Clone(base), providers.OpenCode, "/usr/bin/lich"); !slices.Equal(env, base) {
			t.Errorf("%s=%s: env = %v, want it untouched", opencodeConfigContentVar, own, env)
		}
	}
}
