package ui

import (
	"image"
	"image/color"
	"testing"
	"time"

	"gioui.org/io/input"
)

func progressPixel(t *testing.T, p ProgressStyle, x int) color.NRGBA {
	t.Helper()
	ops := tabsFrame(new(input.Router), time.Unix(100, 0), func(gtx C) D {
		gtx.Constraints.Max.X = 100
		return p.Layout(gtx)
	})
	c := tabsRender(t, ops).RGBAAt(x, 3)
	return color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

func TestProgressFillsTheForegroundToItsPercent(t *testing.T) {
	th := testTheme()
	if d := layoutAt(th.Progress(50).Layout, 0, 100); d.Size != image.Pt(100, 6) {
		t.Fatalf("size %v, want the row by h-1.5", d.Size)
	}
	if got := progressPixel(t, th.Progress(50), 25); !near(got, th.Foreground) {
		t.Errorf("inside the 50%%: %v, want the foreground %v", got, th.Foreground)
	}
	if got := progressPixel(t, th.Progress(50), 75); !near(got, th.Muted) {
		t.Errorf("past the 50%%: %v, want the muted track %v", got, th.Muted)
	}
}

func TestIndeterminateProgressSlidesAndAsksForTheNextFrame(t *testing.T) {
	var r input.Router
	tabsFrame(&r, time.Unix(100, 0), func(gtx C) D { return testTheme().Progress(-1).Layout(gtx) })
	if _, ok := r.WakeupTime(); !ok {
		t.Error("an indeterminate bar must keep the window redrawing")
	}
}
