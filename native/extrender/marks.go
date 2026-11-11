package extrender

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/ui"
)

func init() {
	register(map[string]drawFunc{
		extension.StatusDot.Name:        colorDot(func(gtx C, _ *ui.Theme, c color.NRGBA) D { return ui.StatusDot(gtx, c) }),
		extension.HollowDot.Name:        colorDot(func(gtx C, _ *ui.Theme, c color.NRGBA) D { return ui.HollowDot(gtx, c) }),
		extension.BeatingDot.Name:       colorDot(func(gtx C, th *ui.Theme, c color.NRGBA) D { return th.BeatingDot(gtx, c) }),
		extension.MarkDot.Name:          colorDot(func(gtx C, _ *ui.Theme, c color.NRGBA) D { return ui.MarkDot(gtx, c) }),
		extension.Swatch.Name:           colorDot(func(gtx C, _ *ui.Theme, c color.NRGBA) D { return ui.Swatch(gtx, c) }),
		extension.CornerDot.Name:        colorDot(func(gtx C, th *ui.Theme, c color.NRGBA) D { return th.CornerDot(gtx, c) }),
		extension.PulsingCornerDot.Name: colorDot(func(gtx C, th *ui.Theme, c color.NRGBA) D { return th.PulsingCornerDot(gtx, c) }),
		extension.Dot.Name:              drawDot,
		extension.DotAt.Name:            drawDotAt,
		extension.PulseRing.Name:        drawPulseRing,
		extension.NotifyDot.Name:        drawNotifyDot,

		extension.GaugeBar.Name:   drawGaugeBar,
		extension.SegmentBar.Name: drawSegmentBar,
		extension.DonutGauge.Name: drawDonutGauge,
		extension.GaugeRow.Name:   drawGaugeRow,

		extension.CountBadge.Name:   drawCountBadge,
		extension.OutlineBadge.Name: drawOutlineBadge,
		extension.AbbrBadge.Name:    drawAbbrBadge,
		extension.Tag.Name:          drawTag,
		extension.KeyHint.Name:      drawKeyHint,
		extension.DropHint.Name:     drawDropHint,

		extension.Tray.Name:         drawTray,
		extension.DropTray.Name:     drawDropTray,
		extension.FillIf.Name:       drawFillIf,
		extension.Checker.Name:      drawChecker,
		extension.Fillet.Name:       drawFillet,
		extension.FillRRect.Name:    drawFillRRect,
		extension.InsetBorder.Name:  drawInsetBorder,
		extension.InsetRing.Name:    drawInsetRing,
		extension.StrokeArc.Name:    drawStrokeArc,
		extension.DashedBorder.Name: drawDashedBorder,

		extension.DashedRing.Name:   drawRing(ui.DashedRing),
		extension.MarchingRing.Name: drawRing(ui.MarchingRing),
		extension.DashedCircle.Name: drawDashedCircle,
	})
}

// colorDot draws a mark that takes only its color, the foreground when the
// extension names none.
func colorDot(draw func(C, *ui.Theme, color.NRGBA) D) drawFunc {
	return func(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
		return draw(gtx, th, r.color(th, props[extension.ColorProps](n).Color, th.Foreground))
	}
}

// point is x, y in dp as pixels, unrounded so a circle centers on it.
func point(gtx C, x, y float64) f32.Point {
	return f32.Pt(px(gtx, x), px(gtx, y))
}

func px(gtx C, dp float64) float32 { return float32(dp) * gtx.Metric.PxPerDp }

// sizeOf is a width and height in dp as pixels.
func sizeOf(gtx C, w, h float64) image.Point {
	return image.Pt(gtx.Dp(unit.Dp(w)), gtx.Dp(unit.Dp(h)))
}

// overlay is what a shape drawn at a point of its parent takes: the room it
// was made to fill, and none of its own.
func overlay(gtx C) D { return D{Size: gtx.Constraints.Min} }

func drawDot(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.DotProps](n)
	return ui.Dot(gtx, r.color(th, p.Color, th.Foreground), unit.Dp(p.Size))
}

func drawDotAt(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.DotAtProps](n)
	ui.DotAt(gtx, point(gtx, p.X, p.Y), px(gtx, p.Radius), r.color(th, p.Color, th.Foreground))
	return overlay(gtx)
}

func drawPulseRing(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.PulseRingProps](n)
	th.PulseRing(gtx, point(gtx, p.X, p.Y), px(gtx, p.Radius), px(gtx, p.Reach), r.color(th, p.Color, th.Foreground))
	return overlay(gtx)
}

func drawNotifyDot(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.SizeProps](n)
	th.NotifyDot(gtx, sizeOf(gtx, p.Width, p.Height))
	return overlay(gtx)
}

func drawGaugeBar(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.GaugeBarProps](n)
	return th.GaugeBar(gtx, p.Percent, r.color(th, p.Tone, th.Foreground))
}

func drawSegmentBar(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.SegmentBarProps](n)
	parts := make([]ui.Segment, len(p.Segments))
	for i, s := range p.Segments {
		parts[i] = ui.Segment{Share: s.Share, Color: r.color(th, s.Color, th.Foreground)}
	}
	return th.SegmentBar(gtx, parts...)
}

func drawDonutGauge(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.DonutGaugeProps](n)
	return th.DonutGauge(gtx, unit.Dp(p.Size), p.Percent, r.color(th, p.Color, th.Foreground))
}

func drawGaugeRow(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.GaugeRowProps](n)
	return ui.GaugeRowStyle{
		LabelW: unit.Dp(p.LabelWidth), ReadingW: unit.Dp(p.ReadingWidth), LeftW: unit.Dp(p.LeftWidth),
		Label: r.slot(th, p.Label, id+"/label"), Bar: r.slot(th, p.Bar, id+"/bar"),
		Reading: r.slot(th, p.Reading, id+"/reading"), Left: r.slot(th, p.Left, id+"/left"),
	}.Layout(gtx)
}

func drawCountBadge(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	return th.CountBadge(gtx, props[extension.CountBadgeProps](n).Count)
}

func drawOutlineBadge(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	return th.OutlineBadge(props[extension.TextProps](n).Text).Layout(gtx)
}

func drawAbbrBadge(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.AbbrBadgeProps](n)
	return th.AbbrBadge(p.Abbr, r.color(th, p.Background, th.Muted), r.color(th, p.Foreground, th.Foreground)).Layout(gtx)
}

func drawTag(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.TagProps](n)
	return th.Tag(p.Text, r.color(th, p.Color, th.MutedForeground)).Layout(gtx)
}

func drawKeyHint(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.KeyHintProps](n)
	return th.KeyHint(p.Keys, p.Label).Layout(gtx)
}

func drawDropHint(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.DropHintProps](n)
	ic := r.icon(p.Icon)
	if ic == nil {
		return D{}
	}
	return th.DropHint(ic, p.Before, p.Label, p.After, p.Detail).Layout(gtx)
}

func drawTray(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	fill := r.color(th, props[extension.TrayProps](n).Fill, th.Accent)
	return ui.Tray(gtx, fill, r.stacked(th.On(fill), n, id))
}

func drawDropTray(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.DropTrayProps](n)
	fill := r.color(th, p.Fill, th.Accent)
	return ui.DropTray(gtx, fill, r.color(th, p.Edge, th.MutedForeground), r.stacked(th.On(fill), n, id))
}

func drawFillIf(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.FillIfProps](n)
	return ui.FillIf(gtx, r.color(th, p.Color, color.NRGBA{}), unit.Dp(p.Radius))
}

func drawChecker(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.CheckerProps](n)
	size := sizeOf(gtx, p.Width, p.Height)
	th.Checker(gtx, size, unit.Dp(p.Cell))
	return D{Size: size}
}

func drawFillet(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.FilletProps](n)
	at := sizeOf(gtx, p.X, p.Y)
	ui.Fillet(gtx, at, gtx.Dp(unit.Dp(p.Radius)), p.Right, r.color(th, p.Color, th.Foreground))
	return overlay(gtx)
}

func drawFillRRect(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.FillRRectProps](n)
	ui.FillRRect(gtx, rectOf(gtx, p.X, p.Y, p.Width, p.Height), unit.Dp(p.Radius), r.color(th, p.Color, th.Foreground))
	return overlay(gtx)
}

// rectOf is a rectangle at x, y of width w and height h, all in dp.
func rectOf(gtx C, x, y, w, h float64) image.Rectangle {
	at := sizeOf(gtx, x, y)
	return image.Rectangle{Min: at, Max: at.Add(sizeOf(gtx, w, h))}
}

func drawInsetBorder(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.InsetBorderProps](n)
	size := sizeOf(gtx, p.Width, p.Height)
	ui.InsetBorder(gtx, size, gtx.Dp(unit.Dp(p.Radius)), r.color(th, p.Color, th.Border))
	return D{Size: size}
}

func drawInsetRing(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.InsetRingProps](n)
	size := sizeOf(gtx, p.Width, p.Height)
	ui.InsetRing(gtx, size, gtx.Dp(unit.Dp(p.Radius)), gtx.Dp(unit.Dp(p.StrokeWidth)), r.color(th, p.Color, th.Foreground))
	return D{Size: size}
}

func drawStrokeArc(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.StrokeArcProps](n)
	ui.StrokeArc(gtx, point(gtx, p.X, p.Y), px(gtx, p.Radius), px(gtx, p.Width), float32(p.Start), float32(p.Sweep), r.color(th, p.Color, th.Foreground))
	return overlay(gtx)
}

func drawDashedBorder(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.DashedBorderProps](n)
	size := sizeOf(gtx, p.Width, p.Height)
	ui.DashedBorder(gtx, size, gtx.Dp(unit.Dp(p.Radius)), r.color(th, p.Color, th.MutedForeground), unit.Dp(p.DashLength), unit.Dp(p.GapLength))
	return D{Size: size}
}

// drawRing draws a dashed ring, its halo none unless the extension names one.
func drawRing(ring func(C, image.Rectangle, unit.Dp, unit.Dp, color.NRGBA, color.NRGBA)) drawFunc {
	return func(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
		p := props[extension.RingProps](n)
		ring(gtx, rectOf(gtx, p.X, p.Y, p.Width, p.Height), unit.Dp(p.Radius), unit.Dp(p.StrokeWidth),
			r.color(th, p.Color, th.Foreground), r.color(th, p.Halo, color.NRGBA{}))
		return overlay(gtx)
	}
}

func drawDashedCircle(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.DashedCircleProps](n)
	ui.DashedCircle(gtx, point(gtx, p.X, p.Y), unit.Dp(p.Outer), unit.Dp(p.Stroke), unit.Dp(p.Dash), r.color(th, p.Color, th.MutedForeground))
	return overlay(gtx)
}
