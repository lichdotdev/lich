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

func TestEncodeBGRXKeepsTheColoursAndThePaddedRows(t *testing.T) {
	const w, h, stride = 40, 30, 40*4 + 16
	pixels := make([]byte, stride*h)
	for y := range h {
		for x := range w {
			// B, G, R, unused: red on the left half, blue on the right.
			px := pixels[y*stride+x*4:]
			if x < w/2 {
				px[2] = 255
			} else {
				px[0] = 255
			}
		}
	}
	data, err := EncodeBGRX(pixels, w, h, stride, 85)
	if err != nil {
		t.Fatal(err)
	}
	img, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds() != image.Rect(0, 0, w, h) {
		t.Fatalf("bounds %v, want %dx%d", img.Bounds(), w, h)
	}
	for _, c := range []struct {
		x    int
		want color.RGBA
	}{{5, color.RGBA{R: 255, A: 255}}, {w - 5, color.RGBA{B: 255, A: 255}}} {
		g := img.RGBAAt(c.x, h/2)
		for i, d := range [3]int{int(g.R) - int(c.want.R), int(g.G) - int(c.want.G), int(g.B) - int(c.want.B)} {
			if d < -12 || d > 12 {
				t.Fatalf("pixel %d channel %d is %v, want %v", c.x, i, g, c.want)
			}
		}
	}
}

func TestEncodeBGRXRefusesTooFewPixels(t *testing.T) {
	if _, err := EncodeBGRX(make([]byte, 10), 4, 4, 16, 85); err == nil {
		t.Fatal("encoded 10 bytes as a 4x4 image")
	}
}
