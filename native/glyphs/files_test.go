package glyphs

import (
	"testing"

	"github.com/lichdotdev/lich/native/fonts"
)

// Every terminal and diff view has a Rasterizer of its own; a font file is
// tens of megabytes (a CJK collection, a color emoji font), and one read per
// view was hundreds of megabytes held for a handful of sessions.
func TestRasterizersShareAFontFile(t *testing.T) {
	a, b := sizedRasterizer(t), sizedRasterizer(t)
	if a.styles[0].file != b.styles[0].file {
		t.Error("two rasterizers of one family each read its regular face")
	}
	path, err := fonts.Covering('🚀')
	if err != nil || path == "" {
		t.Fatalf("Covering('🚀') = %q, %v", path, err)
	}
	fa, err := a.load(path)
	if err != nil {
		t.Fatal(err)
	}
	fb, err := b.load(path)
	if err != nil {
		t.Fatal(err)
	}
	if fa.file != fb.file {
		t.Errorf("two rasterizers each read %s", path)
	}
	if fa.face == fb.face {
		t.Error("two rasterizers draw through one sized face, which is not safe to share")
	}
}

// Only a font with color bitmaps needs go-text's parse; on a CJK collection
// it read every face of the file to keep the first.
func TestOnlyAColorFontIsParsedForBitmaps(t *testing.T) {
	r := sizedRasterizer(t)
	if r.styles[0].file.color != nil {
		t.Error("the monospace family was parsed for color bitmaps")
	}
	path, err := fonts.Covering('🚀')
	if err != nil || path == "" {
		t.Fatalf("Covering('🚀') = %q, %v", path, err)
	}
	f, err := r.load(path)
	if err != nil {
		t.Fatal(err)
	}
	if f.file.color == nil {
		t.Errorf("%s, the emoji font, was not parsed for color bitmaps", path)
	}
}

func sizedRasterizer(t *testing.T) *Rasterizer {
	t.Helper()
	r, err := New("monospace")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.SetSize(16); err != nil {
		t.Fatal(err)
	}
	return r
}
