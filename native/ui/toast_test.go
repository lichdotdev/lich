package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

var toastT0 = time.Unix(1000, 0)

func liveIDs(s *toastStack) []string {
	var ids []string
	for _, e := range s.entries {
		if !e.removed() {
			ids = append(ids, e.ID)
		}
	}
	return ids
}

func TestToastWithAnUpIDIsReplacedInPlaceAndItsTimerRestarts(t *testing.T) {
	var s toastStack
	s.show(Toast{ID: "x", Title: "first"}, toastT0)
	s.show(Toast{ID: "y", Title: "other"}, toastT0)
	s.show(Toast{ID: "x", Title: "second", Variant: ToastError}, toastT0.Add(3*time.Second))

	if got := liveIDs(&s); len(got) != 2 || got[0] != "y" || got[1] != "x" {
		t.Fatalf("live = %v, want [y x]: a replace keeps its place", got)
	}
	if e := s.live("x"); e.Title != "second" || e.Variant != ToastError {
		t.Fatalf("x = %q %v, want the new content", e.Title, e.Variant)
	}
	s.advance(toastT0.Add(5 * time.Second))
	if s.live("x") == nil {
		t.Fatal("x expired 4s after its first show; the replace restarts the timer")
	}
	s.advance(toastT0.Add(7 * time.Second))
	if s.live("x") != nil {
		t.Fatal("x outlived 4s from its replace")
	}
}

func TestToastWithoutIDGetsAFreshOne(t *testing.T) {
	var s toastStack
	a := s.show(Toast{Title: "a"}, toastT0)
	b := s.show(Toast{Title: "a"}, toastT0)
	if a == "" || a == b {
		t.Fatalf("ids %q and %q, want two distinct ones", a, b)
	}
}

func TestToastTimerHoldsWhilePausedAndKeepsWhatWasLeft(t *testing.T) {
	var s toastStack
	s.show(Toast{ID: "x"}, toastT0)
	s.pause(true, toastT0.Add(time.Second))
	s.advance(toastT0.Add(time.Minute))
	if s.live("x") == nil {
		t.Fatal("expired while paused")
	}
	resumed := toastT0.Add(time.Minute)
	s.pause(false, resumed)
	s.advance(resumed.Add(2900 * time.Millisecond))
	if s.live("x") == nil {
		t.Fatal("expired before the 3s it had left")
	}
	s.advance(resumed.Add(3 * time.Second))
	if s.live("x") != nil {
		t.Fatal("still up past the 3s it had left")
	}
}

func TestToastShownWhilePausedWaitsForTheResume(t *testing.T) {
	var s toastStack
	s.pause(true, toastT0)
	s.show(Toast{ID: "x", Duration: time.Second}, toastT0)
	s.advance(toastT0.Add(time.Hour))
	if s.live("x") == nil {
		t.Fatal("expired while paused")
	}
}

func TestToastForeverNeverExpires(t *testing.T) {
	var s toastStack
	s.show(Toast{ID: "x", Duration: ToastForever}, toastT0)
	s.pause(true, toastT0.Add(time.Second))
	s.pause(false, toastT0.Add(2*time.Second))
	if wake := s.advance(toastT0.Add(24 * time.Hour)); s.live("x") == nil || !wake.IsZero() {
		t.Fatalf("live %v, wake %v: want up and nothing to wake for", s.live("x") != nil, wake)
	}
}

func TestDismissedToastStaysForItsExitThenGoes(t *testing.T) {
	var s toastStack
	s.show(Toast{ID: "x", Duration: ToastForever}, toastT0)
	s.dismiss("x", toastT0)
	if wake := s.advance(toastT0.Add(100 * time.Millisecond)); len(s.entries) != 1 || !wake.Equal(toastT0.Add(toastUnmountDelay)) {
		t.Fatalf("entries %d, wake %v: want it kept until its unmount", len(s.entries), wake)
	}
	s.advance(toastT0.Add(toastUnmountDelay))
	if len(s.entries) != 0 {
		t.Fatal("still on the stack after its exit")
	}
}

func TestAdvanceWakesAtTheNextDeadline(t *testing.T) {
	var s toastStack
	s.show(Toast{Duration: 2 * time.Second}, toastT0)
	s.show(Toast{Duration: time.Second}, toastT0)
	if wake := s.advance(toastT0); !wake.Equal(toastT0.Add(time.Second)) {
		t.Fatalf("wake = %v, want the 1s toast's deadline", wake)
	}
}

func TestCollapsedStackShowsThreeAndPeeksTheOnesBehind(t *testing.T) {
	e := &toastEntry{}
	slot := toastSlot{own: 80, front: 50, gap: 14}
	front := targetOf(e, slot)
	if front.y != 0 || front.scale != 1 || front.height != 80 || front.inside != 1 {
		t.Fatalf("front = %+v, want in place at its own height", front)
	}
	slot.index = 2
	behind := targetOf(e, slot)
	if behind.y != 28 || behind.scale != 0.9 || behind.height != 50 || behind.inside != 0 || behind.alpha != 1 {
		t.Fatalf("third = %+v, want 2 gaps down, 10%% smaller, the front's height, content hidden", behind)
	}
	slot.index = 3
	if fourth := targetOf(e, slot); fourth.alpha != 0 {
		t.Fatalf("fourth alpha = %v, want hidden: sonner shows 3", fourth.alpha)
	}
}

func TestExpandedStackSitsEachToastAtItsOffset(t *testing.T) {
	e := &toastEntry{view: toastView{offset: 94}}
	got := targetOf(e, toastSlot{index: 1, own: 60, front: 80, gap: 14, expanded: true})
	if got.y != 94 || got.scale != 1 || got.height != 60 || got.inside != 1 {
		t.Fatalf("expanded = %+v, want at its offset, full size", got)
	}
}

func TestSwipeDismissesPastTheThresholdOrFastEnough(t *testing.T) {
	cases := []struct {
		amount  float32
		elapsed time.Duration
		want    bool
	}{
		{45, time.Second, true},
		{-45, time.Second, true},
		{44, time.Second, false},
		{20, 100 * time.Millisecond, true}, // 0.2 px/ms
		{10, 100 * time.Millisecond, false},
		{0, 0, false},
	}
	for _, c := range cases {
		if got := swipeDismisses(c.amount, c.elapsed); got != c.want {
			t.Errorf("swipeDismisses(%v, %v) = %v, want %v", c.amount, c.elapsed, got, c.want)
		}
	}
}

func TestSwipeTheWrongWayIsDampened(t *testing.T) {
	if got := swipeAmount(40, true); got != 40 {
		t.Fatalf("allowed = %v, want the full pull", got)
	}
	if got := swipeAmount(-40, false); got != -40/3.5 {
		t.Fatalf("dampened = %v, want -40/(1.5+40/20)", got)
	}
}

// toasterRig lays a Toaster out alone in a window-sized frame, with input.
type toasterRig struct {
	t      *testing.T
	state  Toaster
	router input.Router
	now    time.Time
	size   image.Point
}

func newToasterRig(t *testing.T) *toasterRig {
	return &toasterRig{t: t, now: time.Now(), size: image.Pt(800, 600)}
}

func (r *toasterRig) frame() {
	ops := new(op.Ops)
	gtx := layout.Context{
		Ops: ops, Now: r.now, Source: r.router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(r.size),
	}
	testTheme().Toaster(&r.state, false).Layout(gtx)
	r.router.Frame(ops)
}

func (r *toasterRig) pointer(kind pointer.Kind, at f32.Point) {
	buttons := pointer.Buttons(0)
	if kind == pointer.Press || kind == pointer.Drag {
		buttons = pointer.ButtonPrimary
	}
	if kind == pointer.Drag {
		kind = pointer.Move // the router makes a move with a button down a drag
	}
	r.router.Queue(pointer.Event{Kind: kind, Source: pointer.Mouse, Position: at, Buttons: buttons})
}

// onFront is a point in the front toast's box, top-right corner.
func (r *toasterRig) onFront() f32.Point {
	return f32.Pt(float32(r.size.X-int(toastOffsetRight)-20), float32(toastOffsetTop)+10)
}

// settle lays out a frame once the enter animation is over, so the areas
// sit where the toasts came to rest.
func (r *toasterRig) settle() {
	r.frame()
	r.now = r.now.Add(500 * time.Millisecond)
	r.frame()
}

func (r *toasterRig) live(id string) bool {
	r.state.mu.Lock()
	defer r.state.mu.Unlock()
	return r.state.stack.live(id) != nil
}

func TestToasterHoverHoldsTheTimer(t *testing.T) {
	r := newToasterRig(t)
	r.state.Show(Toast{ID: "x", Title: "hello", Duration: 2 * time.Second})
	r.settle()
	r.pointer(pointer.Move, r.onFront())
	r.frame()
	r.now = r.now.Add(3 * time.Second)
	r.frame()
	if !r.live("x") {
		t.Fatal("expired under the pointer")
	}
	r.pointer(pointer.Move, f32.Pt(5, 500))
	r.frame()
	r.now = r.now.Add(1600 * time.Millisecond)
	r.frame()
	if r.live("x") {
		t.Fatal("still up past the 1.5s it had left once the pointer left")
	}
}

func TestToasterSwipeRightDismisses(t *testing.T) {
	r := newToasterRig(t)
	r.state.Show(Toast{ID: "x", Title: "hello", Duration: ToastForever})
	r.settle()
	at := r.onFront().Sub(f32.Pt(100, 0))
	r.pointer(pointer.Press, at)
	r.frame()
	r.now = r.now.Add(time.Second)
	r.pointer(pointer.Drag, at.Add(f32.Pt(60, 0)))
	r.frame()
	r.pointer(pointer.Release, at.Add(f32.Pt(60, 0)))
	r.frame()
	if r.live("x") {
		t.Fatal("a 60dp swipe right left it up")
	}
}

func TestToasterShortSlowSwipeSpringsBack(t *testing.T) {
	r := newToasterRig(t)
	r.state.Show(Toast{ID: "x", Title: "hello", Duration: ToastForever})
	r.settle()
	at := r.onFront().Sub(f32.Pt(100, 0))
	r.pointer(pointer.Press, at)
	r.frame()
	r.now = r.now.Add(time.Second)
	r.pointer(pointer.Drag, at.Add(f32.Pt(30, 0)))
	r.frame()
	r.pointer(pointer.Release, at.Add(f32.Pt(30, 0)))
	r.frame()
	if !r.live("x") {
		t.Fatal("a slow 30dp swipe dismissed it")
	}
}

func TestToasterActionRunsItsHandlerAndDismisses(t *testing.T) {
	r := newToasterRig(t)
	ran := false
	r.state.Show(Toast{ID: "x", Title: "Session closed", Duration: ToastForever, Action: &ToastButton{Label: "Undo", OnClick: func() { ran = true }}})
	r.settle()
	r.state.mu.Lock()
	e := r.state.stack.live("x")
	button := e.view.actionAt.Add(e.view.bodyAt)
	origin := e.view.place.Transform(f32.Point{})
	r.state.mu.Unlock()
	base := f32.Pt(float32(r.size.X-int(toastOffsetRight)-int(toastWidth)), float32(toastOffsetTop)).Add(origin)
	center := base.Add(f32.Pt(float32(button.Min.X+button.Dx()/2), float32(button.Min.Y+button.Dy()/2)))
	r.pointer(pointer.Press, center)
	r.pointer(pointer.Release, center)
	r.frame()
	if !ran || r.live("x") {
		t.Fatalf("ran %v, live %v: want the handler run and the toast dismissed", ran, r.live("x"))
	}
}
