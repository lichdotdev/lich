package icons

import (
	"image"
	"image/color"
	"testing"
)

func TestPictureKeepsItsShapeAndColors(t *testing.T) {
	red := color.NRGBA{R: 0xff, A: 0xff}
	img := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	for x := range 4 {
		for y := range 2 {
			img.SetNRGBA(x, y, red)
		}
	}
	b := Picture(img)
	if got := b.src.Bounds().Size(); got != image.Pt(4, 4) {
		t.Fatalf("square = %v, want 4×4", got)
	}
	if b.src.NRGBAAt(0, 0).A != 0 || b.src.NRGBAAt(0, 1) != red || b.src.NRGBAAt(0, 3).A != 0 {
		t.Errorf("a wide picture is not centered on its square: %v %v %v", b.src.NRGBAAt(0, 0), b.src.NRGBAAt(0, 1), b.src.NRGBAAt(0, 3))
	}
	if got := b.render(4, color.NRGBA{G: 0xff, A: 0xff}).(*image.NRGBA).NRGBAAt(1, 1); got.R < 0xf0 || got.G != 0 {
		t.Errorf("drawn as %v, want its own red, not the color passed", got)
	}
}

func TestABlackPictureTakesTheColorItIsDrawnWith(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for x := range 2 {
		for y := range 2 {
			img.SetNRGBA(x, y, color.NRGBA{A: 0xff})
		}
	}
	tone := color.NRGBA{R: 0xaa, G: 0xbb, B: 0xcc, A: 0xff}
	if got := Picture(img).render(2, tone).(*image.NRGBA).NRGBAAt(1, 1); got != tone {
		t.Errorf("drawn as %v, want the theme's %v", got, tone)
	}
}
