package terminal

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/internal/codexprofile"
	"github.com/lichdotdev/lich/internal/prompt"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/relay"
)

func codexHomeWith(t *testing.T, config string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(codexprofile.HomeVar, dir)
	if config == "" {
		return
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCodexBriefingFollowsLanguage(t *testing.T) {
	codexHomeWith(t, "")
	for _, lang := range prompt.Langs {
		args := codexBriefingArgs(providers.Codex, "", relay.RouteSessions, lang)
		want := []string{"-c", "developer_instructions='" + relay.SpawnBriefing(lang, true, relay.RouteSessions) + "'"}
		if !slices.Equal(args, want) {
			t.Errorf("%s: args = %q, want %q", lang, args, want)
		}
	}
}

func TestCodexBriefingIsForBirthsOfCodexOnly(t *testing.T) {
	codexHomeWith(t, "")
	if args := codexBriefingArgs(providers.Codex, "conv-1", relay.RouteSessions, prompt.English); args != nil {
		t.Errorf("resume: args = %q", args)
	}
	if args := codexBriefingArgs(providers.Claude, "", relay.RouteSessions, prompt.English); args != nil {
		t.Errorf("claude: args = %q", args)
	}
}

// The override is a TOML literal string, which holds neither an apostrophe nor
// a line break.
func TestCodexBriefingFitsTOMLLiteral(t *testing.T) {
	for _, lang := range prompt.Langs {
		briefing := relay.SpawnBriefing(lang, true, relay.RouteSessions)
		if strings.ContainsAny(briefing, "'\n\r") {
			t.Errorf("%s: briefing cannot be a TOML literal string: %q", lang, briefing)
		}
	}
}

func TestCodexBriefingLeavesUsersOwnInstructions(t *testing.T) {
	cases := []struct {
		name, config string
		briefed      bool
	}{
		{"unset", "model = \"gpt-5\"\n", true},
		{"set", "model = \"gpt-5\"\ndeveloper_instructions = \"mine\"\n", false},
		{"in a table", "[profiles.x]\ndeveloper_instructions = \"mine\"\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			codexHomeWith(t, tc.config)
			args := codexBriefingArgs(providers.Codex, "", relay.RouteSessions, prompt.English)
			if got := len(args) > 0; got != tc.briefed {
				t.Errorf("briefed = %v, want %v: %q", got, tc.briefed, args)
			}
		})
	}
}

func TestKiroBriefingEnvFollowsLanguage(t *testing.T) {
	for _, lang := range prompt.Langs {
		env := briefingEnv([]string{"A=1"}, providers.Kiro, relay.RouteSessions, lang)
		want := relay.BriefingEnv + "=" + relay.SpawnBriefing(lang, true, relay.RouteSessions)
		if !slices.Contains(env, want) || !slices.Contains(env, "A=1") {
			t.Errorf("%s: env = %q", lang, env)
		}
	}
}

func TestBriefingEnvIsKiroOnly(t *testing.T) {
	for _, kind := range []string{providers.Claude, providers.Codex, providers.Antigravity, providers.OpenCode, providers.OMP, providers.Crush, providers.Cursor, KindShell} {
		if env := briefingEnv([]string{"A=1"}, kind, relay.RouteSessions, prompt.English); len(env) != 1 {
			t.Errorf("%s: env = %q", kind, env)
		}
	}
}
