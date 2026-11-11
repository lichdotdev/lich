package extrender

import (
	"image"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/native/ui"
)

func init() {
	register(map[string]drawFunc{
		extension.IconLine.Name:         drawIconLine,
		extension.BoxIcon.Name:          drawBoxIcon,
		extension.IconLabel.Name:        drawIconLabel,
		extension.MonoGlyph.Name:        drawMonoGlyph,
		extension.MutedGlyph.Name:       drawMutedGlyph,
		extension.CenteredIcon.Name:     drawCenteredIcon,
		extension.TriggerChevron.Name:   drawTriggerChevron,
		extension.TinyChip.Name:         drawTinyChip,
		extension.HoverLink.Name:        drawHoverLink,
		extension.TrailingIconLink.Name: drawTrailingIconLink,
		extension.TwoLines.Name:         drawTwoLines,
		extension.SpinnerLine.Name:      drawSpinnerLine,
		extension.SpinnerBeside.Name:    drawSpinnerBeside,
		extension.Stat.Name:             drawStat,
		extension.CheckRow.Name:         drawCheckRow,
		extension.NavRow.Name:           drawNavRow,
		extension.PanelRow.Name:         drawPanelRow,
		extension.FillRow.Name:          drawFillRow,
		extension.OrRule.Name:           drawOrRule,
		extension.MenuHeader.Name:       drawMenuHeader,
		extension.MenuDivider.Name:      drawMenuSeparator,
		extension.RailHairline.Name:     drawRailHairline,
		extension.CaptionDivider.Name:   drawCaptionDivider,
		extension.GhostTextButton.Name:  drawGhostTextButton,
		extension.CardIconButton.Name:   drawCardIconButton,
		extension.GhostIconButton.Name:  drawGhostIconButton,
		extension.SquareFace.Name:       drawSquareFace,
		extension.LabelFace.Name:        drawLabelFace,
	})
}

// The measures lich-native hands these rows where the extension leaves them
// out.
var (
	navRowHeight = ui.Space(8)
	rowPadX      = ui.Space(1.5)
	panelRowEdge = ui.Space(2)
	cardIconSide = ui.Space(6)
	mutedGlyphDp = ui.Space(4)
	centeredIcon = ui.Space(4)
)

// The kit's glyphs draw through their icon, so a name no set has draws
// nothing rather than the row without its glyph.

func drawIconLine(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.IconColorProps](n)
	ic := r.icon(p.Icon)
	if ic == nil {
		return D{}
	}
	kids := r.children(th, n, id)
	rest := make([]layout.FlexChild, len(kids))
	for i, w := range kids {
		rest[i] = layout.Rigid(w)
	}
	return ui.IconLine(gtx, ic, r.color(th, p.Color, th.Foreground), rest...)
}

func drawBoxIcon(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.IconColorProps](n)
	ic := r.icon(p.Icon)
	if ic == nil {
		return D{}
	}
	return ui.BoxIcon(gtx, ic, r.color(th, p.Color, th.Foreground))
}

func drawIconLabel(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.IconLabelProps](n)
	ic := r.icon(p.Icon)
	if ic == nil {
		return D{}
	}
	return ui.IconLabel(ic, r.color(th, p.Glyph, th.Foreground), r.label(th, p.Label)).Layout(gtx)
}

// label is a label node's props as the kit's style, for a component that
// takes one whole.
func (r *Renderer) label(th *ui.Theme, p extension.LabelProps) ui.LabelStyle {
	l := th.Text(textSize(p.Size, ui.TextSM), p.Text).In(r.color(th, p.Color, th.Foreground)).Weight(weight(p.Weight))
	if p.Mono {
		l = l.Mono()
	}
	l.MaxLines = p.MaxLines
	if p.LineHeight > 0 {
		l.LineHeight = unit.Sp(p.LineHeight)
	}
	switch p.Align {
	case extension.TextAlignCenter:
		l.Align = text.Middle
	case extension.TextAlignEnd:
		l.Align = text.End
	}
	return l
}

func drawMonoGlyph(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.MetaGlyphProps](n)
	ic := r.icon(p.Icon)
	if ic == nil {
		return D{}
	}
	return th.MonoGlyph(ic, p.Text, r.color(th, p.Color, th.MutedForeground)).Layout(gtx)
}

func drawMutedGlyph(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.MutedGlyphProps](n)
	ic := r.icon(p.Icon)
	if ic == nil {
		return D{}
	}
	return th.MutedGlyph(ic, dpOr(p.Size, mutedGlyphDp)).Layout(gtx)
}

func drawCenteredIcon(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.GlyphProps](n)
	ic := r.icon(p.Name)
	if ic == nil {
		return D{}
	}
	return ui.CenteredIcon(gtx, ic, dpOr(p.Size, centeredIcon), r.color(th, p.Color, th.Foreground))
}

// dpOr is v in dp, or def when the extension left it out.
func dpOr(v float64, def unit.Dp) unit.Dp {
	if v <= 0 {
		return def
	}
	return unit.Dp(v)
}

func drawTriggerChevron(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	return th.TriggerChevron(gtx, props[extension.TriggerChevronProps](n).Open)
}

func drawTinyChip(r *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.TinyChipProps](n)
	return th.TinyChip(p.Word, r.color(th, p.Tone, th.MutedForeground)).Layout(gtx)
}

// pressed is the clickable under id, firing h for each press since the last
// frame.
func (r *Renderer) pressed(gtx C, id string, h extension.Handler) *widget.Clickable {
	click := stateOf[widget.Clickable](r, id)
	for click.Clicked(gtx) {
		r.fire(h, extension.EventData{})
	}
	return click
}

func drawHoverLink(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.LinkProps](n)
	return th.HoverLink(gtx, r.pressed(gtx, id, p.OnPress), p.Text)
}

func drawTrailingIconLink(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.TrailingIconLinkProps](n)
	ic := r.icon(p.Icon)
	if ic == nil {
		return D{}
	}
	return th.TrailingIconLink(gtx, r.pressed(gtx, id, p.OnPress), p.Label, ic)
}

func drawTwoLines(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.TwoLinesProps](n)
	return th.TwoLines(p.Top, p.Under).Layout(gtx)
}

func drawSpinnerLine(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	return th.SpinnerLine(gtx, props[extension.TextProps](n).Text)
}

func drawSpinnerBeside(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	return th.SpinnerBeside(gtx, r.stacked(th, n, id))
}

func drawStat(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.StatProps](n)
	return th.Stat(gtx, r.side(th, n, id, 0), p.Text, r.color(th, p.Color, th.Foreground))
}

// checkState is the row's box, showing the extension's Checked until the
// person flips it, then their choice until the extension sends another.
type checkState struct {
	box  widget.Bool
	prop bool
	set  bool
}

func drawCheckRow(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.CheckRowProps](n)
	s := stateOf[checkState](r, id)
	if !s.set || p.Checked != s.prop {
		s.set, s.prop, s.box.Value = true, p.Checked, p.Checked
	}
	before := s.box.Value
	d := th.CheckRow(gtx, &s.box, p.Label, p.Note)
	if s.box.Value != before {
		r.fire(p.OnChange, extension.EventData{Checked: s.box.Value})
	}
	return d
}

// rowContent is n's children drawn in the theme the row hands them, lit
// while the row is.
func (r *Renderer) rowContent(n extension.Node, id string) func(C, *ui.Theme) D {
	return func(gtx C, th *ui.Theme) D { return r.stacked(th, n, id)(gtx) }
}

func drawNavRow(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.NavRowProps](n)
	click := r.pressed(gtx, id, p.OnPress)
	return th.NavRow(gtx, click, p.On, dpOr(p.Height, navRowHeight), dpOr(p.PadX, rowPadX), r.rowContent(n, id))
}

func drawPanelRow(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.PanelRowProps](n)
	click := r.pressed(gtx, id, p.OnPress)
	return th.PanelRow(gtx, click, p.Live, dpOr(p.Edge, panelRowEdge), dpOr(p.PadX, rowPadX), r.rowContent(n, id))
}

func drawFillRow(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.FillRowProps](n)
	size := image.Pt(gtx.Dp(unit.Dp(p.Width)), gtx.Dp(unit.Dp(p.Height)))
	th.FillRow(gtx, size, float32(p.Alpha))
	return D{Size: size}
}

func drawOrRule(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	return th.OrRule(gtx, props[extension.WordProps](n).Word)
}

func drawMenuHeader(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.MenuHeaderProps](n)
	return th.MenuHeader(gtx, p.Title, p.Count)
}

func drawMenuSeparator(_ *Renderer, gtx C, th *ui.Theme, _ extension.Node, _ string) D {
	return th.MenuSeparator(gtx)
}

func drawRailHairline(_ *Renderer, gtx C, th *ui.Theme, _ extension.Node, _ string) D {
	return th.RailHairline(gtx)
}

func drawCaptionDivider(_ *Renderer, gtx C, th *ui.Theme, n extension.Node, _ string) D {
	p := props[extension.CaptionDividerProps](n)
	return th.CaptionDivider(gtx, unit.Dp(p.Inset), unit.Dp(p.Height))
}

func drawGhostTextButton(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.GhostTextButtonProps](n)
	return th.GhostTextButton(gtx, r.pressed(gtx, id, p.OnPress), p.Label, r.color(th, p.Color, th.Foreground))
}

func drawCardIconButton(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.CardIconButtonProps](n)
	ic := r.icon(p.Icon)
	if ic == nil {
		return D{}
	}
	side, turn := dpOr(p.Size, cardIconSide), float32(p.Turn)
	if p.OnPress == "" {
		return th.CardIconDisabled(gtx, ic, side, turn)
	}
	return th.CardIconButton(gtx, r.pressed(gtx, id, p.OnPress), ic, side, turn)
}

type ghostIconState struct {
	click widget.Clickable
	tip   ui.Tooltip
}

func drawGhostIconButton(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.GhostIconButtonProps](n)
	ic := r.icon(p.Icon)
	if ic == nil {
		return D{}
	}
	s := stateOf[ghostIconState](r, id)
	for s.click.Clicked(gtx) {
		if !p.Disabled {
			r.fire(p.OnPress, extension.EventData{})
		}
	}
	return th.GhostIconButton(gtx, r.root, &s.click, &s.tip, ic, p.Hint, p.Disabled)
}

func drawSquareFace(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.FaceProps](n)
	return ui.SquareFace(gtx, r.color(th, p.Background, th.Surface), r.side(th, n, id, 0))
}

func drawLabelFace(r *Renderer, gtx C, th *ui.Theme, n extension.Node, id string) D {
	p := props[extension.LabelFaceProps](n)
	return th.LabelFace(gtx, r.color(th, p.Background, th.Surface), r.side(th, n, id, 0), p.Label)
}
