//go:build !windows && !darwin

package fonts

import (
	"slices"
	"testing"
)

func TestParseFcListFiles(t *testing.T) {
	out := "/usr/share/fonts/TTF/A.ttf: \n/usr/share/fonts/TTF/B Mono.ttf: \n"
	want := []string{"/usr/share/fonts/TTF/A.ttf", "/usr/share/fonts/TTF/B Mono.ttf"}
	if got := parseFcListFiles(out); !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := parseFcListFiles(""); len(got) != 0 {
		t.Fatalf("empty output parsed as %q", got)
	}
}

func TestParseFamilyLines(t *testing.T) {
	if got := parseFamilyLines("Iosevka,Iosevka Term\n\n JetBrains Mono \n"); !slices.Equal(got, []string{"Iosevka", "JetBrains Mono"}) {
		t.Errorf("parsed %v", got)
	}
}

func TestParseFamilyMatchKeepsOnlyTheFamilyAskedFor(t *testing.T) {
	out := "Noto Sans CJK JP,Noto Sans CJK JP Regular\n/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc\n0"
	got, err := parseFamilyMatch(out, "Noto Sans CJK JP")
	if err != nil || got != (Location{Path: "/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc", Index: 0}) {
		t.Errorf("got %+v, %v", got, err)
	}
	if got, err := parseFamilyMatch("Noto Sans\n/usr/share/fonts/noto/NotoSans-Regular.ttf\n0", "Noto Sans CJK JP"); err != nil || got.Path != "" {
		t.Errorf("a substitute family read as %+v, %v", got, err)
	}
}
