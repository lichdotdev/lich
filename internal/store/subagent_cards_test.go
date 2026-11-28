package store

import (
	"slices"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// Cards are on until the user turns them off, and back on again; each
// provider's switch is its own.
func TestSubagentCardsSwitch(t *testing.T) {
	svc := newTestStore(t)
	if !svc.SubagentCards(providers.Claude) {
		t.Fatal("cards off before anyone touched them")
	}
	if err := svc.SetSubagentCards(providers.Claude, false); err != nil {
		t.Fatal(err)
	}
	if svc.SubagentCards(providers.Claude) || !svc.SubagentCards(providers.Codex) {
		t.Error("the switch did not stay with its provider")
	}
	if err := svc.SetSubagentCards(providers.Claude, true); err != nil {
		t.Fatal(err)
	}
	if !svc.SubagentCards(providers.Claude) {
		t.Error("cards did not come back on")
	}
}

// A card brings nothing until an item is picked; picks are kept sorted and
// once, and an unreadable value brings nothing.
func TestSubagentBrings(t *testing.T) {
	svc := newTestStore(t)
	if got := svc.SubagentBrings(providers.Claude); got != nil {
		t.Fatalf("brings %v before any pick", got)
	}
	for _, it := range []string{"mcp:b", "mcp:a", "mcp:b"} {
		if err := svc.SetSubagentBring(providers.Claude, it, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.SetSubagentBring(providers.Claude, "mcp:a", false); err != nil {
		t.Fatal(err)
	}
	if got := svc.SubagentBrings(providers.Claude); !slices.Equal(got, []string{"mcp:b"}) {
		t.Errorf("brings %v, want mcp:b alone", got)
	}
	if _, err := svc.SetSetting(t.Context(), api.SetSettingOptions{Key: subagentBringsKey(providers.OMP), Value: "{not json"}); err != nil {
		t.Fatal(err)
	}
	if got := svc.SubagentBrings(providers.OMP); got != nil {
		t.Errorf("an unreadable value brings %v", got)
	}
}

// What a card brought rides its row, for the tooltip a reload redraws.
func TestSessionBroughtRoundTrip(t *testing.T) {
	svc := sandboxProject(t, "/work/alpha")
	if _, err := svc.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s1", Label: "one", Kind: providers.Claude}); err != nil {
		t.Fatalf("AddSession: %v", err)
	}
	want := []string{"mcp:ai-memory", "memory"}
	if err := svc.SetSessionBrought("s1", want); err != nil {
		t.Fatal(err)
	}
	if got := sessionOf(t, svc, "s1").Brought; !slices.Equal(got, want) {
		t.Errorf("Brought = %v, want %v", got, want)
	}
	if err := svc.SetSessionBrought("s1", nil); err != nil {
		t.Fatal(err)
	}
	if got := sessionOf(t, svc, "s1").Brought; got != nil {
		t.Errorf("Brought = %v after a respawn that brought nothing", got)
	}
}
