package icons

import (
	"image"
	"image/color"
	"testing"
)

func TestDeviconKeepsItsColors(t *testing.T) {
	img := Devicon("python", false).render(28, color.NRGBA{A: 255}).(*image.NRGBA)
	blue, yellow := false, false
	for i := 0; i < len(img.Pix); i += 4 {
		r, g, b, a := img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]
		if a < 200 {
			continue
		}
		blue = blue || (b > r+40 && b > g)
		yellow = yellow || (r > b+80 && g > b+80)
	}
	if !blue || !yellow {
		t.Fatalf("python mark lost its brand colors: blue %v, yellow %v", blue, yellow)
	}
}

func TestMonoDeviconTakesTheGivenColor(t *testing.T) {
	want := color.NRGBA{R: 200, G: 10, B: 90, A: 255}
	img := Devicon("markdown", true).render(28, want).(*image.NRGBA)
	painted := 0
	for i := 0; i < len(img.Pix); i += 4 {
		p := img.Pix[i : i+4]
		if p[3] == 0 {
			continue
		}
		painted++
		if p[0] != want.R || p[1] != want.G || p[2] != want.B {
			t.Fatalf("pixel %v, want the color %v", p, want)
		}
	}
	if painted == 0 {
		t.Fatal("mono mark drew nothing")
	}
}

func TestDeviconPanicsOnUnknownName(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("no panic for a devicon the set lacks")
		}
	}()
	Devicon("cobol", false)
}
