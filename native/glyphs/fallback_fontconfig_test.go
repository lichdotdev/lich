//go:build !windows && !darwin

package glyphs

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Each lookup through fontconfig is a subprocess, ~20 ms, run inside a frame:
// a screen of CJK text, thousands of distinct characters the monospace family
// lacks, froze the window for seconds when every one of them asked again. A
// fallback face already loaded must answer for the characters it covers.
// Emoji stand in for CJK because CI installs a color emoji font and no CJK one.
func TestFallbackFaceServesTheCharactersItCovers(t *testing.T) {
	calls := countFcList(t)
	r, err := New("monospace")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.SetSize(16); err != nil {
		t.Fatal(err)
	}
	const distinct = 20
	for c := rune(0x1F600); c < 0x1F600+distinct; c++ {
		m, err := r.Mask(Key{R: c})
		if err != nil {
			t.Fatal(err)
		}
		if m.Img == nil && m.Color == nil {
			t.Fatalf("%U drew nothing; %s", c, whyNoColor(r, c))
		}
	}
	if got := calls(); got != 1 {
		t.Fatalf("%d distinct emoji asked fontconfig for a covering font %d times, want 1", distinct, got)
	}
}

// countFcList puts a wrapper of the real fc-list first on PATH and returns
// how many times it has run since.
func countFcList(t *testing.T) func() int {
	t.Helper()
	real, err := exec.LookPath("fc-list")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	script := "#!/bin/sh\necho >> '" + log + "'\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "fc-list"), []byte(script), 0o755); err != nil {
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
