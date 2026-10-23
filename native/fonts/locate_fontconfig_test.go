//go:build !windows && !darwin

package fonts

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gioui.org/font"
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

// The ranking against monospace is the same answer for every character, and
// asking for it again is another subprocess inside a frame.
func TestCoveringRanksAgainstMonospaceOnce(t *testing.T) {
	if _, err := Covering('A'); err != nil {
		t.Fatal(err)
	}
	calls := countCalls(t, "fc-match")
	path, err := Covering('B')
	if err != nil || path == "" {
		t.Fatalf("Covering('B') = %q, %v", path, err)
	}
	if got := calls(); got != 0 {
		t.Fatalf("a second Covering ran fc-match %d times, want 0", got)
	}
}

// A face fontconfig already named is the same answer the next time a
// terminal's renderer is built, and asking again is another subprocess
// inside a frame.
func TestLocateAsksFontconfigOncePerFont(t *testing.T) {
	bold := font.Font{Typeface: SystemMono, Weight: font.Bold}
	first, err := Locate(bold)
	if err != nil {
		t.Fatal(err)
	}
	calls := countCalls(t, "fc-match")
	again, err := Locate(bold)
	if err != nil || again != first {
		t.Fatalf("Locate again = %+v, %v; want %+v", again, err, first)
	}
	if got := calls(); got != 0 {
		t.Fatalf("a second Locate ran fc-match %d times, want 0", got)
	}
	if _, err := Locate(font.Font{Typeface: SystemMono, Weight: font.Bold, Style: font.Italic}); err != nil {
		t.Fatal(err)
	}
	if got := calls(); got != 1 {
		t.Fatalf("Locate of another style ran fc-match %d times, want 1", got)
	}
}

// countCalls puts a wrapper of the real command first on PATH and returns
// how many times it has run since.
func countCalls(t *testing.T, command string) func() int {
	t.Helper()
	real, err := exec.LookPath(command)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	script := "#!/bin/sh\necho >> '" + log + "'\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, command), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return func() int {
		out, err := os.ReadFile(log)
		if os.IsNotExist(err) {
			return 0
		}
		if err != nil {
			t.Fatal(err)
		}
		return strings.Count(string(out), "\n")
	}
}
