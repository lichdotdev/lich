//go:build !windows

package terminal

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/prompt"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/relay"
	"github.com/lichdotdev/lich/internal/subagent"
)

// Unix-only with the stubBins it reads, which live in the PTY suite.

// cardSetup is a user's setup as every provider reads it: a server of their
// own config, one of lich's library, and a plugin.
type cardSetup struct{}

func (cardSetup) CardItems(string) []api.SubagentCardItem {
	return []api.SubagentCardItem{
		{ID: "mcp:ai-memory", Kind: api.SubagentBringMCP, Name: "ai-memory", Origin: ".claude.json"},
		{ID: "mcp:github", Kind: api.SubagentBringMCP, Name: "github", Origin: api.SubagentOriginLibrary},
		{ID: "plugin:ponytail@ponytail", Kind: api.SubagentBringPlugin, Name: "ponytail", Origin: "ponytail"},
	}
}

type cardPicks map[string][]string

func (cardPicks) SubagentCards(string) bool                   { return true }
func (cardPicks) SetSubagentCards(string, bool) error         { return nil }
func (p cardPicks) SubagentBrings(id string) []string         { return p[id] }
func (cardPicks) SetSubagentBring(string, string, bool) error { return nil }
func (cardPicks) SessionKind(string) string                   { return "" }

// spawned is the argv and environment one spawn of kind would run with.
func spawned(t *testing.T, bins stubBins, kind string, picks cardPicks) ptySpec {
	t.Helper()
	svc := New(bins, nil, events.New())
	svc.SetCards(subagent.NewCards(picks, cardSetup{}, nil).Card)
	svc.mu.Lock()
	defer svc.mu.Unlock()
	l, err := svc.prepareSpawnLocked(api.StartOptions{ID: "s1", ProjectID: "p1", Cwd: t.TempDir(), Kind: kind}, "", "", false, ptyArgs)
	if err != nil {
		t.Fatalf("prepare %s: %v", kind, err)
	}
	return l.spec
}

// briefingOf is the briefing a spawn hands its provider, wherever that
// provider takes it.
func briefingOf(sp ptySpec) string {
	args, env := sp.args, sp.env
	if at := slices.Index(args, "--append-system-prompt"); at >= 0 {
		return args[at+1]
	}
	for _, a := range args {
		if v, ok := strings.CutPrefix(a, codexInstructionsKey+"="); ok {
			return v
		}
	}
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, relay.BriefingEnv+"="); ok {
			return v
		}
	}
	return ""
}

// With a provider's cards off its briefing, at any depth, keeps the subagent
// native and pushes no delegation: neither the card text nor the line that
// sends fan-out to lich sessions.
func TestCardsOffBriefsNoCards(t *testing.T) {
	withClaudePlugin(t, "0.19.0")
	text := prompt.For(prompt.English)
	cardText := text.BriefingCards
	for _, kind := range []string{providers.Claude, providers.OMP, providers.Codex, providers.Kiro} {
		for _, depth := range []int{0, 1} {
			t.Run(kind, func(t *testing.T) {
				on := briefingOf((spawned(t, stubBins{subagentDepth: depth}, kind, nil)))
				off := briefingOf((spawned(t, stubBins{subagentDepth: depth, cardsOff: map[string]bool{kind: true}}, kind, nil)))
				if !strings.Contains(on, cardText) {
					t.Errorf("depth %d, cards on: briefing %q lacks the card text", depth, on)
				}
				if !strings.Contains(off, text.BriefingOff) {
					t.Errorf("depth %d, cards off: briefing %q, want the cards-off text", depth, off)
				}
				for _, pushed := range []string{text.BriefingCards, text.BriefingNative, text.BriefingSessions} {
					if strings.Contains(off, pushed) {
						t.Errorf("depth %d, cards off: briefing carries %q", depth, pushed)
					}
				}
			})
		}
	}
}

// claudeSettings is the --settings document of a Claude Code argv, nil for
// none.
func claudeSettings(t *testing.T, args []string) map[string]any {
	t.Helper()
	at := slices.Index(args, claudeSettingsFlag)
	if at < 0 {
		return nil
	}
	doc := map[string]any{}
	if err := json.Unmarshal([]byte(args[at+1]), &doc); err != nil {
		t.Fatalf("--settings %q: %v", args[at+1], err)
	}
	return doc
}

// leanOf is what a spawn leaves out, provider by provider, in one comparable
// shape.
func leanOf(t *testing.T, sp ptySpec) []string {
	t.Helper()
	args, env := sp.args, sp.env
	var out []string
	if doc := claudeSettings(t, args); doc != nil {
		if plugins, ok := doc["enabledPlugins"].(map[string]any); ok {
			for id := range plugins {
				out = append(out, "plugin-off:"+id)
			}
		}
		if denied, ok := doc["deniedMcpServers"].([]any); ok {
			for _, d := range denied {
				out = append(out, "mcp-denied:"+d.(map[string]any)["serverName"].(string))
			}
		}
	}
	for _, flag := range ompAlsoFlags {
		if slices.Contains(args, flag) {
			out = append(out, flag)
		}
	}
	for _, e := range env {
		switch {
		case e == claudeDisableMemoryVar+"=1", e == opencodeDisableClaudeVar+"=1":
			out = append(out, e)
		case strings.HasPrefix(e, opencodeConfigContentVar+"="):
			var config map[string]map[string]map[string]any
			_ = json.Unmarshal([]byte(strings.TrimPrefix(e, opencodeConfigContentVar+"=")), &config)
			for name, sv := range config[opencodeMCPKey] {
				if sv["enabled"] == false {
					out = append(out, "mcp-off:"+name)
				}
			}
		}
	}
	slices.Sort(out)
	return out
}

// A card with nothing picked starts as lean as its provider can; one the
// user opened, nobody's subagent, starts with everything.
func TestACardStartsLean(t *testing.T) {
	withClaudePlugin(t, "0.19.0")
	want := map[string][]string{
		providers.Claude:   {"CLAUDE_CODE_DISABLE_AUTO_MEMORY=1", "mcp-denied:ai-memory", "mcp-denied:github", "plugin-off:ponytail@ponytail"},
		providers.OpenCode: {"OPENCODE_DISABLE_CLAUDE_CODE=1", "mcp-off:ai-memory"},
		providers.OMP:      {"--no-extensions", "--no-rules", "--no-skills"},
	}
	for _, p := range providers.Registry {
		t.Run(p.ID, func(t *testing.T) {
			if got := leanOf(t, (spawned(t, stubBins{subagentDepth: 1}, p.ID, nil))); !slices.Equal(got, want[p.ID]) {
				t.Errorf("a card leaves out %v, want %v", got, want[p.ID])
			}
			if got := leanOf(t, (spawned(t, stubBins{}, p.ID, nil))); len(got) != 0 {
				t.Errorf("a session the user opened leaves out %v, want nothing", got)
			}
		})
	}
}

// What the user picks rides the card's spawn.
func TestPickedItemsReachTheSpawn(t *testing.T) {
	withClaudePlugin(t, "0.19.0")
	picks := cardPicks{
		providers.Claude:   {"mcp:ai-memory", api.SubagentAlsoMemory},
		providers.OpenCode: {"mcp:ai-memory", api.SubagentAlsoClaudeSetup},
		providers.OMP:      {api.SubagentAlsoSkills},
	}
	want := map[string][]string{
		providers.Claude:   {"mcp-denied:github", "plugin-off:ponytail@ponytail"},
		providers.OpenCode: nil,
		providers.OMP:      {"--no-extensions", "--no-rules"},
	}
	for kind, w := range want {
		t.Run(kind, func(t *testing.T) {
			if got := leanOf(t, (spawned(t, stubBins{subagentDepth: 1}, kind, picks))); !slices.Equal(got, w) {
				t.Errorf("a card leaves out %v, want %v", got, w)
			}
		})
	}
}

// A card's spawn records and announces what it brought; a session nobody
// opened as a subagent says nothing.
func TestACardReportsWhatItBrought(t *testing.T) {
	bins := stubBins{brought: map[string][]string{}}
	hub := events.New()
	svc := New(bins, nil, hub)
	svc.reportBrought("card", true, subagent.Card{Lean: true, Brought: []string{"mcp:ai-memory"}})
	svc.reportBrought("session", false, subagent.Card{})
	if got := bins.brought["card"]; !slices.Equal(got, []string{"mcp:ai-memory"}) {
		t.Errorf("recorded %v for the card", got)
	}
	if _, ok := bins.brought["session"]; ok {
		t.Error("recorded a list for a session that is no card")
	}
}
