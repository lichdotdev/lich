package profilestore

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func writePNG(t *testing.T, path string, w, h int, fill func(x, y int) color.Color) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, fill(x, y))
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, path, buf.String())
}

func ptr(s string) *string { return &s }

func TestUpdateSetsAndClearsColorWithoutTouchingTheName(t *testing.T) {
	s := toy(machine(t))
	p, err := s.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Update(p.Slug, Update{Color: ptr("teal")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Color != "teal" || got.Name != "Work" {
		t.Errorf("after setting a color: %+v", got)
	}
	if got, _ = s.Update(p.Slug, Update{Color: ptr("")}); got.Color != "" {
		t.Errorf("after clearing the color: %+v", got)
	}
}

func TestUpdateOnPersonalKeepsWhatItDidNotChange(t *testing.T) {
	s := toy(machine(t))
	if _, err := s.Update(Personal, Update{Name: "Home"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update("", Update{Color: ptr("blue")}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Update(Personal, Update{Name: "Mine"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Mine" || got.Color != "blue" || !got.Personal {
		t.Errorf("Personal = %+v, want Mine in blue", got)
	}
}

func TestUpdateStoresTheImageAsACenteredSquare(t *testing.T) {
	s := toy(machine(t))
	p, err := s.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	// A wide image whose centre third is red: the square cut from its middle
	// is red edge to edge.
	src := filepath.Join(t.TempDir(), "wide.png")
	writePNG(t, src, 300, 100, func(x, _ int) color.Color {
		if x >= 100 && x < 200 {
			return color.NRGBA{R: 255, A: 255}
		}
		return color.NRGBA{B: 255, A: 255}
	})
	got, err := s.Update(p.Slug, Update{Image: &src})
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(got.Image))
	if err != nil {
		t.Fatalf("the stored image is not a PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != avatarEdge || b.Dy() != avatarEdge {
		t.Errorf("stored at %v, want %d square", b, avatarEdge)
	}
	for _, x := range []int{0, avatarEdge - 1} {
		if r, _, b, _ := img.At(x, avatarEdge/2).RGBA(); r>>8 < 200 || b>>8 > 50 {
			t.Errorf("pixel %d is not red: the crop is off centre", x)
		}
	}
	list, _ := s.List()
	if !bytes.Equal(list[1].Image, got.Image) {
		t.Error("List does not carry the image Update stored")
	}
	if got, _ = s.Update(p.Slug, Update{Image: ptr("")}); got.Image != nil {
		t.Error("an empty Image did not remove the image")
	}
}

func TestUpdateRefusesAFileThatIsNotAnImageBeforeWritingAnything(t *testing.T) {
	s := toy(machine(t))
	p, err := s.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	notes := filepath.Join(t.TempDir(), "notes.txt")
	mustWrite(t, notes, "hello")
	if _, err := s.Update(p.Slug, Update{Name: "Client", Image: &notes}); err == nil {
		t.Fatal("a text file was taken as an image")
	}
	list, _ := s.List()
	if list[1].Name != "Work" {
		t.Errorf("the name changed although the image was refused: %q", list[1].Name)
	}
}

func TestUpdateRefusesAFileOverTheSizeLimit(t *testing.T) {
	s := toy(machine(t))
	big := filepath.Join(t.TempDir(), "big.png")
	if err := os.WriteFile(big, make([]byte, maxImageFileBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(Personal, Update{Image: &big}); err == nil {
		t.Error("a file over the limit was accepted")
	}
}

func TestDeleteTakesTheImageWithIt(t *testing.T) {
	s := toy(machine(t))
	p, err := s.Create("Work")
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "a.png")
	writePNG(t, src, 8, 8, func(int, int) color.Color { return color.White })
	if _, err := s.Update(p.Slug, Update{Image: &src}); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(p.Slug); err != nil {
		t.Fatal(err)
	}
	path, _ := s.imagePath(p.Slug)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the image outlived its profile: %v", err)
	}
}
