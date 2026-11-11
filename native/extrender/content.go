package extrender

import (
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/ui"
)

func init() {
	register(map[string]drawFunc{
		extension.Notice.Name:        drawNotice,
		extension.Footnote.Name:      drawFootnote,
		extension.BodyLine.Name:      drawBodyLine,
		extension.PanelNotice.Name:   drawPanelNotice,
		extension.IconAction.Name:    drawIconAction,
		extension.CornerButton.Name:  drawCornerButton,
		extension.BarIconButton.Name: drawBarIconButton,
		extension.DiffStat.Name:      drawDiffStat,
		extension.MetaGlyph.Name:     drawMetaGlyph,
		extension.Pill.Name:          drawPill,
		extension.KeyCap.Name:        drawKeyCap,
		extension.PopoverRow.Name:    drawPopoverRow,
		extension.SelectTrigger.Name: drawSelectTrigger,
		extension.VerticalRule.Name:  drawVerticalRule,
	})
}

func drawNotice(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	return th.Notice(props[extension.TextProps](n).Text).Layout(gtx)
}

func drawFootnote(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	return th.Footnote(props[extension.TextProps](n).Text).Layout(gtx)
}

func drawBodyLine(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	return th.BodyLine(props[extension.TextProps](n).Text).Layout(gtx)
}

func drawPanelNotice(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.PanelNoticeProps](n)
	return th.PanelNotice(p.Text, unit.Dp(p.PadX)).Layout(gtx)
}

func drawIconAction(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.IconActionProps](n)
	s := stateOf[ui.IconActionState](r, id)
	if s.Click.Clicked(gtx) {
		r.fire(p.OnPress, extension.EventData{})
	}
	return th.IconAction(r.layer(), s, r.iconOr(p.Icon), p.Label, p.Pressed).Layout(gtx)
}

// drawCornerButton draws disabled with no OnPress, as the contract says: the
// kit's corner button has no disabled look, so it is only made inert.
func drawCornerButton(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.IconToggleProps](n)
	click := stateOf[widget.Clickable](r, id)
	if p.OnPress == "" {
		gtx = gtx.Disabled()
	}
	if click.Clicked(gtx) {
		r.fire(p.OnPress, extension.EventData{})
	}
	return th.CornerButton(click, r.iconOr(p.Icon), p.On).Layout(gtx)
}

// drawBarIconButton hands the kit no clickable with no OnPress, which is how
// it draws a disabled one.
func drawBarIconButton(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.IconToggleProps](n)
	var click *widget.Clickable
	if p.OnPress != "" {
		click = stateOf[widget.Clickable](r, id)
		if click.Clicked(gtx) {
			r.fire(p.OnPress, extension.EventData{})
		}
	}
	return th.BarIconButton(click, r.iconOr(p.Icon), p.On).Layout(gtx)
}

func drawDiffStat(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.DiffStatProps](n)
	return th.DiffStat(p.Added, p.Deleted).Layout(gtx)
}

func drawMetaGlyph(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.MetaGlyphProps](n)
	return th.MetaGlyph(r.iconOr(p.Icon), p.Text, r.color(th, p.Color, th.Foreground)).Layout(gtx)
}

// drawPill draws its children side by side inside the pill.
func drawPill(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.PillProps](n)
	return th.Pill(ui.Row(0, r.children(th, n, id)...), unit.Dp(p.PadLeft), unit.Dp(p.PadRight)).Layout(gtx)
}

func drawKeyCap(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	return th.KeyCap(props[extension.KeyCapProps](n).Keys).Layout(gtx)
}

func drawPopoverRow(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.PopoverRowProps](n)
	click := stateOf[widget.Clickable](r, id)
	if click.Clicked(gtx) {
		r.fire(p.OnPress, extension.EventData{})
	}
	return th.PopoverRow(click, func(gtx C, on *ui.Theme) D { return r.body(on, n, id)(gtx) }).Layout(gtx)
}

// drawSelectTrigger is the row's whole width when the extension names none:
// the kit's trigger has no width of its own.
func drawSelectTrigger(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.SelectTriggerProps](n)
	width := unit.Dp(p.Width)
	if width <= 0 {
		width = gtx.Metric.PxToDp(gtx.Constraints.Max.X)
	}
	t := th.SelectTrigger(p.Value, width)
	if p.Lead != nil {
		t.Lead = r.extra(th, *p.Lead, id+"/lead")
	}
	return t.Layout(gtx)
}

func drawVerticalRule(_ *Renderer, gtx C, th *ui.Theme, _ extension.Node, _ string) D {
	return th.VerticalRule(gtx)
}
