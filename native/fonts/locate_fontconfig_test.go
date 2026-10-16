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
