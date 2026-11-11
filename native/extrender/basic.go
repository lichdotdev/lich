package extrender

import (
	"image"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/ui"
)

func init() {
	register(map[string]drawFunc{
		"box":    drawBox,
		"label":  drawLabel,
		"button": drawButton,
	})
}

func drawBox(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.BoxProps](n)
	if p.Width > 0 {
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(unit.Dp(p.Width)), gtx.Dp(unit.Dp(p.Width))
	}
	if p.Height > 0 {
		gtx.Constraints.Min.Y, gtx.Constraints.Max.Y = gtx.Dp(unit.Dp(p.Height)), gtx.Dp(unit.Dp(p.Height))
	}
	inner := th
	if p.Background != "" {
		inner = th.On(r.color(th, p.Background, th.Background))
	}
	body := func(gtx C) D { return boxInset(p).Layout(gtx, boxFlex(r, inner, n, id, p)) }
	if p.Background == "" {
		return body(gtx)
	}
	bg := r.color(th, p.Background, th.Background)
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, th.Over(bg), unit.Dp(p.Radius)) },
		body)
}

func boxInset(p extension.BoxProps) layout.Inset {
	x, y := p.Padding, p.Padding
	if p.PaddingX > 0 {
		x = p.PaddingX
	}
	if p.PaddingY > 0 {
		y = p.PaddingY
	}
	return layout.Inset{Left: unit.Dp(x), Right: unit.Dp(x), Top: unit.Dp(y), Bottom: unit.Dp(y)}
}

// boxFlex lays the children out along the box's axis; a child box with Grow
// takes that share of the room left.
func boxFlex(r *Renderer, th *ui.Theme, n extension.Node, id string, p extension.BoxProps) layout.Widget {
	axis := layout.Vertical
	if p.Direction == extension.DirectionRow {
		axis = layout.Horizontal
	}
	kids := make([]layout.FlexChild, 0, 2*len(n.Children))
	for i, c := range n.Children {
		if i > 0 && p.Gap > 0 {
			kids = append(kids, layout.Rigid(gapOf(axis, p.Gap)))
		}
		w := r.child(th, c, id, i)
		if grow := props[extension.BoxProps](c).Grow; c.Type == extension.Box.Name && grow > 0 {
			kids = append(kids, layout.Flexed(float32(grow), w))
			continue
		}
		kids = append(kids, layout.Rigid(stretched(axis, p.Align, w)))
	}
	flex := layout.Flex{Axis: axis, Alignment: alignment(p.Align), Spacing: spacing(p.Justify)}
	return func(gtx C) D {
		if axis == layout.Vertical || p.Justify != "" {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
		}
		return flex.Layout(gtx, kids...)
	}
}

func gapOf(axis layout.Axis, dp float64) layout.Widget {
	return func(gtx C) D {
		if axis == layout.Horizontal {
			return D{Size: image.Pt(gtx.Dp(unit.Dp(dp)), 0)}
		}
		return D{Size: image.Pt(0, gtx.Dp(unit.Dp(dp)))}
	}
}

// stretched makes a child fill the cross axis for AlignStretch.
func stretched(axis layout.Axis, a extension.Align, w layout.Widget) layout.Widget {
	if a != extension.AlignStretch {
		return w
	}
	return func(gtx C) D {
		if axis == layout.Vertical {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
		} else {
			gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
		}
		return w(gtx)
	}
}

func alignment(a extension.Align) layout.Alignment {
	switch a {
	case extension.AlignCenter:
		return layout.Middle
	case extension.AlignEnd:
		return layout.End
	}
	return layout.Start
}

func spacing(j extension.Justify) layout.Spacing {
	switch j {
	case extension.JustifyCenter:
		return layout.SpaceSides
	case extension.JustifyEnd:
		return layout.SpaceStart
	case extension.JustifyBetween:
		return layout.SpaceBetween
	}
	return layout.SpaceEnd
}

func drawLabel(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	return r.label(th, props[extension.LabelProps](n)).Layout(gtx)
}

func drawButton(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.ButtonProps](n)
	click := stateOf[widget.Clickable](r, id)
	if click.Clicked(gtx) && !p.Disabled {
		r.fire(p.OnPress, extension.EventData{})
	}
	b := th.Button(click, p.Text)
	b.Icon, b.Variant, b.Size, b.Disabled, b.Lit = r.icon(p.Icon), variant(p.Variant), size(p.Size), p.Disabled, p.Lit
	return b.Layout(gtx)
}
