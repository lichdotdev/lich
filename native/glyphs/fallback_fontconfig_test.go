//go:build !windows && !darwin

package glyphs

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/native/fonts"
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

// A lookup that fails is a subprocess run inside a frame: a grapheme it fails
// for must neither end the frame nor run it again on the next one.
func TestAFailedLookupForAGraphemeIsKeptAndDoesNotFailTheFrame(t *testing.T) {
	r, err := New("monospace")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.SetSize(16); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	calls := filepath.Join(dir, "calls")
	script := "#!/bin/sh\necho >> '" + calls + "'\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "fc-list"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	k := Key{R: 0x10FFFD, Rest: "\u0301"}
	if _, err := r.Mask(k); err != nil {
		t.Fatalf("the frame failed: %v", err)
	}
	first, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Mask(k); err != nil {
		t.Fatalf("the frame failed: %v", err)
	}
	again, err := os.ReadFile(calls)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != len(first) {
		t.Fatalf("the failed lookup ran again: %d calls then %d", len(first), len(again))
	}
}

// fakeFcList puts an fc-list first on PATH that runs case on its pattern,
// falling through to the real one.
func fakeFcList(t *testing.T, cases string) {
	t.Helper()
	real, err := exec.LookPath("fc-list")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	script := "#!/bin/sh\ncase \"$1\" in\n" + cases + "\nesac\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(dir, "fc-list"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// A text font can cover the character the emoji font is looked up by, and is
// the closer to a monospace: DejaVu Sans has the emoticons. Only a query for
// color finds the emoji font then.
func TestTheEmojiFontIsLookedUpByColor(t *testing.T) {
	fakeFcList(t, "*color=true*) ;;\n*) exit 1 ;;")
	r := newTestRasterizer(t)
	if m := mask(t, r, Key{R: '⚠', Rest: "\ufe0f"}); m.Color == nil {
		t.Fatal("the emoji drew no color glyph")
	}
}

// With no color font on the machine the sequence still draws, in text.
func TestAKeycapWithoutAnEmojiFontDrawsItsDigit(t *testing.T) {
	fakeFcList(t, "*color=true*) exit 0 ;;")
	r := newTestRasterizer(t)
	m := mask(t, r, Key{R: '1', Rest: "\ufe0f\u20e3"})
	if m.Color != nil || m.Img == nil {
		t.Fatalf("drew color %v, outline %v; want the digit's outline", m.Color != nil, m.Img != nil)
	}
}

// A font lich downloaded sits in its own cache, where fontconfig never looks:
// with none installed covering a character, the terminal must still draw it
// from the download. fc-list answers nothing, as it does on a machine with no
// CJK font, and an emoji font linked into the download folder stands in for
// the downloaded Noto Sans SC.
func TestADownloadedFontCoversWhatNoInstalledOneDoes(t *testing.T) {
	const c = '🚀'
	installed, err := fonts.Covering(c)
	if err != nil || installed == "" {
		t.Fatalf("Covering(%q) = %q, %v", c, installed, err)
	}
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	dir := filepath.Join(cache, "lich", "fonts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(installed, filepath.Join(dir, "NotoSansSC-Regular.otf")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "NotoSansSC-Bold.otf.part"), []byte("half a font"), 0o644); err != nil {
		t.Fatal(err)
	}
	fakeFcList(t, "*) exit 0 ;;")
	r := sizedRasterizer(t)
	f, err := r.faceFor(Key{R: c})
	if err != nil {
		t.Fatal(err)
	}
	if f == nil {
		t.Fatalf("%q has no face, though the downloaded font covers it", c)
	}
}
