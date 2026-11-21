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
)

// memoRig is a window holding one memoized subtree.
type memoRig struct {
	router  input.Router
	ops     op.Ops
	memo    Memo
	key     MemoKey
	renders int
	draw    func(gtx C) D
}

func (r *memoRig) frame() {
	r.ops.Reset()
	gtx := layout.Context{Ops: &r.ops, Now: time.Now(), Constraints: layout.Exact(image.Pt(100, 100)), Source: r.router.Source()}
	r.memo.Layout(gtx, r.key, func(gtx C) D {
		r.renders++
		return r.draw(gtx)
	})
	r.router.Frame(&r.ops)
}

func (r *memoRig) queue(events ...pointer.Event) {
	for i := range events {
		events[i].Source = pointer.Mouse
		events[i].Position = f32.Pt(10, 10)
	}
	for _, e := range events {
		r.router.Queue(e)
	}
}

// clickRig memoizes a full-size button, counting its clicks.
func clickRig() (*memoRig, *Clickable, *int) {
	r := &memoRig{}
	btn, clicks := new(Clickable), new(int)
	r.draw = func(gtx C) D {
		if btn.Clicked(gtx) {
			*clicks++
		}
		return btn.Layout(gtx, func(gtx C) D { return D{Size: gtx.Constraints.Max} })
	}
	return r, btn, clicks
}

func TestMemoReplaysWhileItsKeyHolds(t *testing.T) {
	r := &memoRig{draw: func(gtx C) D { return D{Size: gtx.Constraints.Max} }}
	r.frame()
	r.frame()
	r.frame()
	if r.renders != 1 {
		t.Fatalf("three frames of one key laid the subtree out %d times, want 1", r.renders)
	}
	r.key.Version++
	r.frame()
	if r.renders != 2 {
		t.Fatalf("a new version laid the subtree out %d times in all, want 2", r.renders)
	}
}

func TestMemoTakesAClickAfterReplayedFrames(t *testing.T) {
	r, _, clicks := clickRig()
	r.frame()
	r.frame()
	r.frame()
	r.queue(pointer.Event{Kind: pointer.Press, Buttons: pointer.ButtonPrimary}, pointer.Event{Kind: pointer.Release})
	r.frame()
	if *clicks != 1 {
		t.Fatalf("a click after replayed frames was taken %d times, want 1", *clicks)
	}
}

func TestMemoTakesAHoverAfterReplayedFrames(t *testing.T) {
	r, btn, _ := clickRig()
	r.frame()
	r.frame()
	r.frame()
	r.queue(pointer.Event{Kind: pointer.Move})
	r.frame()
	if !btn.Hovered() {
		t.Fatal("the pointer moved onto a replayed button, which does not see it hovered")
	}
}

// Content that arrives for the subtree, as a diff's highlighting does, comes
// with a new version and reaches the screen.
func TestMemoShowsContentThatArrivesWithANewVersion(t *testing.T) {
	shown, content := "", "plain"
	r := &memoRig{draw: func(gtx C) D {
		shown = content
		return D{Size: gtx.Constraints.Max}
	}}
	r.frame()
	r.frame()
	content = "highlighted"
	r.key.Version++
	r.frame()
	if shown != "highlighted" {
		t.Fatalf("the subtree shows %q after its content changed, want it highlighted", shown)
	}
}

// A blink or a tooltip's delay asks for a frame; replayed meanwhile, the
// subtree is laid out again once that frame comes.
func TestMemoLaysOutAgainWhenAFrameItAskedForFallsDue(t *testing.T) {
	at := time.Now().Add(-time.Millisecond)
	r := &memoRig{draw: func(gtx C) D {
		gtx.Execute(op.InvalidateCmd{At: at})
		return D{Size: gtx.Constraints.Max}
	}}
	r.frame()
	r.frame()
	if r.renders != 2 {
		t.Fatalf("a frame the subtree asked for came and it was laid out %d times, want 2", r.renders)
	}
	at = time.Now().Add(time.Hour)
	r.frame()
	r.frame()
	r.frame()
	if r.renders != 3 {
		t.Fatalf("with its next frame an hour away the subtree was laid out %d times, want 3", r.renders)
	}
}

func TestThemeGenerationFollowsTheLook(t *testing.T) {
	th := testTheme()
	if th.Generation() != th.On(th.Surface).Generation() {
		t.Fatal("the same theme on the same surface has another generation")
	}
	if th.Generation() == th.On(th.Primary).Generation() {
		t.Fatal("a theme on another surface has the same generation")
	}
	other := *th
	other.Background.R ^= 0xff
	if th.Generation() == other.Generation() {
		t.Fatal("another palette has the same generation")
	}
}
