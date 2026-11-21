package grimoire

import (
	"strings"
	"testing"
)

func TestMarkIsTheMedallionsForegroundAlone(t *testing.T) {
	for _, id := range []string{"claude", "codex", "opencode", "omp", "antigravity", "cursor", "kiro", "crush"} {
		g := Mark(id)
		if g == nil {
			t.Fatalf("%s: no mark", id)
		}
		if h, w := len(g), len(g[0]); h > 2*markR+1 || w > 2*markR+1 {
			t.Errorf("%s: mark is %dx%d, wider than the medallion", id, w, h)
		}
		ink := false
		for _, row := range g {
			if strings.Trim(string(row), "42.") != "" {
				t.Errorf("%s: row %q holds more than the mark's pixels", id, row)
			}
			ink = ink || strings.Contains(string(row), "4")
		}
		if !ink || !strings.Contains(string(g[0]), "4") || !strings.Contains(string(g[len(g)-1]), "4") {
			t.Errorf("%s: mark is not cropped to its pixels", id)
		}
	}
}

func TestMarkOfAProviderWithNoBookIsNil(t *testing.T) {
	if g := Mark("shell"); g != nil {
		t.Fatalf("shell: %v, want none", g)
	}
}

func TestOpencodesMarkIsAFrameHalfLitBelow(t *testing.T) {
	want := []string{
		"444444444",
		"444444444",
		"44.....44",
		"44.....44",
		"44.....44",
		"442222244",
		"442222244",
		"442222244",
		"442222244",
		"442222244",
		"444444444",
		"444444444",
	}
	sameMark(t, "opencode", want)
}

func TestCursorsMarkIsACubeWithTheArrowCutOut(t *testing.T) {
	sameMark(t, "cursor", []string{
		".....444.....",
		"...4444444...",
		".44444444444.",
		"4444444444444",
		"4...........4",
		"444........44",
		"44444......44",
		"444444....444",
		"444444...4444",
		"444444...4444",
		"444444..44444",
		"444444..44444",
		".44444.44444.",
		"...4444444...",
		".....444.....",
	})
}

func sameMark(t *testing.T, id string, want []string) {
	t.Helper()
	g := Mark(id)
	if len(g) != len(want) {
		t.Fatalf("%s: mark is %d rows, want %d", id, len(g), len(want))
	}
	for y, row := range want {
		if string(g[y]) != row {
			t.Errorf("%s: row %d = %q, want %q", id, y, g[y], row)
		}
	}
}

func TestOnlyOpencodesMarkIsHalfLit(t *testing.T) {
	for _, id := range []string{"claude", "codex", "omp", "antigravity", "cursor", "kiro", "crush"} {
		for _, row := range Mark(id) {
			if strings.Contains(string(row), "2") {
				t.Errorf("%s: row %q takes the medallion's ring into the mark", id, row)
			}
		}
	}
}
