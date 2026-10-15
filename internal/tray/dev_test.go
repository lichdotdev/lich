package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// The flame turns amber and the ground keeps its color and alpha.
func TestDevPNGPaintsTheFlameAmber(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	src.SetNRGBA(0, 0, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	src.SetNRGBA(1, 0, color.NRGBA{R: 0, G: 0, B: 0, A: 200})
	var in bytes.Buffer
	if err := png.Encode(&in, src); err != nil {
		t.Fatal(err)
	}
	data, err := DevPNG(in.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	got, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if c := color.NRGBAModel.Convert(got.At(0, 0)); c != devFlame {
		t.Errorf("flame %v, want %v", c, devFlame)
	}
	if c := color.NRGBAModel.Convert(got.At(1, 0)); c != (color.NRGBA{A: 200}) {
		t.Errorf("ground %v, want black at alpha 200", c)
	}
}
