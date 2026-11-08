package glyphs

import (
	"bytes"
	"image"
	"log"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"
)

func newTestRasterizer(t *testing.T) *Rasterizer {
	t.Helper()
	r, err := New("monospace")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.SetSize(28); err != nil {
		t.Fatal(err)
	}
	return r
}

func mask(t *testing.T, r *Rasterizer, k Key) *Mask {
	t.Helper()
	m, err := r.Mask(k)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// A flag and a ZWJ family are one ligature glyph of the emoji font, not the
// glyph of their first codepoint.
func TestAnEmojiSequenceDrawsItsLigature(t *testing.T) {
	r := newTestRasterizer(t)
	seqs := []Key{{R: 0x1F468, Rest: "\u200d👩\u200d👧"}}
	// Segoe UI Emoji, Windows' emoji font, has no flags.
	if runtime.GOOS != "windows" {
		seqs = append(seqs, Key{R: 0x1F1E7, Rest: "\U0001F1F7"})
	}
	for _, k := range seqs {
		seq, first := mask(t, r, k), mask(t, r, Key{R: k.R})
		if seq.Color == nil {
			t.Fatalf("%+q drew no color glyph", string(k.R)+k.Rest)
		}
		if first.Color != nil && bytes.Equal(seq.Color.Pix, first.Color.Pix) {
			t.Fatalf("%+q drew the glyph of %U alone", string(k.R)+k.Rest, k.R)
		}
	}
}

// e and a combining acute draw the e and, above it, the accent.
func TestACombiningMarkDrawsOverItsBase(t *testing.T) {
	r := newTestRasterizer(t)
	e := mask(t, r, Key{R: 'e'})
	accented := mask(t, r, Key{R: 'e', Rest: "\u0301"})
	if accented.Img == nil {
		t.Fatal("e + U+0301 drew nothing")
	}
	top := e.Off.Y
	inkAbove, inCell := 0, true
	b := accented.Img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if accented.Img.AlphaAt(x, y).A < 0x80 {
				continue
			}
			p := image.Pt(x, y).Add(accented.Off)
			if p.Y < top {
				inkAbove++
			}
			if p.X >= r.CellW() {
				inCell = false
			}
		}
	}
	if inkAbove == 0 {
		t.Fatal("no ink above the e: the accent is missing")
	}
	if !inCell {
		t.Fatal("the accented e spills into the next cell")
	}
}

// A variation selector asks for the emoji, whose base a text font may cover
// too: drawn from that font it is narrow, monochrome and beside a blank
// column. The keycap's digit is in every font.
func TestAnEmojiPresentationComesFromTheColorFont(t *testing.T) {
	r := newTestRasterizer(t)
	for _, k := range []Key{
		{R: '⚠', Rest: "\ufe0f"},
		{R: '✔', Rest: "\ufe0f"},
		{R: '❤', Rest: "\ufe0f"},
		{R: '1', Rest: "\ufe0f\u20e3"},
		{R: '❤', Rest: "\ufe0f\u200d🔥"},
	} {
		if m := mask(t, r, k); m.Color == nil {
			t.Errorf("%+q drew no color glyph", string(k.R)+k.Rest)
		}
	}
}

func TestKeyAsksForEmoji(t *testing.T) {
	for _, tc := range []struct {
		k    Key
		want bool
	}{
		{Key{R: '⚠', Rest: "\ufe0f"}, true},
		{Key{R: '1', Rest: "\ufe0f\u20e3"}, true},
		{Key{R: 0x1F468, Rest: "\u200d👩"}, true},
		{Key{R: 0x1F1E7, Rest: "\U0001F1F7"}, true},
		{Key{R: 0x1F44D, Rest: "\U0001F3FD"}, true},
		{Key{R: 0x1F3F4, Rest: "\U000E0067\U000E007F"}, true},
		{Key{R: '⚠', Rest: "\ufe0e"}, false},
		{Key{R: 'e', Rest: "\u0301"}, false},
		{Key{R: 'ക', Rest: "\u0d4d\u200d"}, false},
		{Key{R: 'क', Rest: "\u094d\u200dष"}, false},
		{Key{R: '#', Rest: "\u200d"}, false},
		{Key{R: '1', Rest: "\u200d"}, false},
		{Key{R: 0x1F1E7}, false},
	} {
		if got := tc.k.Emoji(); got != tc.want {
			t.Errorf("%+q: Emoji() = %v, want %v", string(tc.k.R)+tc.k.Rest, got, tc.want)
		}
	}
}

// A font sfnt reads and go-text refuses must not fail the frame over one
// accent in a program's output.
func TestAFontGoTextRefusesDrawsTheFirstCodepoint(t *testing.T) {
	r := newTestRasterizer(t)
	f := r.styles[0]
	f.file = &fontFile{sf: f.file.sf, data: []byte("not a font")}
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	m, err := r.Mask(Key{R: 'e', Rest: "\u0301"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Img == nil {
		t.Fatal("the e was not drawn")
	}
	if _, err := r.Mask(Key{R: 'a', Rest: "\u0301"}); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(logged.String(), "\n"); n != 1 {
		t.Fatalf("logged %d lines for one bad font, want 1:\n%s", n, logged.String())
	}
}

func TestClusterMasksAreBounded(t *testing.T) {
	r := newTestRasterizer(t)
	for i := range 1100 {
		mask(t, r, Key{R: 'e', Rest: string(rune(0x10000 + i))})
	}
	if n := len(r.clusters); n > 1024 {
		t.Fatalf("%d cluster masks kept", n)
	}
}

// What shaping places outside the two cells stays in the mask, as a single
// codepoint's does: an italic's left overhang, a tall stack of marks.
func TestOutlinesKeepWhatFallsOutsideTheCell(t *testing.T) {
	r := newTestRasterizer(t)
	face, err := r.textFace(r.styles[0])
	if err != nil {
		t.Fatal(err)
	}
	gid, ok := face.NominalGlyph('e')
	if !ok {
		t.Fatal("no e in the font")
	}
	up := fixed.I(r.CellH())
	left := fixed.I(r.CellW() / 2)
	img, off := r.outlines(face, []shaping.Glyph{{GlyphID: gid, XOffset: -left, YOffset: up}})
	if img == nil {
		t.Fatal("no mask")
	}
	if off.X >= 0 || off.Y >= 0 {
		t.Fatalf("mask at %v, want it left of and above the cell", off)
	}
	if !hasInk(img) {
		t.Fatal("the mask is empty")
	}
}

func hasInk(img *image.Alpha) bool {
	for _, a := range img.Pix {
		if a != 0 {
			return true
		}
	}
	return false
}
