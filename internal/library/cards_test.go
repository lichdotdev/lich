package library

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// What a card can leave out is the provider's own servers, the library's
// switched on for it and its plugins switched on; lich's own never.
func TestCardItems(t *testing.T) {
	r := newRig(t)
	writeJSON(t, filepath.Join(r.home, ".claude.json"), map[string]any{"mcpServers": map[string]any{
		"ai-memory": map[string]any{"command": "ai-memory", "args": []string{"mcp"}},
		"lich":      map[string]any{"command": "/usr/bin/lich", "args": []string{"mcp"}},
	}})
	writeJSON(t, filepath.Join(r.config, "lich", "library", mcpFile), map[string]any{"mcpServers": map[string]any{
		"postgres": map[string]any{"command": "npx", "args": []string{"pg"}},
		"unused":   map[string]any{"command": "npx", "args": []string{"x"}},
	}})
	if _, err := r.Set(t.Context(), api.LibrarySwitchOptions{Kind: api.LibraryMCP, Name: "postgres", Provider: providers.Claude, On: true}); err != nil {
		t.Fatal(err)
	}
	installClaude(t, r.home, "ponytail@ponytail", true)

	got := r.CardItems(providers.Claude)
	want := []api.SubagentCardItem{
		{ID: "mcp:ai-memory", Kind: api.SubagentBringMCP, Name: "ai-memory", Origin: ".claude.json"},
		{ID: "mcp:postgres", Kind: api.SubagentBringMCP, Name: "postgres", Origin: api.SubagentOriginLibrary},
		{ID: "plugin:ponytail@ponytail", Kind: api.SubagentBringPlugin, Name: "ponytail", Origin: "ponytail"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("CardItems = %+v\nwant %+v", got, want)
	}
	if got := r.CardItems(providers.OpenCode); len(got) != 0 {
		t.Errorf("opencode has none of it, got %+v", got)
	}
}
