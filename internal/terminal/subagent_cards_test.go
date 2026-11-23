package terminal

import (
	"github.com/lichdotdev/lich/api"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/internal/prompt"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/relay"
)

// withClaudePlugin points Claude Code's config at a directory whose plugin
// state says version is installed, "" for none.
func withClaudePlugin(t *testing.T, version string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	if version == "" {
		return
	}
	state := `{"plugins":{"lich@lich-plugin":[{"scope":"user","version":"` + version + `"}]}}`
	if err := os.MkdirAll(filepath.Join(dir, "plugins"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugins", "installed_plugins.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The lich-plugin mod reads LICH_SUBAGENT_DEPTH on every Claude Code session to
// tell a worker. A mod older than the subagent hook also reads
// LICH_SUBAGENT_CARDS=off where its subagents stay native; a newer one has
// lich decide at /subagent and is told nothing more.
func TestSubagentEnv(t *testing.T) {
	t.Parallel()
	base := []string{"A=1"}
	cases := []struct {
		name     string
		kind     string
		depth    int
		cardsOn  bool
		asksLich bool
		want     []string
	}{
		{"claude with cards on", providers.Claude, 0, true, false, []string{"A=1", "LICH_SUBAGENT_DEPTH=0"}},
		{"a worker that opens cards", providers.Claude, 1, true, false, []string{"A=1", "LICH_SUBAGENT_DEPTH=1"}},
		{"a worker kept native under an older mod", providers.Claude, 2, false, false,
			[]string{"A=1", "LICH_SUBAGENT_DEPTH=2", "LICH_SUBAGENT_CARDS=off"}},
		{"a worker kept native under a mod that asks lich", providers.Claude, 2, false, true,
			[]string{"A=1", "LICH_SUBAGENT_DEPTH=2"}},
		{"another provider", providers.Codex, 1, false, false, []string{"A=1"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := subagentEnv(slices.Clone(base), c.kind, c.depth, c.cardsOn, c.asksLich)
			if !slices.Equal(got, c.want) {
				t.Errorf("subagentEnv = %v, want %v", got, c.want)
			}
		})
	}
}

// A spawn is briefed for where its subagents actually run: as cards, kept
// native in a card, or inside a harness whose hooks do not run.
func TestSubagentRoute(t *testing.T) {
	withClaudePlugin(t, "0.19.0")
	cases := []struct {
		name    string
		kind    string
		view    string
		depth   int
		cardsOn bool
		want    relay.SubagentRoute
	}{
		{"a session whose subagents become cards", providers.Claude, "", 0, true, relay.RouteCards},
		{"a worker whose subagents become cards", providers.Claude, "", 1, true, relay.RouteCards},
		{"a worker kept native", providers.Claude, "", 2, false, relay.RouteNative},
		{"a session the user kept native", providers.Claude, "", 0, false, relay.RouteSessions},
		{"a Codex session", providers.Codex, api.ViewChat, 0, true, relay.RouteCards},
		{"a Codex worker kept native", providers.Codex, "", 2, false, relay.RouteNative},
		{"Cursor CLI in a terminal", providers.Cursor, api.ViewTerminal, 0, true, relay.RouteCards},
		{"Cursor CLI's chat, which runs no hooks", providers.Cursor, api.ViewChat, 0, true, relay.RouteSessions},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := subagentRoute(c.kind, c.view, "", c.depth, c.cardsOn); got != c.want {
				t.Errorf("subagentRoute = %v, want %v", got, c.want)
			}
		})
	}
}

// Without a plugin whose mod opens cards, nothing is told about the Agent tool.
func TestSubagentRouteWithoutTheCardPlugin(t *testing.T) {
	withClaudePlugin(t, "0.14.0")
	if got := subagentRoute(providers.Claude, "", "", 1, false); got != relay.RouteSessions {
		t.Errorf("subagentRoute = %v, want the sessions route", got)
	}
}

// A spawn whose subagents become lich cards is briefed to fan out through its
// own subagent tool; every other spawn keeps the line against it.
func TestBriefingFollowsSubagentCards(t *testing.T) {
	t.Parallel()
	cards := providerArgs(providers.Claude, "", "", "", "", "/usr/bin/lich", "", false, false, false, relay.RouteCards, prompt.English)
	plain := providerArgs(providers.Claude, "", "", "", "", "/usr/bin/lich", "", false, false, false, relay.RouteSessions, prompt.English)
	briefing := func(args []string) string {
		at := slices.Index(args, "--append-system-prompt")
		if at < 0 || at+1 >= len(args) {
			t.Fatalf("no briefing in %v", args)
		}
		return args[at+1]
	}
	if !strings.Contains(briefing(cards), "subagent tool") {
		t.Errorf("a card spawn is not pointed at its subagent tool:\n%s", briefing(cards))
	}
	if strings.Contains(briefing(plain), "subagent tool") {
		t.Errorf("a spawn without cards is pointed at its subagent tool:\n%s", briefing(plain))
	}
}

// Any provider's worker opens cards one level down: only Claude Code has a
// plugin old enough to tell a worker by something other than its depth.
func TestAnotherProvidersWorkerOpensCards(t *testing.T) {
	if !subagentCardsOn(providers.Codex, "", 1) || subagentCardsOn(providers.Codex, "", 2) {
		t.Error("a Codex worker's cards do not follow subagent.MaxDepth")
	}
}
