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

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
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
	f.paint(img, rows, cover, image.Pt(cols/2, rows/2), image.Rectangle{}, &inks)
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

// Two stirred panes paint their field once per step of their clock: frames
// in between reuse it, the next step paints both again, and the frame asks
// for that step.
func TestPaneBackdropMovesOncePerStep(t *testing.T) {
	u, r, _ := besideFrames(t, image.Pt(1200, 800), func(m *model) {
		m.splitBeside(context.Background(), "orch", "wa", edgeRight, false)
	})
	for _, id := range []string{"orch", "wa"} {
		if _, ok := u.backdrops.byID[id]; !ok {
			t.Fatalf("pane %s has no backdrop", id)
		}
		u.backdrops.stirOf(id).until = r.now.Add(time.Hour)
	}
	r.frame()
	r.now = r.now.Add(backdropStep / 4)
	r.frame()
	e := u.backdrops.byID["orch"]
	left := backdropStep - e.clock.moved%backdropStep
	painted := u.backdrops.painted
	r.now = r.now.Add(left / 2)
	r.frame()
	if u.backdrops.painted != painted {
		t.Fatalf("painted %d times inside one step, want %d", u.backdrops.painted, painted)
	}
	left -= left / 2
	// frame already took the wakeup; the time it asked for stays
	if at, _ := r.router.WakeupTime(); !at.Equal(r.now.Add(left)) {
		t.Fatalf("asked for a frame at %v, want the next step at %v", at, r.now.Add(left))
	}
	r.now = r.now.Add(left)
	r.frame()
	if u.backdrops.painted != painted+2 {
		t.Fatalf("painted %d at the next step, want %d", u.backdrops.painted, painted+2)
	}
}

// A backdrop that does not move holds its frame and asks for none; moving
// again, it goes on from where it stopped.
func TestBackdropHoldsStillAndGoesOn(t *testing.T) {
	var b backdrops
	moving := true
	r := newFrameRig(image.Pt(400, 300), func(gtx C, th *ui.Theme) {
		b.layout(gtx, th, "p", image.Pt(300, 200), th.Background, 1, backdropMark{}, moving)
	})
	r.frame()
	r.now = r.now.Add(backdropStep)
	r.frame()
	moving = false
	r.frame()
	held, painted := b.byID["p"].clock.moved, b.painted
	r.now = r.now.Add(time.Minute)
	r.frame()
	if at, ok := backdropNext(r.now, held, false, false); ok {
		t.Fatalf("a still backdrop asked for a frame at %v", at)
	}
	if b.byID["p"].clock.moved != held || b.painted != painted {
		t.Fatalf("a still backdrop moved to %v and painted %d, want %v and %d", b.byID["p"].clock.moved, b.painted, held, painted)
	}
	moving = true
	r.frame()
	r.now = r.now.Add(backdropStep)
	r.frame()
	if got := b.byID["p"].clock.moved; got != held+backdropStep {
		t.Fatalf("moving again it went to %v, want %v on from %v", got, held+backdropStep, held)
	}
}

// A session's backdrop moves while its agent works or its user types, and
// backdropAwake after the last of either; a chat's start screen stirs it
// for as long as it is up.
func TestStirKeepsMovingAfterWork(t *testing.T) {
	var s backdropStir
	now := time.Unix(100, 0)
	s.keep(now, false, 0, false)
	if s.until.After(now) {
		t.Fatal("an idle session stirred its backdrop")
	}
	s.keep(now, true, 0, false)
	done := now.Add(3 * time.Second)
	s.keep(done, true, 0, false)
	s.keep(done.Add(time.Second), false, 0, false)
	if !s.until.Equal(done.Add(backdropAwake)) {
		t.Fatalf("stirred until %v, want backdropAwake after the turn's end %v", s.until, done.Add(backdropAwake))
	}
	typed := done.Add(time.Minute)
	s.keep(typed, false, 3, false)
	if !s.until.Equal(typed.Add(backdropAwake)) {
		t.Fatalf("typing stirred it until %v, want %v", s.until, typed.Add(backdropAwake))
	}
	opened := typed.Add(time.Minute)
	s.keep(opened, false, 3, true)
	later := opened.Add(time.Hour)
	s.keep(later, false, 3, true)
	if !s.until.Equal(later.Add(backdropAwake)) {
		t.Fatalf("a start screen up for an hour stirred it until %v, want %v", s.until, later.Add(backdropAwake))
	}
}

// A window without focus moves its backdrop at the still pace.
func TestStillWindowMovesItsBackdropLessOften(t *testing.T) {
	var b backdrops
	r := newFrameRig(image.Pt(400, 300), func(gtx C, th *ui.Theme) {
		b.layout(gtx, th, "p", image.Pt(300, 200), th.Background, 1, backdropMark{}, true)
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
		b.layout(gtx, th, id, size, th.Background, 1, backdropMark{}, true)
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
				f.paint(img, drawn, cover, image.Pt(cols/2, rows/2), image.Rectangle{}, &inks)
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
	marks := newMarkInks(fieldInk, fieldHot, fieldGround)
	img = image.NewRGBA(image.Rect(0, 0, cols, rows))
	f.step(t, rows)
	f.paint(img, rows, 0, image.Pt(cols/2, rows/2), image.Rectangle{}, &inks)
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

// The field is inked in the theme's primary, the color of its buttons.
func TestBackdropTakesTheThemesPrimary(t *testing.T) {
	var b backdrops
	pink := color.NRGBA{R: 0xf3, G: 0x8b, B: 0xa8, A: 0xff}
	r := newFrameRig(image.Pt(400, 300), func(gtx C, th *ui.Theme) {
		b.layout(gtx, th, "p", image.Pt(300, 200), th.Background, 1, backdropMark{}, true)
	})
	r.theme.Primary = pink
	r.frame()
	e := b.byID["p"]
	if e.primary != pink || e.inks[0] == e.inks[32] {
		t.Fatalf("inks from %v, want the primary %v", e.primary, pink)
	}
	hot := mixSRGB(pink, color.NRGBA{R: 255, G: 255, B: 255, A: 255}, backdropHot)
	if want := newBackdropInks(pink, hot, r.theme.Background); e.inks != want {
		t.Fatal("the inks are not the primary's ramp")
	}
}

// The start screen's field is calm around its centre and whole at the edges.
func TestStartFieldIsCalmInTheMiddle(t *testing.T) {
	f := newBackdropField(300, 200)
	centre := image.Pt(150, 90)
	if got := f.calm(centre.X, centre.Y, centre); got != centreFloor {
		t.Fatalf("calm %v at the centre, want %v", got, centreFloor)
	}
	if got := f.calm(0, 0, centre); got != 1 {
		t.Fatalf("calm %v in a corner, want 1", got)
	}
}

// An empty chat's start screen moves its backdrop for as long as it shows.
func TestStartScreenBackdropMovesWhileItShows(t *testing.T) {
	u, r := startFrames(t, rigWindow)
	r.now = r.now.Add(2 * backdropAwake)
	r.frame()
	if until := u.backdrops.stirOf("s1").until; !r.now.Before(until) {
		t.Fatalf("the start screen stopped its backdrop at %v, still up at %v", until, r.now)
	}
}

// A chat's session reports itself busy while its chat opens, and can stay so
// with no turn running, past its start screen: the report alone does not keep the backdrop moving,
// only the chat's own turn does.
func TestABusyReportDoesNotKeepAChatsBackdropMoving(t *testing.T) {
	u, r := startFrames(t, rigWindow)
	u.model.chat.apply(api.ChatItemUpdate{SessionID: "s1", Item: backend.ChatItem{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "go"}})
	if err := u.model.applyStatus(context.Background(), statusEv(t, statusEvent{ID: "s1", State: statusBusy})); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		r.now = r.now.Add(backdropAwake)
		r.frame()
	}
	if until := u.backdrops.stirOf("s1").until; r.now.Before(until) {
		t.Fatalf("a busy report with no turn stirs the backdrop until %v at %v", until, r.now)
	}
}

// The start screen's field lulls to lineFloor at the middle of the line under
// its composer, and is whole far from it.
func TestStartFieldLullsAroundTheLine(t *testing.T) {
	line := image.Rect(100, 50, 200, 56)
	if got := lull(150, 53, line); math.Abs(got-lineFloor) > 1e-9 {
		t.Fatalf("lull %v at the line's middle, want %v", got, lineFloor)
	}
	if got := lull(150, 53+5*lineReachY, line); got < 0.999 {
		t.Fatalf("lull %v far under the line, want the whole field", got)
	}
	if got := lull(0, 0, image.Rectangle{}); got != 1 {
		t.Fatalf("lull %v with no line, want 1", got)
	}
}
