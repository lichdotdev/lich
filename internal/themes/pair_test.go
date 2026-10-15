package themes

import (
	"strings"
	"testing"
)

func TestBundledThemesPairEachOther(t *testing.T) {
	themes, err := NewInDir(t.TempDir()).List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if themes[0].Pair != "dark" || themes[1].Pair != "light" {
		t.Fatalf("bundled pairs = %q, %q; want dark, light", themes[0].Pair, themes[1].Pair)
	}
}

func TestImportKeepsAPair(t *testing.T) {
	s := NewInDir(t.TempDir())
	day := customTheme("forest-day")
	day.Pair = "forest-night"
	if _, err := importTheme(t, s, day); err != nil {
		t.Fatalf("Import: %v", err)
	}
	themes, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := themes[len(themes)-1]; got.ID != "forest-day" || got.Pair != "forest-night" {
		t.Fatalf("listed %q with pair %q, want forest-day paired with forest-night", got.ID, got.Pair)
	}
}

func TestImportRejectsABadPair(t *testing.T) {
	for _, tc := range []struct{ pair, want string }{
		{"forest", "cannot be its own pair"},
		{"Forest Night", "pair: theme id"},
	} {
		theme := customTheme("forest")
		theme.Pair = tc.pair
		_, err := importTheme(t, NewInDir(t.TempDir()), theme)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("pair %q: Import error = %v, want one containing %q", tc.pair, err, tc.want)
		}
	}
}

// A pair of the same scheme cannot be followed to, so the theme is listed
// as broken with the reason, not loaded with a pair the window would ignore.
func TestListBreaksAThemeWhosePairHasItsScheme(t *testing.T) {
	s := NewInDir(t.TempDir())
	theme := customTheme("pale")
	theme.Pair = "light"
	if _, err := importTheme(t, s, theme); err != nil {
		t.Fatalf("Import: %v", err)
	}
	assertBroken(t, s, "pale", `pair "light" is a light theme too`)
}

// A pack removed in part leaves its other themes pointing at nothing; they
// still load.
func TestListLoadsAThemeWhosePairIsMissing(t *testing.T) {
	s := NewInDir(t.TempDir())
	theme := customTheme("orphan")
	theme.Pair = "gone"
	if _, err := importTheme(t, s, theme); err != nil {
		t.Fatalf("Import: %v", err)
	}
	themes, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := themes[len(themes)-1].ID; got != "orphan" {
		t.Fatalf("last listed theme = %q, want orphan", got)
	}
}
