package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// The dot fills the top right corner, a clear ring cuts the icon around it,
// and the rest of the icon is left as it was.
func TestBadgePNGDotsTheCorner(t *testing.T) {
	const size = 100
	ground := color.NRGBA{R: 0x10, G: 0x10, B: 0x12, A: 0xff}
	src := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			src.SetNRGBA(x, y, ground)
		}
	}
	var in bytes.Buffer
	if err := png.Encode(&in, src); err != nil {
		t.Fatal(err)
	}
	fill := badgeFills[badgeWaiting]
	data, err := badgePNG(in.Bytes(), fill)
	if err != nil {
		t.Fatal(err)
	}
	got, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	r := size * badgeRadius
	at := func(x, y float64) color.NRGBA {
		return color.NRGBAModel.Convert(got.At(int(x), int(y))).(color.NRGBA)
	}
	if c := at(size-r, r); c != fill {
		t.Errorf("badge centre %v, want %v", c, fill)
	}
	if c := at(size-r, 2*r+size*badgeRing/2); c.A != 0 {
		t.Errorf("ring under the badge %v, want clear", c)
	}
	if c := at(10, size-10); c != ground {
		t.Errorf("far corner %v, want the icon untouched", c)
	}
}

// Every image of the shipped icons takes the badge.
func TestBadgedIconsBadgeTheShippedIcons(t *testing.T) {
	pngIcon, err := os.ReadFile(filepath.Join("..", "..", "build", "appicon-256.png"))
	if err != nil {
		t.Fatal(err)
	}
	icoIcon, err := os.ReadFile(filepath.Join("..", "..", "build", "windows", "lich.ico"))
	if err != nil {
		t.Fatal(err)
	}
	icons := badgedIcons(Icons{PNG: pngIcon, ICO: icoIcon})
	if !bytes.Equal(icons[badgeNone].PNG, pngIcon) || !bytes.Equal(icons[badgeNone].ICO, icoIcon) {
		t.Error("the unbadged icon changed")
	}
	for b := badgeBusy; b < badgeCount; b++ {
		if bytes.Equal(icons[b].PNG, pngIcon) || bytes.Equal(icons[b].ICO, icoIcon) {
			t.Errorf("badge %d left an icon as it was", b)
		}
	}
}

// An icon that cannot be badged is shown without the badge.
func TestBadgedIconsKeepAnIconTheyCannotDecode(t *testing.T) {
	bad := []byte("not an image")
	icons := badgedIcons(Icons{PNG: bad, ICO: bad})
	if !bytes.Equal(icons[badgeWaiting].PNG, bad) || !bytes.Equal(icons[badgeWaiting].ICO, bad) {
		t.Errorf("an undecodable icon was replaced: %+v", icons[badgeWaiting])
	}
}
