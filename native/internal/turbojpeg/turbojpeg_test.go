package turbojpeg

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestDecodeMatchesTheStandardDecoder(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 64, 48))
	for y := range 48 {
		for x := range 64 {
			src.SetRGBA(x, y, color.RGBA{R: uint8(x * 4), G: uint8(y * 5), B: uint8(x + y), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	want, err := jpeg.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds() != want.Bounds() {
		t.Fatalf("bounds %v, want %v", got.Bounds(), want.Bounds())
	}
	// The two decoders round and upsample chroma differently; a wrong pixel
	// format or stride is off by far more than this.
	const tolerance = 12
	for y := range 48 {
		for x := range 64 {
			g := got.RGBAAt(x, y)
			wr, wg, wb, _ := want.At(x, y).RGBA()
			for i, d := range [3]int{int(g.R) - int(wr>>8), int(g.G) - int(wg>>8), int(g.B) - int(wb>>8)} {
				if d < -tolerance || d > tolerance {
					t.Fatalf("pixel %d,%d channel %d is %d off the standard decoder's", x, y, i, d)
				}
			}
			if g.A != 255 {
				t.Fatalf("pixel %d,%d alpha %d, want opaque", x, y, g.A)
			}
		}
	}
}

func TestDecodeRefusesWhatIsNoJPEG(t *testing.T) {
	for name, data := range map[string][]byte{"empty": nil, "garbage": []byte("not a jpeg"), "cut short": {0xff, 0xd8, 0xff}} {
		if img, err := Decode(data); err == nil {
			t.Errorf("%s: decoded to %v, want an error", name, img.Bounds())
		}
	}
}
