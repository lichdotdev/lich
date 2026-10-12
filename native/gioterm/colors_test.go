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
	if err := v.SetColors(Colors{Foreground: color.NRGBA{R: 0x1f, G: 0x23, B: 0x28, A: 255}, Background: bg}); err != nil {
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
	if err := v.SetColors(Colors{Foreground: color.NRGBA{A: 255}, Background: color.NRGBA{R: 255, G: 255, B: 255, A: 255}}); err != nil {
		t.Fatal(err)
	}
	if s, _ := v.Update(); !s.s.full {
		t.Errorf("after SetColors: full %v dirty %v, want a full redraw", s.s.full, s.dirty)
	}
}

// A theme's ANSI entry colors what a program writes in that color, an entry
// it leaves out keeps the built-in one, and a program's OSC 4 still wins.
func TestSetColorsANSIReachesCells(t *testing.T) {
	v, err := New(10, 2, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	red := color.NRGBA{R: 0xeb, G: 0x6f, B: 0x92, A: 255}
	c := Colors{Foreground: color.NRGBA{A: 255}, Background: color.NRGBA{A: 255}}
	c.ANSI[1] = red
	if err := v.SetColors(c); err != nil {
		t.Fatal(err)
	}
	v.Write([]byte("\x1b[31mR\x1b[32mG"))
	s, err := v.Update()
	if err != nil {
		t.Fatal(err)
	}
	if got := rgba(styleOf(s.cells[0], s.s).fg); got != red {
		t.Errorf("red cell is %v, want the theme's %v", got, red)
	}
	if got := rgba(styleOf(s.cells[1], s.s).fg); got == red || got == (color.NRGBA{A: 255}) {
		t.Errorf("green cell is %v, want the built-in green", got)
	}
}

// A theme's cursor color paints the block, its ink the background; without
// one the cursor inverts the cell.
func TestCursorColor(t *testing.T) {
	v, err := New(10, 2, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	fg, bg := color.NRGBA{R: 0xe0, G: 0xde, B: 0xf4, A: 255}, color.NRGBA{R: 0x19, G: 0x17, B: 0x24, A: 255}
	cursor := color.NRGBA{R: 0x52, G: 0x4f, B: 0x67, A: 255}
	if err := v.SetColors(Colors{Foreground: fg, Background: bg}); err != nil {
		t.Fatal(err)
	}
	s, _ := v.Update()
	if st := cursorStyleOf(s.cells[0], s.s); rgba(st.bg) != fg || rgba(st.fg) != bg {
		t.Errorf("without a cursor color the cursor is %v on %v, want the cell inverted", rgba(st.fg), rgba(st.bg))
	}
	if err := v.SetColors(Colors{Foreground: fg, Background: bg, Cursor: cursor}); err != nil {
		t.Fatal(err)
	}
	s, _ = v.Update()
	if st := cursorStyleOf(s.cells[0], s.s); rgba(st.bg) != cursor || rgba(st.fg) != bg {
		t.Errorf("cursor is %v on %v, want %v on %v", rgba(st.fg), rgba(st.bg), bg, cursor)
	}
}
