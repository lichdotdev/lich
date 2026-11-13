package main

import (
	"image"
	"image/color"
	"math"
	"slices"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/lichdotdev/lich/native/ui/icons"
)

// The sketch's eraser: its drag leaves a trail that fades behind the pointer,
// every stroke the trail touches goes faint, and they all go when the button
// lets go. A click takes the stroke under it the same way.

var icBrowserErase = icons.Lucide("eraser")

const (
	// sketchEraseReach is how near a stroke, in CSS pixels, the eraser takes
	// it: wider than the stroke, since a mouse rarely lands on a 3 pixel line.
	sketchEraseReach = 8
	// sketchDoomedOpacity draws a stroke the eraser touched.
	sketchDoomedOpacity = 0.25
	// rubTrailFor is how long a point of the eraser's trail shows, and
	// rubTrailWide and rubTrailThin its width, in CSS pixels, from the pointer
	// to its end.
	rubTrailFor  = 220 * time.Millisecond
	rubTrailWide = 10
	rubTrailThin = 2
	// rubRing is the radius of the eraser's ring around the pointer.
	rubRing = 7
)

// The trail and the ring are neither lich's nor the pencil's: grays that read
// on a light page and on a dark one.
var (
	rubTrailColor = color.NRGBA{R: 120, G: 120, B: 128, A: 0x73}
	rubRingFill   = color.NRGBA{R: 255, G: 255, B: 255, A: 0xb3}
	rubRingEdge   = color.NRGBA{R: 40, G: 40, B: 48, A: 0xcc}
)

// rubPoint is one point of the eraser's trail, in CSS pixels, and when the
// pointer was there.
type rubPoint struct {
	at f32.Point
	t  time.Time
}

// rubPointer is the eraser's side of the pointer: it follows it for the ring,
// and a drag of the primary button, or a click, dooms what it passes over
// until the release takes it off.
func (n *browserNotes) rubPointer(gtx C, at f32.Point, e pointer.Event) {
	n.rubAt, n.rubOver = at, e.Kind != pointer.Leave
	switch e.Kind {
	case pointer.Press:
		if e.Buttons.Contain(pointer.ButtonPrimary) {
			n.stroking = true
			n.doomAt(at)
		}
	case pointer.Drag:
		if n.stroking {
			n.rubTrail = append(n.rubTrail, rubPoint{at: at, t: gtx.Now})
			n.doomAt(at)
		}
	case pointer.Release:
		n.erase()
	case pointer.Cancel:
		n.spare()
	}
}

// doomAt marks every stroke that passes within sketchEraseReach of p, whole:
// a stroke is one drag, and the eraser never cuts one in two.
func (n *browserNotes) doomAt(p f32.Point) {
	for i := range n.strokes {
		s := &n.strokes[i]
		for j := 1; j < len(s.points) && !s.doomed; j++ {
			s.doomed = segmentDistance(p, s.points[j-1], s.points[j]) <= sketchEraseReach
		}
	}
}

// erase takes the doomed strokes off, for undoSketch to bring back.
func (n *browserNotes) erase() {
	n.stroking = false
	if !slices.ContainsFunc(n.strokes, func(s sketchStroke) bool { return s.doomed }) {
		return
	}
	kept := slices.DeleteFunc(slices.Clone(n.strokes), func(s sketchStroke) bool { return s.doomed })
	n.spare()
	n.checkpoint()
	n.strokes = kept
}

// spare lets every doomed stroke stay: the drag that marked them ended
// without a release.
func (n *browserNotes) spare() {
	for i := range n.strokes {
		n.strokes[i].doomed = false
	}
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

// trailEdges are the two sides of a ribbon through points, half[i] to each
// side of points[i], square to the way the trail runs there. The trail is
// filled as that one shape: a stroke a segment, each its own width, showed
// every step between them and darkened where two overlapped.
func trailEdges(points []f32.Point, half []float32) (left, right []f32.Point) {
	left, right = make([]f32.Point, len(points)), make([]f32.Point, len(points))
	var normal f32.Point
	for i, p := range points {
		d := points[min(i+1, len(points)-1)].Sub(points[max(i-1, 0)])
		if l := float32(math.Hypot(float64(d.X), float64(d.Y))); l > 0 {
			normal = f32.Pt(-d.Y/l, d.X/l)
		}
		left[i], right[i] = p.Add(normal.Mul(half[i])), p.Sub(normal.Mul(half[i]))
	}
	return left, right
}

// paintEraser draws the eraser's trail, thinning out behind the pointer as
// its points age, and its ring.
func (n *browserNotes) paintEraser(gtx C, scale float32) {
	n.rubTrail = slices.DeleteFunc(n.rubTrail, func(p rubPoint) bool { return gtx.Now.Sub(p.t) >= rubTrailFor })
	if !n.erasing {
		n.rubTrail = nil
		return
	}
	if len(n.rubTrail) > 1 {
		points, half := make([]f32.Point, len(n.rubTrail)), make([]float32, len(n.rubTrail))
		for i, p := range n.rubTrail {
			fresh := 1 - progress(gtx.Now.Sub(p.t), rubTrailFor)
			points[i], half[i] = p.at.Mul(scale), (rubTrailThin+(rubTrailWide-rubTrailThin)*fresh)*scale/2
		}
		left, right := trailEdges(points, half)
		var path clip.Path
		path.Begin(gtx.Ops)
		path.MoveTo(left[0])
		for _, p := range left[1:] {
			path.LineTo(p)
		}
		for _, p := range slices.Backward(right) {
			path.LineTo(p)
		}
		path.Close()
		paint.FillShape(gtx.Ops, rubTrailColor, clip.Outline{Path: path.End()}.Op())
	}
	if len(n.rubTrail) > 0 {
		gtx.Execute(op.InvalidateCmd{})
	}
	if !n.rubOver {
		return
	}
	center, r := n.rubAt.Mul(scale).Round(), int(rubRing*scale)
	ring := clip.Ellipse{Min: center.Sub(image.Pt(r, r)), Max: center.Add(image.Pt(r, r))}
	paint.FillShape(gtx.Ops, rubRingFill, ring.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, rubRingEdge, clip.Stroke{Path: ring.Path(gtx.Ops), Width: 1.5 * scale}.Op())
}
