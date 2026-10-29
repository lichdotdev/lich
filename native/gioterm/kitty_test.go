package gioterm

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func pngOf(t *testing.T, w, h int, c color.NRGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// kittyAPC is a Kitty graphics command carrying payload base64-encoded.
func kittyAPC(keys string, payload []byte) string {
	return "\x1b_G" + keys + ";" + base64.StdEncoding.EncodeToString(payload) + "\x1b\\"
}

func newKittyTerm(t *testing.T) *Terminal {
	t.Helper()
	v, err := New(20, 6, testCellW, testCellH, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(v.Close)
	return v
}

func TestKittyPNGPlacedAtCursor(t *testing.T) {
	v := newKittyTerm(t)
	v.Write([]byte("\x1b[3;5H" + kittyAPC("a=T,f=100,i=7,q=2", pngOf(t, 12, 9, color.NRGBA{R: 255, A: 255}))))
	s, err := v.Update()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.above) != 1 || len(s.below) != 0 {
		t.Fatalf("placements above/below = %d/%d, want 1/0", len(s.above), len(s.below))
	}
	p := s.above[0]
	x, y := 4*testCellW, 2*testCellH
	if want := image.Rect(x, y, x+12, y+9); p.dst != want {
		t.Errorf("dst = %v, want %v", p.dst, want)
	}
	if want := image.Rect(0, 0, 12, 9); p.src != want {
		t.Errorf("src = %v, want %v", p.src, want)
	}
	if p.op.Size() != image.Pt(12, 9) {
		t.Errorf("texture is %v, want 12x9", p.op.Size())
	}
}

func TestKittyNegativeZDrawsUnderText(t *testing.T) {
	v := newKittyTerm(t)
	v.Write([]byte(kittyAPC("a=T,f=100,i=1,z=-1,q=2", pngOf(t, 4, 4, color.NRGBA{G: 255, A: 255}))))
	v.Write([]byte(kittyAPC("a=T,f=100,i=2,z=-1500000000,q=2", pngOf(t, 4, 4, color.NRGBA{B: 255, A: 255}))))
	s, err := v.Update()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.below) != 2 || len(s.above) != 0 {
		t.Fatalf("placements above/below = %d/%d, want 0/2", len(s.above), len(s.below))
	}
	if s.below[0].z > s.below[1].z {
		t.Errorf("below placements out of z order: %d before %d", s.below[0].z, s.below[1].z)
	}
}

// What gets deleted stops drawing, and its texture is let go.
func TestKittyDeleteDropsPlacementAndTexture(t *testing.T) {
	v := newKittyTerm(t)
	v.Write([]byte(kittyAPC("a=T,f=100,i=3,q=2", pngOf(t, 4, 4, color.NRGBA{A: 255}))))
	if _, err := v.Update(); err != nil {
		t.Fatal(err)
	}
	v.Write([]byte("\x1b_Ga=d,d=I,i=3,q=2\x1b\\"))
	s, err := v.Update()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.above)+len(s.below) != 0 || len(v.img.textures) != 0 {
		t.Fatalf("after delete: %d placements, %d textures", len(s.above)+len(s.below), len(v.img.textures))
	}
}

// A program must not make the window read files: only images sent in the
// stream itself are taken.
func TestKittyRefusesFileMedium(t *testing.T) {
	path := filepath.Join(t.TempDir(), "i.png")
	if err := os.WriteFile(path, pngOf(t, 4, 4, color.NRGBA{A: 255}), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, medium := range []string{"f", "t", "s"} {
		t.Run(medium, func(t *testing.T) {
			v := newKittyTerm(t)
			v.Write([]byte(kittyAPC(fmt.Sprintf("a=T,f=100,t=%s,i=4,q=2", medium), []byte(path))))
			s, err := v.Update()
			if err != nil {
				t.Fatal(err)
			}
			if len(s.above)+len(s.below) != 0 {
				t.Fatalf("medium %s placed an image", medium)
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("the file is gone: %v", err)
			}
		})
	}
}

func TestKittyQueryAnswered(t *testing.T) {
	var got []byte
	v, err := New(20, 6, testCellW, testCellH, func(b []byte) { got = append(got, b...) })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(v.Close)
	v.Write([]byte(kittyAPC("i=31,s=1,v=1,a=q,t=d,f=24", []byte{0, 0, 0})))
	if want := "\x1b_Gi=31;OK\x1b\\"; string(got) != want {
		t.Fatalf("query answered %q, want %q", got, want)
	}
}

// Programs size Kitty images from the cell's pixels, asked with XTWINOPS.
func TestSizeReportAnswersPixels(t *testing.T) {
	var got []byte
	v, err := New(20, 6, 8, 16, func(b []byte) { got = append(got, b...) })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(v.Close)
	v.Write([]byte("\x1b[14t\x1b[16t\x1b[18t"))
	if want := "\x1b[4;96;160t\x1b[6;16;8t\x1b[8;6;20t"; string(got) != want {
		t.Fatalf("size reports %q, want %q", got, want)
	}
}

func TestDecodePNGRefusesOversizedHeader(t *testing.T) {
	b := pngOf(t, 1, 1, color.NRGBA{A: 255})
	// IHDR's width and height sit right after the signature and chunk header;
	// the CRC no longer matches, which DecodeConfig does not check.
	copy(b[16:24], []byte{0, 0, 0x40, 0, 0, 0, 0x40, 0})
	if _, _, _, ok := decodePNG(b); ok {
		t.Fatal("decoded a PNG claiming 16384x16384")
	}
	if _, w, h, ok := decodePNG(pngOf(t, 3, 2, color.NRGBA{A: 255})); !ok || w != 3 || h != 2 {
		t.Fatalf("decodePNG of a 3x2 image = %dx%d ok=%v", w, h, ok)
	}
}

func TestToNRGBAFormats(t *testing.T) {
	cases := []struct {
		name   string
		format int
		pix    []byte
		want   [4]byte
	}{
		{"rgba", 1, []byte{1, 2, 3, 4}, [4]byte{1, 2, 3, 4}},
		{"rgb", 0, []byte{1, 2, 3}, [4]byte{1, 2, 3, 255}},
		{"gray alpha", 3, []byte{9, 7}, [4]byte{9, 9, 9, 7}},
		{"gray", 4, []byte{9}, [4]byte{9, 9, 9, 255}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			img := toNRGBA(1, 1, c.format, c.pix)
			if img == nil || [4]byte(img.Pix) != c.want {
				t.Fatalf("toNRGBA = %v, want %v", img, c.want)
			}
		})
	}
	if toNRGBA(2, 2, 1, []byte{1, 2, 3, 4}) != nil {
		t.Error("took pixels shorter than the size")
	}
}

// Under an image, the row renderer leaves default-background cells
// transparent so the image shows through, and keeps colored cells opaque.
func TestRowsClearUnderImages(t *testing.T) {
	r, err := NewRenderer(RenderRows, "monospace", 14)
	if err != nil {
		t.Fatal(err)
	}
	vt, err := New(20, 4, 8, 16, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vt.Close)
	vt.Write([]byte("\x1b[44m \x1b[0m"))
	drawFrame(t, r, vt)
	rr := r.(*rowRenderer)
	lastAlpha := func() byte { p := rr.imgs[0].Pix; return p[len(p)-1] }
	if lastAlpha() != 255 {
		t.Fatal("a row without images is not opaque")
	}
	vt.Write([]byte("\x1b[3;1H" + kittyAPC("a=T,f=100,i=5,z=-1,q=2", pngOf(t, 4, 4, color.NRGBA{R: 255, A: 255}))))
	drawFrame(t, r, vt)
	if lastAlpha() != 0 {
		t.Error("a default-background cell over an image is not transparent")
	}
	if rr.imgs[0].Pix[3] != 255 {
		t.Error("a colored cell over an image is not opaque")
	}
}
