package glyphs

import (
	"image"
	"testing"
)

// A color emoji font is a bitmap font (CBDT on Linux): it has no outline for
// sfnt to rasterize, so the glyph must come back as a color image instead.
func TestMaskDrawsColorEmoji(t *testing.T) {
	r, err := New("monospace")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.SetSize(16); err != nil {
		t.Fatal(err)
	}
	m, err := r.Mask(Key{R: '🚀'})
	if err != nil {
		t.Fatal(err)
	}
	if m.Color == nil {
		t.Fatalf("🚀 drew no color glyph (alpha mask: %v)", m.Img != nil)
	}
	got := m.Color.Bounds().Add(m.Off)
	if box := (image.Rect(0, 0, 2*r.CellW(), r.CellH())); !got.In(box) {
		t.Fatalf("glyph at %v spills out of its two cells %v", got, box)
	}
	if !hasChroma(m.Color) {
		t.Fatal("glyph has no colored pixel")
	}
}

func hasChroma(img *image.RGBA) bool {
	for i := 0; i < len(img.Pix); i += 4 {
		p := img.Pix[i : i+4]
		if p[3] > 0 && (p[0] != p[1] || p[1] != p[2]) {
			return true
		}
	}
	return false
}
