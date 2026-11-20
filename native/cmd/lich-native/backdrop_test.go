package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"slices"
	"testing"

	"github.com/lichdotdev/lich/native/ui"
)

var grainInk = color.NRGBA{R: 200, G: 200, B: 200, A: 255}

// The values the mockup's hash2 gives (backdrop-embers.html, run in node):
// every mark and spark is placed by it, so a drift moves the whole field.
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

func TestGrainSameSeedAndSizeSamePixels(t *testing.T) {
	size := image.Pt(640, 400)
	a := grainImage(size, 3, 2, grainInk, grainSeeds[3])
	b := grainImage(size, 3, 2, grainInk, grainSeeds[3])
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Fatal("the same seed and size painted different pixels")
	}
	if c := grainImage(size, 3, 2, grainInk, grainSeeds[4]); bytes.Equal(a.Pix, c.Pix) {
		t.Fatal("another seed painted the same pixels")
	}
}

func TestGrainBandEndsAtTheMiddle(t *testing.T) {
	size, cell := image.Pt(400, 1000), 3
	img := grainImage(size, cell, 2, grainInk, grainSeeds[0])
	top, last := false, -1
	for y := range img.Rect.Dy() {
		for x := range img.Rect.Dx() {
			if img.RGBAAt(x, y).A != 0 {
				top = top || y < 10*cell
				last = max(last, y/cell*cell)
			}
		}
	}
	if !top {
		t.Fatal("no dot near the pane's top")
	}
	if last < 0 || last >= size.Y/2 {
		t.Fatalf("a dot starts at y %d, past the middle %d", last, size.Y/2)
	}
	if img.Rect.Dy() > size.Y/2+cell {
		t.Fatalf("the image is %d tall, more than the band", img.Rect.Dy())
	}
}

func TestGrainSeedIsTheSameForTheSamePane(t *testing.T) {
	if grainSeed("s1") != grainSeed("s1") {
		t.Fatal("one pane got two seeds")
	}
	// FNV-1a of "s1" is 0x0851b8c9: every run picks the tenth seed.
	if got := grainSeed("s1"); got != 66777 {
		t.Fatalf("seed %d, want 66777", got)
	}
}

func TestGrainLowersItsDotsWhereTextWouldNotRead(t *testing.T) {
	ground := color.NRGBA{R: 30, G: 30, B: 30, A: 255}
	if a := grainAlpha(color.NRGBA{R: 240, G: 240, B: 240, A: 255}, color.NRGBA{R: 160, G: 160, B: 160, A: 255}, ground, 1); a != grainSolid {
		t.Fatalf("alpha %v on a readable ground, want %v", a, grainSolid)
	}
	// dots in a muted gray far brighter than the text wash the text out
	dim := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	if a := grainAlpha(color.NRGBA{R: 60, G: 60, B: 60, A: 255}, dim, ground, 1); a >= grainSolid {
		t.Fatalf("alpha %v where the text would not read, want it lowered", a)
	}
}

// Two panes of a layout each paint their backdrop once; frames after that
// reuse it, and a theme change paints both again.
func TestPaneBackdropIsPaintedOncePerSizeAndTheme(t *testing.T) {
	u, r, _ := besideFrames(t, image.Pt(1200, 800), func(m *model) {
		m.splitBeside(context.Background(), "orch", "wa", edgeRight, false)
	})
	for _, id := range []string{"orch", "wa"} {
		e, ok := u.backdrops.byID[id]
		if !ok {
			t.Fatalf("pane %s has no backdrop", id)
		}
		if e.key.seed != grainSeed(id) || e.size.Y == 0 {
			t.Fatalf("pane %s backdrop %+v", id, e.key)
		}
	}
	painted := u.backdrops.painted
	if painted != 2 {
		t.Fatalf("painted %d backdrops for two panes", painted)
	}
	r.frame()
	r.frame()
	if u.backdrops.painted != painted {
		t.Fatalf("painted %d times over frames that changed nothing, want %d", u.backdrops.painted, painted)
	}
	r.theme = &ui.Theme{Palette: ui.Light, Shaper: r.theme.Shaper, Surface: ui.Light.Background}
	r.frame()
	if u.backdrops.painted != painted+2 {
		t.Fatalf("painted %d after a theme change, want %d", u.backdrops.painted, painted+2)
	}
}

func TestBackdropSweepForgetsPanesNoFrameDrew(t *testing.T) {
	var b backdrops
	b.byID = map[string]*backdropEntry{"gone": {}, "drawn": {seen: true}}
	b.sweep()
	if _, ok := b.byID["gone"]; ok {
		t.Fatal("kept a backdrop no frame drew")
	}
	if _, ok := b.byID["drawn"]; !ok {
		t.Fatal("forgot a backdrop the last frame drew")
	}
}

func BenchmarkGrainImage(b *testing.B) {
	for _, c := range []struct {
		name      string
		size      image.Point
		cell, dot int
	}{
		{"1600x1000@1x", image.Pt(1600, 1000), 3, 2},
		{"1600x1000@2x", image.Pt(3200, 2000), 6, 4},
	} {
		b.Run(c.name, func(b *testing.B) {
			for b.Loop() {
				grainImage(c.size, c.cell, c.dot, grainInk, grainSeeds[0])
			}
		})
	}
}

// A pane resized keeps its field while the size moves, and paints the new
// one on the frame after the size holds.
func TestBackdropWaitsForAResizeToHold(t *testing.T) {
	var b backdrops
	size := image.Pt(600, 400)
	r := newFrameRig(image.Pt(800, 600), func(gtx C, th *ui.Theme) {
		b.layout(gtx, th, "p", size, th.Background)
	})
	r.frame()
	size = image.Pt(620, 400)
	if _, redraw := r.frame(); !redraw || b.painted != 1 {
		t.Fatalf("a resize painted %d times and asked for a frame %v, want 1 and true", b.painted, redraw)
	}
	size = image.Pt(640, 400)
	r.frame()
	if b.painted != 1 {
		t.Fatalf("painted %d times while the size moved, want 1", b.painted)
	}
	if _, redraw := r.frame(); redraw || b.painted != 2 {
		t.Fatalf("painted %d times once the size held, asking for a frame %v, want 2 and false", b.painted, redraw)
	}
	if got := b.byID["p"].key.size; got != size {
		t.Fatalf("painted for %v, want %v", got, size)
	}
}
