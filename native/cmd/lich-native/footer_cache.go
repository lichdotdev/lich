package main

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

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
	bar := func(gtx C) D { return cacheBar(gtx, th, c) }
	return func(gtx C) D {
		gtx.Constraints.Min.X = min(gtx.Dp(planMinWide), gtx.Constraints.Max.X)
		return stack(gtx, 1, []layout.Widget{head, line, bar})
	}
}

// cacheBar is gaugeBar split three ways, each part as wide as its share.
func cacheBar(gtx C, th *ui.Theme, c api.SessionCache) D {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(ui.Space(1.5)))
	defer clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(ui.Space(0.75))).Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, th.Over(th.Border), clip.Rect{Max: size}.Op())
	total := c.Read + c.Write + c.Uncached
	parts := []struct {
		tokens int
		color  color.NRGBA
	}{{c.Read, th.MutedForeground}, {c.Write, th.ToneWait}, {c.Uncached, th.Foreground}}
	x, sum := 0, 0
	for _, p := range parts {
		sum += p.tokens
		// Each edge comes from the running sum, so rounding never leaves the
		// bar a pixel short of full.
		end := size.X * sum / total
		stackOp := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
		paint.FillShape(gtx.Ops, p.color, clip.Rect{Max: image.Pt(end-x, size.Y)}.Op())
		stackOp.Pop()
		x = end
	}
	return D{Size: size}
}
