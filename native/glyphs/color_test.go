package glyphs

import (
	"bytes"
	"fmt"
	"image"
	"testing"

	gotext "github.com/go-text/typesetting/font"

	"github.com/lichdotdev/lich/native/fonts"
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
		t.Fatalf("🚀 drew no color glyph (alpha mask: %v); %s", m.Img != nil, whyNoColor(r, '🚀'))
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

// whyNoColor walks the path Mask takes for c and says where it stopped, for a
// failure on a machine nobody here can look at.
func whyNoColor(r *Rasterizer, c rune) string {
	path, err := fonts.Covering(c)
	if err != nil || path == "" {
		return fmt.Sprintf("Covering = %q, %v", path, err)
	}
	f, err := r.load(path)
	if err != nil {
		return fmt.Sprintf("Covering = %q, load: %v", path, err)
	}
	faces, err := gotext.ParseTTC(bytes.NewReader(f.file.data))
	if err != nil {
		return fmt.Sprintf("Covering = %q, covers %v, ParseTTC: %v", path, r.covers(f, c), err)
	}
	out := fmt.Sprintf("Covering = %q, covers %v, %d faces", path, r.covers(f, c), len(faces))
	for i, face := range faces {
		gid, ok := face.NominalGlyph(c)
		bm, hasBitmap := face.GlyphDataBitmap(gid)
		out += fmt.Sprintf("; face %d: glyph %d %v, bitmap %v format %v, %T",
			i, gid, ok, hasBitmap, bm.Format, face.GlyphData(gid))
	}
	return out
}
