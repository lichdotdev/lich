package main

import (
	"gioui.org/layout"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// cacheWarmPercent is the hit rate the cache line turns from amber to green
// at: below it, most of the turn's input was paid at the full rate.
const cacheWarmPercent = 50

// cacheHit is the rounded share of the last turn's input read back from the
// prompt cache. false when the provider filed no split, or an empty one.
func cacheHit(c *api.SessionCache) (int, bool) {
	if c == nil {
		return 0, false
	}
	total := c.Read + c.Write + c.Uncached
	if total <= 0 {
		return 0, false
	}
	return (c.Read*100 + total/2) / total, true
}

// cacheDetail is a tooltip body with a cache reading under it: head (the
// context tooltip's token line, or the cost tooltip's lines on a provider with
// no window), then the hit rate in its tone over the read / written / uncached
// bar.
func cacheDetail(th *ui.Theme, head layout.Widget, c api.SessionCache, hit int) layout.Widget {
	tone := th.ToneWait
	if hit >= cacheWarmPercent {
		tone = th.TonePass
	}
	line := para(th.Text(ui.TextXS, i18n.T("shell.footerSession.cacheHit", i18n.Args{"percent": hit})).In(tone)).Layout
	bar := func(gtx C) D {
		return th.SegmentBar(gtx,
			ui.Segment{Share: c.Read, Color: th.MutedForeground},
			ui.Segment{Share: c.Write, Color: th.ToneWait},
			ui.Segment{Share: c.Uncached, Color: th.Foreground})
	}
	return func(gtx C) D {
		gtx.Constraints.Min.X = min(gtx.Dp(planMinWide), gtx.Constraints.Max.X)
		return ui.Stack(1, head, line, bar)(gtx)
	}
}
