package glyphs

import (
	"bytes"
	"image"
	"runtime"
	"testing"
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
	seqs := []Key{{R: 0x1F468, Rest: "‍👩‍👧"}}
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
	accented := mask(t, r, Key{R: 'e', Rest: "́"})
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
