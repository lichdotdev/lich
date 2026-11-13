package main

import (
	"image"
	"image/color"
	"image/draw"
	"testing"

	"gioui.org/f32"

	"github.com/lichdotdev/lich/api"
)

func TestASketchPictureIsTheFrameWithTheStrokesOnItAtScreenshotSize(t *testing.T) {
	frame := image.NewRGBA(image.Rect(0, 0, 2560, 1600))
	draw.Draw(frame, frame.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)
	ink := color.NRGBA{R: 255, A: 255}
	// The page is 1280 CSS pixels wide on a 2x screen; the stroke turns
	// sharply at 200,100.
	strokes := []sketchStroke{{ink: ink, points: []f32.Point{{X: 100, Y: 100}, {X: 200, Y: 100}, {X: 200, Y: 200}}}}

	got := sketchPicture(frame, strokes, 1280)

	if size := got.Bounds().Size(); size != image.Pt(1280, 800) {
		t.Fatalf("picture is %v, want 1280x800", size)
	}
	red := color.RGBA{R: 255, A: 255}
	for _, p := range []image.Point{{X: 150, Y: 100}, {X: 200, Y: 100}, {X: 200, Y: 150}} {
		if c := got.RGBAAt(p.X, p.Y); c != red {
			t.Errorf("pixel %v on the stroke is %v, want the ink %v", p, c, red)
		}
	}
	if c := got.RGBAAt(150, 102); c.R < 150 || c.G < 150 || c.B < 150 {
		t.Errorf("pixel beside the stroke is %v, want the white halo over the page", c)
	}
	if c := got.RGBAAt(600, 400); c != (color.RGBA{A: 255}) {
		t.Errorf("pixel away from the stroke is %v, want the frame's black", c)
	}
}

func TestASketchPictureKeepsAFrameAlreadySmall(t *testing.T) {
	frame := image.NewRGBA(image.Rect(0, 0, 800, 600))
	if size := sketchPicture(frame, nil, 800).Bounds().Size(); size != image.Pt(800, 600) {
		t.Fatalf("picture is %v, want the frame's 800x600", size)
	}
}

func TestTheSketchFollowsTheNotesInTheMessage(t *testing.T) {
	notes := []*browserNote{{el: api.BrowserElement{Label: "nav", Selector: "nav", HTML: "<nav></nav>", URL: "http://localhost:5173/"}, text: "taller"}}
	got := sketchedMessage(notes, "/drops/s1/sketch.png", "http://localhost:5173/", "before the demo")
	want := `[lich] The person left a note on the page in your browser tab:
1. nav on http://localhost:5173/
   nav
   <nav></nav>
   Note: taller
The person also drew on the page. The screenshot with their drawing: /drops/s1/sketch.png
Each selector above works as the selector of browser_click and browser_type.

before the demo`
	if got != want {
		t.Fatalf("message:\n%s\nwant:\n%s", got, want)
	}
}

func TestASketchAloneNamesItsPage(t *testing.T) {
	got := sketchedMessage(nil, "/drops/s1/sketch.png", "http://localhost:5173/pricing", "")
	want := "[lich] The person drew on the page in your browser tab, http://localhost:5173/pricing. The screenshot with their drawing: /drops/s1/sketch.png"
	if got != want {
		t.Fatalf("message:\n%s\nwant:\n%s", got, want)
	}
}

func TestAPressThatNeverMovedLeavesNoStroke(t *testing.T) {
	n := newBrowserNotes()
	n.startStroke(f32.Pt(10, 10), color.NRGBA{A: 255})
	n.extendStroke(f32.Pt(10.5, 10))
	n.endStroke()
	if len(n.strokes) != 0 {
		t.Fatalf("a press without a drag left %d strokes", len(n.strokes))
	}
	n.startStroke(f32.Pt(10, 10), color.NRGBA{A: 255})
	n.extendStroke(f32.Pt(40, 10))
	n.endStroke()
	n.extendStroke(f32.Pt(80, 10))
	if len(n.strokes) != 1 || len(n.strokes[0].points) != 2 {
		t.Fatalf("strokes after a drag and a move with no button: %+v", n.strokes)
	}
	n.undoStroke()
	if n.pending() {
		t.Fatal("the undone stroke still waits")
	}
}

func TestTheSketchGoesWithThePageItWasDrawnOn(t *testing.T) {
	n := newBrowserNotes()
	n.startStroke(f32.Pt(10, 10), color.NRGBA{A: 255})
	n.extendStroke(f32.Pt(40, 10))
	n.keepSketchOn("http://localhost:5173/pricing")
	n.keepSketchOn("http://localhost:5173/pricing")
	if len(n.strokes) != 1 {
		t.Fatal("the sketch left while its page was still shown")
	}
	n.keepSketchOn("http://localhost:5173/docs")
	if len(n.strokes) != 0 {
		t.Fatal("the sketch stayed over another page")
	}
}

func TestThePencilAndThePickTakeEachOthersPlace(t *testing.T) {
	n := newBrowserNotes()
	n.setPicking(true)
	n.setDrawing(true)
	if n.picking || !n.drawing {
		t.Fatalf("after Draw: picking %v, drawing %v", n.picking, n.drawing)
	}
	n.setPicking(true)
	if !n.picking || n.drawing {
		t.Fatalf("after Select: picking %v, drawing %v", n.picking, n.drawing)
	}
}

func TestTheEraserTakesWholeStrokesItPassesNearAndLeavesTheRest(t *testing.T) {
	n := newBrowserNotes()
	black := color.NRGBA{A: 255}
	n.strokes = []sketchStroke{
		{ink: black, points: []f32.Point{{X: 10, Y: 10}, {X: 110, Y: 10}}},
		{ink: black, points: []f32.Point{{X: 10, Y: 60}, {X: 110, Y: 60}}},
		{ink: black, points: []f32.Point{{X: 60, Y: 0}, {X: 60, Y: 12}}},
	}
	n.eraseAt(f32.Pt(200, 200))
	if len(n.strokes) != 3 {
		t.Fatalf("erasing away from every stroke left %d of 3", len(n.strokes))
	}
	// 6 pixels under the first stroke's middle, on the third's end: both go,
	// by their whole length; the second, 44 pixels away, stays.
	n.eraseAt(f32.Pt(60, 16))
	if len(n.strokes) != 1 || n.strokes[0].points[0].Y != 60 {
		t.Fatalf("strokes left: %+v, want only the one at y=60", n.strokes)
	}
	// Past a stroke's end by more than the reach is not on it.
	n.eraseAt(f32.Pt(125, 60))
	if len(n.strokes) != 1 {
		t.Fatal("the eraser took a stroke it stood beyond the end of")
	}
}
