//go:build !windows

package terminal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/providers"
)

// Unix-only with the stubBins it reads, which live in the PTY suite.

// A worker opens cards one level down, and only under a plugin that tells a
// worker by its depth; a card at maxSubagentDepth keeps its own subagents
// native whatever the setting says.
func TestSubagentCardsOnFollowsTheDepth(t *testing.T) {
	cases := []struct {
		name   string
		bins   stubBins
		plugin string
		wantOn bool
	}{
		{"setting on", stubBins{}, "0.19.0", true},
		{"setting off", stubBins{subagentCardsOff: true}, "0.19.0", false},
		{"a worker", stubBins{subagentDepth: 1}, "0.19.0", true},
		{"a worker under an older plugin", stubBins{subagentDepth: 1}, "0.18.4", false},
		{"a worker with the setting off", stubBins{subagentDepth: 1, subagentCardsOff: true}, "0.19.0", false},
		{"a worker's worker", stubBins{subagentDepth: 2}, "0.19.0", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withClaudePlugin(t, c.plugin)
			svc := New(c.bins, nil, events.New())
			depth := c.bins.SessionSubagentDepth("s1")
			if got := svc.subagentCardsOn(providers.Claude, "", depth); got != c.wantOn {
				t.Errorf("subagentCardsOn = %v, want %v", got, c.wantOn)
			}
		})
	}
}

// A fork of a worker's conversation carries the worker's depth onto its own
// row before it spawns, so the copy opens cards exactly as the worker would.
func TestForkOfASubagentKeepsItsDepth(t *testing.T) {
	withClaudePlugin(t, "0.19.0")
	store := stubBins{
		bin:               stayAliveBin(t),
		subagentParents:   map[string]int{"conv-worker": maxSubagentDepth},
		inheritedSubagent: map[string]int{},
	}
	svc := New(store, nil, events.New())
	t.Cleanup(func() { _, _ = svc.Close(t.Context(), api.SessionIDOptions{ID: "s2"}) })

	if _, err := svc.Start(t.Context(), api.StartOptions{ID: "s2", ProjectID: "p1", Cwd: t.TempDir(), Kind: providers.Claude, Resume: "conv-worker", Fork: true, Cols: 80, Rows: 24}); err != nil {
		t.Fatalf("Start = %v, want nil", err)
	}
	if got := store.inheritedSubagent["s2"]; got != maxSubagentDepth {
		t.Errorf("the fork's row is %d deep, want the worker's %d", got, maxSubagentDepth)
	}
	if svc.subagentCardsOn(providers.Claude, "", store.SessionSubagentDepth("s2")) {
		t.Error("the fork of a worker at the limit opens cards, want its subagents native")
	}
}

// A session in a profile opens a worker's cards by the plugin that profile
// holds, not by Personal's.
func TestSubagentCardsOnReadsTheProfilesPlugin(t *testing.T) {
	withClaudePlugin(t, "0.19.0")
	profile := t.TempDir()
	if err := os.MkdirAll(filepath.Join(profile, "plugins"), 0o755); err != nil {
		t.Fatal(err)
	}
	state := `{"plugins":{"lich@lich-plugin":[{"scope":"user","version":"0.18.4"}]}}`
	if err := os.WriteFile(filepath.Join(profile, "plugins", "installed_plugins.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := New(stubBins{subagentDepth: 1}, nil, events.New())

	if !svc.subagentCardsOn(providers.Claude, "", 1) {
		t.Error("Personal on 0.19.0: want a worker's cards on")
	}
	if svc.subagentCardsOn(providers.Claude, profile, 1) {
		t.Error("a profile on 0.18.4 beside a Personal on 0.19.0: want a worker's subagents native")
	}
}
