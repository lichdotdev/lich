package main

import (
	"crypto/sha256"
	"fmt"
	"image"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

// maxSettleFrames bounds the frames a rig waits for its screen to go still:
// far more than any transition in the window takes at a second a frame.
const maxSettleFrames = 30

const (
	// quickClick is how long the mouse rests on a control before a click,
	// well under ui's 600ms tooltip delay.
	quickClick = 300 * time.Millisecond
	frameStep  = 16 * time.Millisecond
)

// frameRig lays a screen out through a real input router, one frame per
// call, and reads back what each frame drew: a digest of its paint, then its
// semantic tree, the labels, their bounds, and the selected and disabled
// states.
type frameRig struct {
	router input.Router
	ops    op.Ops
	now    time.Time
	size   image.Point
	theme  *ui.Theme
	draw   func(C, *ui.Theme)
	// asked is an invalidate call since the last frame, for a screen that
	// asks for frames through the window rather than through gtx.
	asked atomic.Bool
}

func newFrameRig(size image.Point, draw func(C, *ui.Theme)) *frameRig {
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Background}
	return &frameRig{now: time.Unix(0, 0), size: size, theme: th, draw: draw}
}

// frame lays the screen out once and reports what it drew, and whether it
// asked for another frame now.
func (r *frameRig) frame() (drawn string, redraw bool) {
	r.ops.Reset()
	gtx := layout.Context{Ops: &r.ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(r.size), Source: r.router.Source(), Now: r.now}
	r.draw(gtx, r.theme)
	r.router.Frame(&r.ops)
	at, wake := r.router.WakeupTime()
	asked := r.asked.Swap(false)
	return r.drawn(), asked || wake && !at.After(r.now)
}

// invalidate stands in for the window's Invalidate.
func (r *frameRig) invalidate() { r.asked.Store(true) }

func (r *frameRig) drawn() string {
	var b strings.Builder
	fmt.Fprintf(&b, "paint %x\n", sha256.Sum256(reflect.ValueOf(&r.ops.Internal).Elem().FieldByName("data").Bytes()))
	for _, n := range r.nodes() {
		d := n.Desc
		fmt.Fprintf(&b, "%q %q %v sel=%v dis=%v\n", d.Label, d.Description, d.Bounds, d.Selected, d.Disabled)
	}
	return b.String()
}

// settle frames the screen, a second apart, until it draws the same thing
// twice and asks for no frame.
func (r *frameRig) settle(t *testing.T) string {
	t.Helper()
	last, _ := r.frame()
	for range maxSettleFrames {
		r.now = r.now.Add(time.Second)
		drawn, redraw := r.frame()
		if drawn == last && !redraw {
			return drawn
		}
		last = drawn
	}
	t.Fatal("the screen never went still")
	return ""
}

// waitFor frames the screen until something is drawn with label, for a
// screen that reads its backend off the frame, then lets it settle.
func (r *frameRig) waitFor(t *testing.T, label string) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !r.shows(label); time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("nothing drawn as %q:\n%s", label, r.drawn())
		}
		r.now = r.now.Add(time.Second)
		r.frame()
	}
	r.settle(t)
}

func (r *frameRig) shows(label string) bool {
	for _, n := range r.nodes() {
		if n.Desc.Label == label {
			return true
		}
	}
	return false
}

// nodes is the semantic tree of the last frame, flattened.
func (r *frameRig) nodes() []input.SemanticNode { return r.router.AppendSemantics(nil) }

// smallestClickIn is the centre of the smallest clickable drawn inside
// within, for a button with an icon and no label.
func (r *frameRig) smallestClickIn(t *testing.T, within image.Rectangle) f32.Point {
	t.Helper()
	var best image.Rectangle
	for _, n := range r.nodes() {
		b := n.Desc.Bounds
		if n.Desc.Gestures&input.ClickGesture != 0 && b.In(within) && (best.Empty() || area(b) < area(best)) {
			best = b
		}
	}
	if best.Empty() {
		t.Fatalf("nothing clickable drawn in %v:\n%s", within, r.drawn())
	}
	return centre(best)
}

// controlOf is the centre of the smallest clickable on the same line as the
// first thing drawn with label and right of it: a settings row's control
// beside its title.
func (r *frameRig) controlOf(t *testing.T, label string) f32.Point {
	t.Helper()
	for _, n := range r.nodes() {
		if n.Desc.Label != label {
			continue
		}
		at := n.Desc.Bounds.Min
		return r.smallestClickIn(t, image.Rect(at.X+1, at.Y-rowReach, r.size.X, at.Y+3*rowReach))
	}
	t.Fatalf("nothing drawn as %q:\n%s", label, r.drawn())
	return f32.Point{}
}

// rowReach is how far above a row's title its control may start: a control
// is centred on a row that is taller than the title's line.
const rowReach = 20

func centre(b image.Rectangle) f32.Point {
	c := b.Min.Add(b.Max).Div(2)
	return f32.Pt(float32(c.X), float32(c.Y))
}

// at is a point on the first thing drawn with label: the centre of the
// smallest clickable under its first glyph, else that glyph. A label's own
// bounds run on to the edge of its constraints, so its centre can miss it.
func (r *frameRig) at(t *testing.T, label string) f32.Point {
	t.Helper()
	for _, n := range r.nodes() {
		if n.Desc.Label != label {
			continue
		}
		glyph := n.Desc.Bounds.Min.Add(image.Pt(2, 2))
		if b, ok := r.smallestClickAt(glyph); ok {
			return centre(b)
		}
		return f32.Pt(float32(glyph.X), float32(glyph.Y))
	}
	t.Fatalf("nothing drawn as %q:\n%s", label, r.drawn())
	return f32.Point{}
}

func (r *frameRig) smallestClickAt(p image.Point) (image.Rectangle, bool) {
	var best image.Rectangle
	for _, n := range r.nodes() {
		b := n.Desc.Bounds
		if n.Desc.Gestures&input.ClickGesture != 0 && p.In(b) && (best.Empty() || area(b) < area(best)) {
			best = b
		}
	}
	return best, !best.Empty()
}

func area(b image.Rectangle) int { return b.Dx() * b.Dy() }

// hover brings the mouse to p and lets the hover settle in the time a
// quick click takes, short of a tooltip's delay: a tooltip that came up
// would ask for frames of its own on the press and hide what the click
// left undone.
func (r *frameRig) hover(p f32.Point) {
	r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: p})
	for start := r.now; r.now.Sub(start) < quickClick; r.now = r.now.Add(frameStep) {
		if _, redraw := r.frame(); !redraw {
			return
		}
	}
}

// click presses and releases the primary button at p and reports what the
// frame that took the click drew, and what the frame after it drew.
// Gio draws one frame per event, so the two differ only when the click
// changed state the frame had already read; such a frame must ask for the
// next one, or the change waits for the next mouse move.
func (r *frameRig) click(t *testing.T, p f32.Point) (took, next string) {
	t.Helper()
	r.hover(p)
	r.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: p},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: p},
	)
	took, redraw := r.frame()
	next, _ = r.frame()
	if took != next && !redraw {
		t.Errorf("the frame that took the click drew the old state and asked for no next frame:\ntook:\n%s\nnext:\n%s", took, next)
	}
	return took, next
}

// clickLabel clicks the first thing drawn with label.
func (r *frameRig) clickLabel(t *testing.T, label string) (took, next string) {
	t.Helper()
	r.settle(t)
	return r.click(t, r.at(t, label))
}
