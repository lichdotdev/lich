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

// The web's FooterBar is min-h-11 with py-1.5 around h-8 buttons: 44px.
func TestFooterBarIsTheWebsFortyFourPixels(t *testing.T) {
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
