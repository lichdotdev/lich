package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"slices"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/vector"

	"github.com/lichdotdev/lich/native/ui/icons"
)

// A sketch on the page: with the bar's Draw on, the pointer stops reaching
// the page and a drag draws a stroke over it. The strokes are the pane's,
// like the notes' outlines, and reach the agent with the notes as a picture:
// the frame the pane shows with the strokes on it.

var (
	icBrowserDraw  = icons.Lucide("pencil")
	icBrowserErase = icons.Lucide("eraser")
)

const (
	// sketchLine and sketchHalo are a stroke and the white under it, which
	// keeps it readable on a page of its own color, in the page's CSS pixels.
	sketchLine = 3
	sketchHalo = 7
	// sketchStep is how far, in CSS pixels, the pointer moves before a
	// stroke takes another point.
	sketchStep = 1.5
	// sketchEraseReach is how near a stroke, in CSS pixels, the eraser takes
	// it: wider than the stroke, since a mouse rarely lands on a 3 pixel line.
	sketchEraseReach = 8
	// sketchMax is the longest side of the picture a sketch sends: the size of
	// the screenshots the agent takes itself (shotMax, internal/browser).
	sketchMax = 1280
	// sketchName is the picture's file name in the session's dropped files.
	sketchName = "sketch.png"
	// sketchDiscSides is how many sides approximate a stroke's round end.
	sketchDiscSides = 16
)

// defaultInk is the pencil's color until the person picks another.
const defaultInk = "red"

// sketchStroke is one drag: its color and its points, in CSS pixels of the
// page's viewport.
type sketchStroke struct {
	ink    color.NRGBA
	points []f32.Point
}

// setDrawing turns the pencil on or off; on takes the pick's place.
func (n *browserNotes) setDrawing(on bool) {
	if on {
		n.setPicking(false)
	}
	n.drawing, n.stroking, n.erasing = on, false, false
}

// pending is whether anything waits to be sent: a note or a sketch.
func (n *browserNotes) pending() bool { return len(n.list) > 0 || len(n.strokes) > 0 }

// startStroke begins a stroke of ink at p.
func (n *browserNotes) startStroke(p f32.Point, ink color.NRGBA) {
	n.strokes = append(n.strokes, sketchStroke{ink: ink, points: []f32.Point{p}})
	n.stroking = true
}

// extendStroke takes p into the stroke being drawn, once the pointer moved
// sketchStep from its last point.
func (n *browserNotes) extendStroke(p f32.Point) {
	if !n.stroking || len(n.strokes) == 0 {
		return
	}
	s := &n.strokes[len(n.strokes)-1]
	last := s.points[len(s.points)-1]
	if math.Hypot(float64(p.X-last.X), float64(p.Y-last.Y)) >= sketchStep {
		s.points = append(s.points, p)
	}
}

// endStroke closes the stroke being drawn; a press that never moved drew
// nothing and leaves none.
func (n *browserNotes) endStroke() {
	if n.stroking && len(n.strokes) > 0 && len(n.strokes[len(n.strokes)-1].points) < 2 {
		n.undoStroke()
	}
	n.stroking = false
}

func (n *browserNotes) undoStroke() {
	if len(n.strokes) > 0 {
		n.strokes = n.strokes[:len(n.strokes)-1]
	}
	n.stroking = false
}

// eraseAt takes off every stroke that passes within sketchEraseReach of p,
// whole: a stroke is one drag, and the eraser never cuts one in two.
func (n *browserNotes) eraseAt(p f32.Point) {
	n.strokes = slices.DeleteFunc(n.strokes, func(s sketchStroke) bool {
		for i := 1; i < len(s.points); i++ {
			if segmentDistance(p, s.points[i-1], s.points[i]) <= sketchEraseReach {
				return true
			}
		}
		return false
	})
}

// segmentDistance is how far p stands from the segment a-b.
func segmentDistance(p, a, b f32.Point) float64 {
	d, ap := b.Sub(a), p.Sub(a)
	t := float32(0)
	if l := d.X*d.X + d.Y*d.Y; l > 0 {
		t = min(max((ap.X*d.X+ap.Y*d.Y)/l, 0), 1)
	}
	near := a.Add(d.Mul(t))
	return math.Hypot(float64(p.X-near.X), float64(p.Y-near.Y))
}

// keepSketchOn drops the strokes once the pane shows another page than url,
// the one they were drawn on: they point at what was on screen.
func (n *browserNotes) keepSketchOn(url string) {
	switch {
	case len(n.strokes) == 0:
		n.sketchURL = ""
	case n.sketchURL == "":
		n.sketchURL = url
	case n.sketchURL != url:
		n.strokes, n.sketchURL, n.stroking = nil, "", false
	}
}

// sketchPointer handles the pointer while drawing: a drag of the primary
// button is a stroke, or with the eraser on takes off the strokes it passes
// over, as a click does. Nothing reaches the page.
func (u *window) sketchPointer(gtx C, p *browserPaneUI, session string, x, y float64, e pointer.Event) {
	n := &p.notes
	at := f32.Pt(float32(x), float32(y))
	switch e.Kind {
	case pointer.Press:
		gtx.Execute(key.FocusCmd{Tag: &p.tag})
		u.layoutStage.focus = browserPaneOf(session)
		switch {
		case !e.Buttons.Contain(pointer.ButtonPrimary):
		case n.erasing:
			n.stroking = true
			n.eraseAt(at)
		default:
			n.startStroke(at, cardColors[u.notesBar.inkName()])
		}
	case pointer.Drag:
		if n.erasing && n.stroking {
			n.eraseAt(at)
		} else {
			n.extendStroke(at)
		}
	case pointer.Release, pointer.Cancel:
		n.endStroke()
	}
}

// paintSketch draws the strokes over the page, every halo under every line.
func paintSketch(gtx C, strokes []sketchStroke, scale float32) {
	for _, s := range strokes {
		paint.FillShape(gtx.Ops, noteHalo, clip.Stroke{Path: strokePath(gtx.Ops, s.points, scale), Width: sketchHalo * scale}.Op())
	}
	for _, s := range strokes {
		paint.FillShape(gtx.Ops, s.ink, clip.Stroke{Path: strokePath(gtx.Ops, s.points, scale), Width: sketchLine * scale}.Op())
	}
}

func strokePath(ops *op.Ops, points []f32.Point, scale float32) clip.PathSpec {
	var path clip.Path
	path.Begin(ops)
	path.MoveTo(points[0].Mul(scale))
	for _, p := range points[1:] {
		path.LineTo(p.Mul(scale))
	}
	return path.End()
}

// sketchPicture is frame, the page as the pane shows it, with strokes drawn
// on it, its longest side at most sketchMax. viewW is the page's width in
// the CSS pixels the strokes are in.
func sketchPicture(frame image.Image, strokes []sketchStroke, viewW int) *image.RGBA {
	b := frame.Bounds()
	k := min(1, float64(sketchMax)/float64(max(b.Dx(), b.Dy())))
	dst := image.NewRGBA(image.Rect(0, 0, max(int(float64(b.Dx())*k), 1), max(int(float64(b.Dy())*k), 1)))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), frame, b, xdraw.Src, nil)
	css := float32(dst.Bounds().Dx()) / float32(max(viewW, 1))
	for _, s := range strokes {
		inkStroke(dst, s.points, css, sketchHalo, noteHalo)
	}
	for _, s := range strokes {
		inkStroke(dst, s.points, css, sketchLine, s.ink)
	}
	return dst
}

// inkStroke fills the stroke through points, width CSS pixels wide with
// round ends and joins, as one shape: a disc at each point and a quad between
// two. Every piece winds the same way, so where they overlap they add up to
// full cover instead of cancelling out.
func inkStroke(dst *image.RGBA, points []f32.Point, css, width float32, c color.Color) {
	size := dst.Bounds().Size()
	r := vector.NewRasterizer(size.X, size.Y)
	half := width * css / 2
	for i, p := range points {
		p = p.Mul(css)
		for side := range sketchDiscSides {
			a := -2 * math.Pi * float64(side) / sketchDiscSides
			x, y := p.X+half*float32(math.Cos(a)), p.Y+half*float32(math.Sin(a))
			if side == 0 {
				r.MoveTo(x, y)
			} else {
				r.LineTo(x, y)
			}
		}
		r.ClosePath()
		if i == 0 {
			continue
		}
		from := points[i-1].Mul(css)
		d := p.Sub(from)
		l := float32(math.Hypot(float64(d.X), float64(d.Y)))
		if l == 0 {
			continue
		}
		normal := f32.Pt(-d.Y, d.X).Mul(half / l)
		r.MoveTo(from.X+normal.X, from.Y+normal.Y)
		r.LineTo(p.X+normal.X, p.Y+normal.Y)
		r.LineTo(p.X-normal.X, p.Y-normal.Y)
		r.LineTo(from.X-normal.X, from.Y-normal.Y)
		r.ClosePath()
	}
	r.Draw(dst, dst.Bounds(), image.NewUniform(c), image.Point{})
}

// sentSketch is the sketch a send took: the strokes, and the frame and page
// width they were drawn over.
type sentSketch struct {
	strokes []sketchStroke
	frame   image.Image
	viewW   int
	url     string
}

// uploadSketch stores the sketch's picture among the session's dropped
// files, which a confined session reads too, and answers with its path.
func (u *window) uploadSketch(session string, s sentSketch) (string, error) {
	if s.frame == nil {
		return "", errors.New("the page has not drawn yet")
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, sketchPicture(s.frame, s.strokes, s.viewW)); err != nil {
		return "", fmt.Errorf("encode the sketch: %w", err)
	}
	a, err := u.client.UploadDrop(u.ctx, session, sketchName, &buf)
	if err != nil {
		return "", err
	}
	return a.Path, nil
}
