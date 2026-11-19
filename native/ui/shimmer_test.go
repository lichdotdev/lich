package ui

import (
	"image"
	"testing"
	"time"

	"gioui.org/io/input"
)

// A text asks for a frame only while it shimmers: a plain one leaves the
// window at rest.
func TestAShimmeringTextAsksForFramesAndAPlainOneDoesNot(t *testing.T) {
	th := testTheme()
	if _, ok := loopWake(func(gtx C) D { return th.Text(TextSM, "Thinking").Muted().Layout(gtx) }); ok {
		t.Error("a plain text asked for a frame")
	}
	at, ok := loopWake(func(gtx C) D { return th.Text(TextSM, "Thinking").Muted().Shimmer().Layout(gtx) })
	if want := time.Unix(100, 0).Add(LoopFrame); !ok || !at.Equal(want) {
		t.Errorf("a shimmering text asked for its next frame at %v (asked %v), want %v", at, ok, want)
	}
}

// The highlight enters left of the text, crosses it and leaves by the right,
// and is back where it started a lap on.
func TestTheShimmerSweepsAndLoops(t *testing.T) {
	start := time.Unix(160, 0) // a whole number of laps after the epoch
	first, band := shimmerAt(start, 100)
	if first > -band/2 {
		t.Errorf("the band starts at %d, want it off the text's left edge", first)
	}
	if mid, _ := shimmerAt(start.Add(ShimmerLap/2), 100); mid < 40 || mid > 60 {
		t.Errorf("half a lap on the band's centre is at %d, want the text's middle", mid)
	}
	if late, _ := shimmerAt(start.Add(ShimmerLap*99/100), 100); late < 100+band/2 {
		t.Errorf("at the lap's end the band is at %d, want it past the right edge", late)
	}
	if again, _ := shimmerAt(start.Add(ShimmerLap), 100); again != first {
		t.Errorf("a lap later the band is at %d, want %d", again, first)
	}
}

// brightest is the brightest red channel among the lit pixels of img in the
// columns from..to.
func brightest(img *image.RGBA, from, to int) uint8 {
	var best uint8
	for x := from; x < to; x++ {
		for y := 0; y < img.Bounds().Dy(); y++ {
			if c := img.RGBAAt(x, y); c.R > best {
				best = c.R
			}
		}
	}
	return best
}

// The light band is where the sweep says it is: at the middle of a lap it
// lights the middle of the text and leaves its two ends at the base color.
func TestTheShimmerLightsTheTextWhereTheBandIs(t *testing.T) {
	th := testTheme()
	now := time.Unix(160, 0).Add(ShimmerLap / 2)
	var width int
	ops := tabsFrame(new(input.Router), now, func(gtx C) D {
		d := th.Text(TextBase, "Thinking about it").In(th.MutedForeground).Shimmer().Layout(gtx)
		width = d.Size.X
		return d
	})
	img := tabsRender(t, ops)
	if width < 60 {
		t.Fatalf("text %dpx wide", width)
	}
	end, mid := brightest(img, 0, 6), brightest(img, width/2-4, width/2+4)
	if mid <= end+30 {
		t.Errorf("brightest red %d in the middle and %d at the left end: the band is not on the middle", mid, end)
	}
}

// After the band has crossed a stretch of text that stretch is back at the
// base color, not left lit behind it.
func TestTheShimmerLeavesTheTextBehindItAtItsBase(t *testing.T) {
	th := testTheme()
	now := time.Unix(160, 0).Add(ShimmerLap * 65 / 100)
	var width int
	ops := tabsFrame(new(input.Router), now, func(gtx C) D {
		d := th.Text(TextSM, "1 tool call").In(th.MutedForeground).Shimmer().Layout(gtx)
		width = d.Size.X
		return d
	})
	img := tabsRender(t, ops)
	behind, band := brightest(img, 0, width/3), brightest(img, width*5/8, width*7/8)
	if band < behind+40 {
		t.Errorf("brightest red %d behind the band and %d in it: the band left the text lit", behind, band)
	}
}
