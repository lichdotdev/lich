package ui

import (
	"image"
	"slices"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
)

// motionRig lays one widget out per frame on a clock it owns.
type motionRig struct {
	router input.Router
	now    time.Time
}

func (r *motionRig) frame(w layout.Widget) D {
	var ops op.Ops
	gtx := layout.Context{Ops: &ops, Now: r.now, Source: r.router.Source(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: image.Pt(200, 200)}}
	d := w(gtx)
	r.router.Frame(&ops)
	return d
}

func (r *motionRig) at(after time.Duration, w layout.Widget) D {
	r.now = r.now.Add(after)
	return r.frame(w)
}

func (r *motionRig) click(p image.Point) {
	r.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: pt(p)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: pt(p)},
	)
}

// clickBox is a 100x80 clickable that counts its clicks.
type clickBox struct {
	click  widget.Clickable
	clicks int
}

func (b *clickBox) Layout(gtx C) D {
	if b.click.Clicked(gtx) {
		b.clicks++
	}
	return b.click.Layout(gtx, func(gtx C) D { return D{Size: image.Pt(100, 80)} })
}

func TestCollapseFoldsOverItsDuration(t *testing.T) {
	var r motionRig
	var c Collapse
	var b clickBox
	open := true
	fold := func(gtx C) D { return c.Layout(gtx, open, b.Layout) }
	if d := r.frame(fold); d.Size.Y != 80 {
		t.Fatalf("drawn open: height %d, want 80 at once", d.Size.Y)
	}
	open = false
	r.frame(fold)
	if h := r.at(100*time.Millisecond, fold).Size.Y; h <= 0 || h >= 80 {
		t.Errorf("half way: height %d, want between 0 and 80", h)
	}
	if h := r.at(100*time.Millisecond, fold).Size.Y; h != 0 {
		t.Errorf("after 200ms: height %d, want 0", h)
	}
}

// Folding shut, the child still draws but a click on it lands nowhere.
func TestCollapseIsInertWhileFolding(t *testing.T) {
	var r motionRig
	var c Collapse
	var b clickBox
	open := true
	fold := func(gtx C) D { return c.Layout(gtx, open, b.Layout) }
	r.frame(fold)
	r.click(image.Pt(10, 10))
	r.frame(fold)
	open = false
	r.frame(fold)
	r.click(image.Pt(10, 10))
	r.at(50*time.Millisecond, fold)
	r.frame(fold)
	if b.clicks != 1 {
		t.Errorf("%d clicks, want the open one alone", b.clicks)
	}
}

func TestPresenceHoldsALeavingKeyInItsSlot(t *testing.T) {
	var p Presence
	t0 := time.Unix(0, 0)
	p.Update(t0, []string{"a", "b", "c"})
	if got := p.Update(t0, []string{"a", "c"}); !slices.Equal(got, []string{"a", "b", "c"}) || !p.Leaving("b") {
		t.Errorf("b closed: shown %v, leaving %v; want b kept in its slot", got, p.Leaving("b"))
	}
	if got := p.Update(t0.Add(PresenceExit-time.Millisecond), []string{"a", "c"}); !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Errorf("just before the exit ends: %v, want b still drawn", got)
	}
	if got := p.Update(t0.Add(PresenceExit), []string{"a", "c"}); !slices.Equal(got, []string{"a", "c"}) {
		t.Errorf("after the exit: %v, want b gone", got)
	}
}

func TestPresenceKeyComingBackStopsLeaving(t *testing.T) {
	var p Presence
	t0 := time.Unix(0, 0)
	p.Update(t0, []string{"a", "b"})
	p.Update(t0, []string{"a"})
	if got := p.Update(t0.Add(PresenceExit), []string{"a", "b"}); !slices.Equal(got, []string{"a", "b"}) || p.Leaving("b") {
		t.Errorf("b back: %v, leaving %v; want b live", got, p.Leaving("b"))
	}
}

// Only a key arriving after the list first drew grows in; the first ones are
// drawn whole.
func TestPresenceGrowsInOnlyWhatArrives(t *testing.T) {
	var r motionRig
	var p Presence
	var b clickBox
	item := func(key string) layout.Widget {
		return func(gtx C) D { return p.Layout(gtx, key, b.Layout) }
	}
	p.Update(r.now, []string{"a"})
	if h := r.frame(item("a")).Size.Y; h != 80 {
		t.Errorf("first key: height %d, want 80", h)
	}
	p.Update(r.now, []string{"a", "b"})
	if h := r.frame(item("b")).Size.Y; h != 0 {
		t.Errorf("arriving key, first frame: height %d, want 0", h)
	}
	if h := r.at(80*time.Millisecond, item("b")).Size.Y; h <= 0 || h >= 80 {
		t.Errorf("arriving key at 80ms: height %d, want between", h)
	}
	if h := r.at(80*time.Millisecond, item("b")).Size.Y; h != 80 {
		t.Errorf("arriving key at 160ms: height %d, want 80", h)
	}
}

func TestPresenceLeavingItemShrinksAndTakesNoInput(t *testing.T) {
	var r motionRig
	var p Presence
	var b clickBox
	item := func(gtx C) D { return p.Layout(gtx, "a", b.Layout) }
	p.Update(r.now, []string{"a"})
	r.frame(item)
	p.Update(r.now, nil)
	r.frame(item)
	r.click(image.Pt(10, 10))
	if h := r.at(90*time.Millisecond, item).Size.Y; h <= 0 || h >= 80 {
		t.Errorf("leaving at 90ms: height %d, want between", h)
	}
	r.frame(item)
	if b.clicks != 0 {
		t.Error("a leaving item took a click")
	}
	if h := r.at(90*time.Millisecond, item).Size.Y; h != 0 {
		t.Errorf("leaving at 180ms: height %d, want 0", h)
	}
}
