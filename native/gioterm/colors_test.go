package gioterm

import (
	"image/color"
	"testing"
)

// The defaults reach the snapshot a renderer paints from, and a program's
// OSC 11 still overrides them.
func TestSetColorsReachTheSnapshot(t *testing.T) {
	v, err := New(10, 2, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	bg := color.NRGBA{R: 0xe8, G: 0xe8, B: 0xea, A: 255}
	if err := v.SetColors(color.NRGBA{R: 0x1f, G: 0x23, B: 0x28, A: 255}, bg); err != nil {
		t.Fatal(err)
	}
	s, err := v.Update()
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Background(); got != bg {
		t.Errorf("background %v, want %v", got, bg)
	}
	v.Write([]byte("\x1b]11;#102030\x1b\\"))
	if s, _ = v.Update(); s.Background() != (color.NRGBA{R: 0x10, G: 0x20, B: 0x30, A: 255}) {
		t.Errorf("after OSC 11 the background is %v, want the program's #102030", s.Background())
	}
}

func TestSetColorsRedrawsEveryRow(t *testing.T) {
	v, err := New(10, 2, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	v.Write([]byte("hi"))
	if _, err := v.Update(); err != nil {
		t.Fatal(err)
	}
	if s, _ := v.Update(); s.s.full {
		t.Fatal("an unchanged terminal reported a full redraw")
	}
	if err := v.SetColors(color.NRGBA{A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 255}); err != nil {
		t.Fatal(err)
	}
	if s, _ := v.Update(); !s.s.full {
		t.Errorf("after SetColors: full %v dirty %v, want a full redraw", s.s.full, s.dirty)
	}
}
