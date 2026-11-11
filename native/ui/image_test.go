package ui

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func TestDecodeImageTakesPlainBytes(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	if d, err := DecodeImage(buf.Bytes()); err != nil || d == nil {
		t.Fatalf("png: %v", err)
	}
	if _, err := DecodeImage([]byte("not an image")); err == nil {
		t.Error("garbage decoded")
	}
}

func TestImageDrawsAtItsSize(t *testing.T) {
	th := testTheme()
	src := NewImageData(image.NewNRGBA(image.Rect(0, 0, 40, 20)))
	img := th.Image(src)
	img.Width, img.Height = 60, 30
	if got := layoutAt(img.Layout, 0, 300).Size; got != image.Pt(60, 30) {
		t.Errorf("sized = %v, want the 60x30 asked", got)
	}
	bound := th.Image(src)
	bound.Width = 500
	if got := layoutAt(bound.Layout, 0, 300).Size.X; got != 300 {
		t.Errorf("width past the constraint = %d, want it clamped to 300", got)
	}
}
