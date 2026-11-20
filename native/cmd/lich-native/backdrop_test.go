package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/lichdotdev/lich/native/ui"
)

var (
	fieldInk    = color.NRGBA{R: 159, G: 159, B: 169, A: 255}
	fieldHot    = color.NRGBA{R: 250, G: 250, B: 250, A: 255}
	fieldGround = color.NRGBA{R: 16, G: 16, B: 18, A: 255}
)

// The values the mockup's hash gives (pixel.js, run in node): every band
// moves by it, so a drift moves the whole field.
func TestHash2MatchesTheMockup(t *testing.T) {
	for _, c := range []struct {
		x, y, s int
		want    float64
	}{
		{1, 2, 3, 0.5008470041211694},
		{-5, 7, 12345 * 131, 0.5060585443861783},
		{100000, -3, 260 + 95744*131, 0.8175736246630549},
	} {
		if got := hash2(c.x, c.y, c.s); got != c.want {
			t.Errorf("hash2(%d, %d, %d) = %v, want %v", c.x, c.y, c.s, got, c.want)
		}
	}
}

// P.fbm's values in node, at the field's own seeds.
func TestFbmMatchesTheMockup(t *testing.T) {
	for _, c := range []struct {
		x, y   float64
		oct, s int
		want   float64
	}{
		{1.3, -2.7, 2, 70 + 11918*131, 0.5407644148759148},
		{-14.25, 3.5, 3, 40 + 11918*131, 0.5397531646511717},
		{123.456, 78.9, 3, 40 + 11918*131, 0.35876839790518095},
	} {
		if got := fbm(c.x, c.y, c.oct, c.s); math.Abs(got-c.want) > 1e-12 {
			t.Errorf("fbm(%v, %v, %d, %d) = %v, want %v", c.x, c.y, c.oct, c.s, got, c.want)
		}
	}
}

func TestBayerIsAPermutationOfItsSteps(t *testing.T) {
	var seen []float64
	for _, row := range bayer8 {
		seen = append(seen, row[:]...)
	}
	slices.Sort(seen)
	for i, v := range seen {
		if want := (float64(i) + 0.5) / 64; v != want {
			t.Fatalf("step %d is %v, want %v", i, v, want)
		}
	}
}

func paintedField(cols, rows int, t, cover float64) *image.RGBA {
	f := newBackdropField(cols, rows)
	inks := newBackdropInks(fieldInk, fieldHot, fieldGround)
	img := image.NewRGBA(image.Rect(0, 0, cols, rows))
	f.step(t, rows)
	f.paint(img, rows, cover, &inks)
	return img
}

func TestBackdropSameTimeSamePixels(t *testing.T) {
	a := paintedField(300, 200, 5000, 0)
	if b := paintedField(300, 200, 5000, 0); !bytes.Equal(a.Pix, b.Pix) {
		t.Fatal("the same time and size painted different pixels")
	}
	if c := paintedField(300, 200, 25000, 0); bytes.Equal(a.Pix, c.Pix) {
		t.Fatal("twenty seconds later the field had not moved")
	}
}

// lastLitRow is the last row with a lit cell, -1 for none.
func lastLitRow(img *image.RGBA) int {
	last := -1
	for y := range img.Rect.Dy() {
		for x := range img.Rect.Dx() {
			if img.RGBAAt(x, y).A != 0 {
				last = y
			}
		}
	}
	return last
}

func TestBackdropBandEndsAtTheMiddle(t *testing.T) {
	rows := 300
	if last := lastLitRow(paintedField(400, rows, 0, 1)); last < 0 || last >= rows/2 {
		t.Fatalf("a pane's band lights row %d, want some row above %d and none past it", last, rows/2)
	}
	if last := lastLitRow(paintedField(400, rows, 0, 0)); last < rows/2 {
		t.Fatalf("the whole-pane field lights no row past %d", last)
	}
}

// Two panes paint their field once per step of the clock: frames in between
// reuse it, the next step paints both again, and the frame asks for that
// step.
func TestPaneBackdropMovesOncePerStep(t *testing.T) {
	u, r, _ := besideFrames(t, image.Pt(1200, 800), func(m *model) {
		m.splitBeside(context.Background(), "orch", "wa", edgeRight, false)
	})
	for _, id := range []string{"orch", "wa"} {
		if _, ok := u.backdrops.byID[id]; !ok {
			t.Fatalf("pane %s has no backdrop", id)
		}
	}
	// the frames below start on a step, so they stay inside it
	r.now = u.backdrops.epoch.Add(time.Duration(int64(r.now.Sub(u.backdrops.epoch)/backdropStep)+1) * backdropStep)
	r.frame()
	painted := u.backdrops.painted
	r.now = r.now.Add(backdropStep / 2)
	r.frame()
	if u.backdrops.painted != painted {
		t.Fatalf("painted %d times inside one step, want %d", u.backdrops.painted, painted)
	}
	// frame already took the wakeup; the time it asked for stays
	if at, _ := r.router.WakeupTime(); !at.Equal(r.now.Add(backdropStep / 2)) {
		t.Fatalf("asked for a frame at %v, want the next step at %v", at, r.now.Add(backdropStep/2))
	}
	r.now = r.now.Add(backdropStep / 2)
	r.frame()
	if u.backdrops.painted != painted+2 {
		t.Fatalf("painted %d at the next step, want %d", u.backdrops.painted, painted+2)
	}
}

// A window without focus moves its backdrop at the still pace.
func TestStillWindowMovesItsBackdropLessOften(t *testing.T) {
	var b backdrops
	r := newFrameRig(image.Pt(400, 300), func(gtx C, th *ui.Theme) {
		b.layout(gtx, th, "p", image.Pt(300, 200), th.Background, 1, backdropMark{})
	})
	r.theme.Still = true
	r.frame()
	if at, _ := r.router.WakeupTime(); !at.Equal(r.now.Add(ui.StillFrame)) {
		t.Fatalf("asked for a frame at %v, want %v", at, r.now.Add(ui.StillFrame))
	}
}

// Going from one card to another and back finds the first one's backdrop
// kept; a pane no frame drew for backdropKeep loses it.
func TestBackdropOutOfSightIsKeptAWhile(t *testing.T) {
	var b backdrops
	id, size := "a", image.Pt(300, 200)
	r := newFrameRig(image.Pt(400, 300), func(gtx C, th *ui.Theme) {
		b.sweep(gtx.Now)
		b.layout(gtx, th, id, size, th.Background, 1, backdropMark{})
	})
	r.frame()
	first := b.byID["a"]
	id = "b"
	r.now = r.now.Add(time.Second)
	r.frame()
	id = "a"
	r.now = r.now.Add(time.Second)
	r.frame()
	if b.byID["a"] != first {
		t.Fatal("a's backdrop was built again after b was shown")
	}
	r.now = r.now.Add(backdropKeep + time.Second)
	r.frame()
	if _, ok := b.byID["b"]; ok {
		t.Fatal("kept b's backdrop past backdropKeep out of sight")
	}
	if _, ok := b.byID["a"]; !ok {
		t.Fatal("forgot the backdrop of the pane on screen")
	}
}

func BenchmarkBackdropStep(b *testing.B) {
	for _, c := range []struct {
		name  string
		size  image.Point
		cell  int
		whole bool
	}{
		{"1600x1000@1x band", image.Pt(1600, 1000), 3, false},
		{"1600x1000@1x whole", image.Pt(1600, 1000), 3, true},
		{"1600x1000@2x whole", image.Pt(3200, 2000), 6, true},
	} {
		b.Run(c.name, func(b *testing.B) {
			cols, rows := ceilDiv(c.size.X, c.cell), ceilDiv(c.size.Y, c.cell)
			f, inks := newBackdropField(cols, rows), newBackdropInks(fieldInk, fieldHot, fieldGround)
			img := image.NewRGBA(image.Rect(0, 0, cols, rows))
			drawn, cover := rows/2, 1.0
			if c.whole {
				drawn, cover = rows, 0
			}
			t := 0.0
			for b.Loop() {
				t += float64(backdropStep / time.Millisecond)
				f.step(t, drawn)
				f.paint(img, drawn, cover, &inks)
			}
		})
	}
}

func TestStartMarkIsTheLogoAtHalfSize(t *testing.T) {
	want := image.Pt((len(chatStartLogo[0])+1)/2, (len(chatStartLogo)+1)/2)
	if chatStartMarkSize != want {
		t.Fatalf("mark is %v cells, want %v", chatStartMarkSize, want)
	}
}

// markCells paints the mark over a field at t and counts its dots lit and
// dropped.
func markCells(t float64) (lit, dropped int, img *image.RGBA) {
	cols, rows := 200, 160
	f := newBackdropField(cols, rows)
	inks := newBackdropInks(fieldInk, fieldHot, fieldGround)
	th := &ui.Theme{Palette: ui.Dark}
	marks := newMarkInks(th, fieldGround)
	img = image.NewRGBA(image.Rect(0, 0, cols, rows))
	f.step(t, rows)
	f.paint(img, rows, 0, &inks)
	m := backdropMark{at: image.Pt(80, 40), fade: 1}
	f.paintMark(img, rows, m, &marks)
	for j, line := range chatStartMark {
		for i, s := range line {
			if s == '.' {
				continue
			}
			if img.RGBAAt(m.at.X+i, m.at.Y+j).A == 0 {
				dropped++
			} else {
				lit++
			}
		}
	}
	return lit, dropped, img
}

// The mark keeps most of its dots, drops some where the field thins, and
// drops others as the bands move on.
func TestStartMarkTakesTheFieldsMotion(t *testing.T) {
	lit, dropped, a := markCells(0)
	if lit == 0 || dropped == 0 || dropped > lit {
		t.Fatalf("%d dots lit and %d dropped: want most lit and some dropped", lit, dropped)
	}
	if _, _, b := markCells(40000); bytes.Equal(a.Pix, b.Pix) {
		t.Fatal("forty seconds later the mark had not changed")
	}
}
