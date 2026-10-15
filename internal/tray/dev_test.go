package tray

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
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

// Every image in the shipped ICO turns amber, and the directory still points
// at each one.
func TestDevICOPaintsEveryImageAmber(t *testing.T) {
	ico, err := os.ReadFile(filepath.Join("..", "..", "build", "windows", "lich.ico"))
	if err != nil {
		t.Fatal(err)
	}
	dev, err := DevICO(ico)
	if err != nil {
		t.Fatal(err)
	}
	count := int(binary.LittleEndian.Uint16(dev[4:]))
	if count == 0 {
		t.Fatal("ICO has no images")
	}
	for i := range count {
		entry := dev[icoHeader+i*icoEntry:]
		size := binary.LittleEndian.Uint32(entry[8:])
		offset := binary.LittleEndian.Uint32(entry[12:])
		img, err := png.Decode(bytes.NewReader(dev[offset : offset+size]))
		if err != nil {
			t.Fatalf("image %d: %v", i, err)
		}
		if !hasAmber(img) {
			t.Errorf("image %d (%v) has no amber pixel", i, img.Bounds().Size())
		}
	}
}

func TestDevICORejectsATruncatedDirectory(t *testing.T) {
	if _, err := DevICO([]byte{0, 0, 1, 0, 3, 0}); err == nil {
		t.Error("an ICO claiming 3 images with no directory was accepted")
	}
}

func hasAmber(img image.Image) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA); c.R == devFlame.R && c.G == devFlame.G && c.B == devFlame.B {
				return true
			}
		}
	}
	return false
}
