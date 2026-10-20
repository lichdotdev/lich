package main

import (
	"context"
	"image"
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

// The footer bar is 44px tall.
func TestFooterBarIsFortyFourPixels(t *testing.T) {
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Canvas}
	f := newFooter(context.Background(), nil, new(ui.Root), newFooterUsage(), newPlanQuota(quotaReader(nil), func() {}), func() {})
	f.dock = new(dock)
	for _, in := range []footerInput{{now: busySession().now}, busySession()} {
		gtx := layout.Context{
			Ops:         new(op.Ops),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Constraints{Max: image.Pt(2000, 1000)},
		}
		if got := f.layoutBar(gtx, th, in).Size.Y; got != 44 {
			t.Errorf("session %q: footer height %d, want 44", in.sessionID, got)
		}
	}
}
