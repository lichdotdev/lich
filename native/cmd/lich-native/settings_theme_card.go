package main

import (
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/i18n"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// ThemeMiniature's proportions: a 26% sidebar column, and three bars 8%
// tall with 15% between them inside 12% of padding, as wide as 60%, 85% and
// 45% of the column.
const (
	miniSidebar   = 0.26
	miniPad       = 0.12
	miniBarHeight = 0.08
	miniBarGap    = 0.15
)

var miniBarWidths = [3]float64{0.60, 0.85, 0.45}

// The card's rings: selected is ring-foreground/70, two pixels; a
// miniature's edge is ring-foreground/20, one.
const (
	cardSelectedRing = 0.7
	cardMiniRing     = 0.2
	cardActionsFill  = 0.9 // bg-popover/90
	spinTurn         = 1.0 // animate-spin: one turn a second
	importDash       = 4
	importDashGap    = 3
)

var (
	icRefresh     = icons.Lucide("refresh-cw")
	icTrash       = icons.Lucide("trash-2")
	icImportTheme = icons.Lucide("plus")
	cardActionBtn = ui.Space(6) // icon-xs
	cardActionPad = ui.Space(0.5)
)

// themeCard is one card's widget state.
type themeCard struct {
	click                widget.Clickable
	tip                  ui.Tooltip
	update, remove       widget.Clickable
	updateTip, removeTip ui.Tooltip
}

func themeCardOf(cards map[string]*themeCard, id string) *themeCard {
	if c, ok := cards[id]; ok {
		return c
	}
	c := new(themeCard)
	cards[id] = c
	return c
}

// themeCardSpec is what one card shows. Only a custom theme has actions,
// and only one from a repository can be updated.
type themeCardSpec struct {
	id, name, caption string
	selected          bool
	custom, fromRepo  bool
	updating          string // the theme an update is running for, "" for none
	preview           func(gtx C, th *ui.Theme, size image.Point) D
}

// cardLayout is ThemeCard: the preview in its frame, the name and caption
// under it, and the update and remove actions over its top-right corner
// while the pointer is on it.
func (p *themePicker) cardLayout(gtx C, th *ui.Theme, c *themeCard, s themeCardSpec) D {
	if c.update.Clicked(gtx) && s.updating == "" {
		p.updateTheme(s.id)
	}
	if c.remove.Clicked(gtx) {
		p.askRemove(s.id, s.name)
	}
	if c.click.Clicked(gtx) {
		p.look.choose(s.id)
	}
	tip := th.Tooltip(p.root, &c.tip, s.name+"  "+s.caption)
	tip.Side = ui.SideBottom
	d := tip.Layout(gtx, func(gtx C) D {
		return c.click.Layout(gtx, func(gtx C) D { return cardBody(gtx, th, s.name, s.caption, s.selected, s.preview) })
	})
	if s.custom && (c.click.Hovered() || c.update.Hovered() || c.remove.Hovered()) {
		p.cardActions(gtx, th, c, s, d.Size.X)
	}
	return d
}

// cardBody is the p-1 rounded-lg frame around the preview, ringed when
// selected, and the mt-1.5 line of name and caption.
func cardBody(gtx C, th *ui.Theme, name, caption string, selected bool, preview func(gtx C, th *ui.Theme, size image.Point) D) D {
	w, pad := gtx.Dp(themeCardWidth), gtx.Dp(themeCardPad)
	frame := image.Pt(w, gtx.Dp(themeCardPreview)+2*pad)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			if selected {
				insetRing(gtx, frame, gtx.Dp(ui.RadiusLG), gtx.Dp(2), th.Over(ui.Alpha(th.Foreground, cardSelectedRing)))
			}
			inner := frame.Sub(image.Pt(2*pad, 2*pad))
			t := op.Offset(image.Pt(pad, pad)).Push(gtx.Ops)
			cl := clip.UniformRRect(image.Rectangle{Max: inner}, gtx.Dp(ui.RadiusMD)).Push(gtx.Ops)
			preview(gtx, th, inner)
			cl.Pop()
			t.Pop()
			return D{Size: frame}
		}),
		ui.VGap(1.5),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints = layout.Exact(image.Pt(w, gtx.Constraints.Max.Y))
			gtx.Constraints.Min.Y = 0
			return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(th.Text(ui.TextXS, name).Weight(font.Medium).Layout),
				ui.Gap(1.5),
				layout.Flexed(1, th.Text(themeCaptionSize, caption).Muted().Layout),
			)
		}),
	)
}

// cardActions is the absolute right-1 top-1 box of icon buttons.
func (p *themePicker) cardActions(gtx C, th *ui.Theme, c *themeCard, s themeCardSpec, cardWidth int) {
	n := 1
	if s.fromRepo {
		n = 2
	}
	btn, pad, gap := gtx.Dp(cardActionBtn), gtx.Dp(cardActionPad), gtx.Dp(cardActionPad)
	size := image.Pt(2*pad+n*btn+(n-1)*gap, 2*pad+btn)
	inset := gtx.Dp(ui.Space(1))
	defer op.Offset(image.Pt(cardWidth-inset-size.X, inset)).Push(gtx.Ops).Pop()
	bg := th.Over(ui.Alpha(th.Popover, cardActionsFill))
	box := gtx
	box.Constraints = layout.Exact(size)
	ui.Fill(box, bg, ui.RadiusMD)
	on := th.On(bg)
	x := pad
	if s.fromRepo {
		var turn float32
		if s.updating == s.id {
			turn = spinAngle(gtx)
		}
		t := op.Offset(image.Pt(x, pad)).Push(gtx.Ops)
		tip := on.Tooltip(p.root, &c.updateTip, i18n.T("settings.themePicker.updateHint"))
		tip.Layout(gtx, func(gtx C) D { return cardIconButton(gtx, on, &c.update, icRefresh, turn, s.updating != "") })
		t.Pop()
		x += btn + gap
	}
	t := op.Offset(image.Pt(x, pad)).Push(gtx.Ops)
	tip := on.Tooltip(p.root, &c.removeTip, i18n.T("settings.themePicker.removeNamed", i18n.Args{"name": s.name}))
	tip.Layout(gtx, func(gtx C) D { return cardIconButton(gtx, on, &c.remove, icTrash, 0, false) })
	t.Pop()
}

// spinAngle is animate-spin's rotation now, and asks for the next frame.
func spinAngle(gtx C) float32 {
	gtx.Execute(op.InvalidateCmd{})
	secs := float64(gtx.Now.UnixMilli()%1000) / 1000 / spinTurn
	return float32(secs * 2 * math.Pi)
}

// cardIconButton is a ghost icon-xs button whose icon can be turned; a
// disabled one is drawn at half strength and takes no click.
func cardIconButton(gtx C, th *ui.Theme, click *widget.Clickable, ic *icons.Icon, turn float32, disabled bool) D {
	draw := func(gtx C) D {
		size := gtx.Dp(cardActionBtn)
		gtx.Constraints = layout.Exact(image.Pt(size, size))
		ink := th.Foreground
		if disabled {
			ink = th.Over(ui.Alpha(ink, 0.5))
		} else if click.Hovered() {
			ui.Fill(gtx, th.Over(ui.Alpha(th.Muted, 0.5)), ui.RadiusMD)
		}
		icon := gtx.Dp(ui.Space(3))
		off := (size - icon) / 2
		defer op.Offset(image.Pt(off, off)).Push(gtx.Ops).Pop()
		c := f32.Pt(float32(icon)/2, float32(icon)/2)
		defer op.Affine(f32.AffineId().Rotate(c, turn)).Push(gtx.Ops).Pop()
		ic.Layout(gtx, ui.Space(3), ink)
		return D{Size: image.Pt(size, size)}
	}
	if disabled {
		return draw(gtx)
	}
	return click.Layout(gtx, draw)
}

// importCardLayout is the strip's first card: a dashed frame with a + in
// the preview's place and "Import theme" where a theme's name goes, so it
// lines up with the cards and reads as the action that adds one.
func (p *themePicker) importCardLayout(gtx C, th *ui.Theme) D {
	c := &p.importCard
	if c.click.Clicked(gtx) {
		p.openImport()
	}
	return c.click.Layout(gtx, func(gtx C) D {
		hovered := c.click.Hovered()
		ink, edge := th.MutedForeground, th.Over(ui.Alpha(th.Foreground, cardMiniRing))
		if hovered {
			ink, edge = th.Foreground, th.Over(ui.Alpha(th.Foreground, 2*cardMiniRing))
		}
		preview := func(gtx C, th *ui.Theme, size image.Point) D {
			if hovered {
				box := gtx
				box.Constraints = layout.Exact(size)
				ui.Fill(box, th.Over(ui.Alpha(th.Accent, 0.5)), ui.RadiusMD)
			}
			dashedBorder(gtx, size, gtx.Dp(ui.RadiusMD), edge)
			icon := gtx.Dp(ui.Space(5))
			defer op.Offset(image.Pt((size.X-icon)/2, (size.Y-icon)/2)).Push(gtx.Ops).Pop()
			icImportTheme.Layout(gtx, ui.Space(5), ink)
			return D{Size: size}
		}
		return cardBody(gtx, th, i18n.T("settings.importThemeDialog.title"), "", false, preview)
	})
}

// themeMiniature is ThemeMiniature: the window at a glance in p's colors,
// the sidebar, the canvas and three bars for the text, the muted text and a
// filled row. ring draws its rounded-sm edge; a bare one is half of
// System's.
func themeMiniature(gtx C, th *ui.Theme, p ui.Palette, size image.Point, ring bool) D {
	if ring {
		defer clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(ui.RadiusSM)).Push(gtx.Ops).Pop()
	}
	paint.FillShape(gtx.Ops, p.Background, clip.Rect{Max: size}.Op())
	side := int(math.Round(float64(size.X) * miniSidebar))
	paint.FillShape(gtx.Ops, p.Sidebar, clip.Rect{Max: image.Pt(side, size.Y)}.Op())
	col := float64(size.X - side)
	pad := col * miniPad
	contentW, contentH := col-2*pad, float64(size.Y)-2*pad
	barH := math.Max(1, math.Round(contentH*miniBarHeight))
	y := pad
	for i, c := range []color.NRGBA{p.Foreground, p.MutedForeground, p.Accent} {
		x0 := float64(side) + pad
		r := image.Rect(int(math.Round(x0)), int(math.Round(y)), int(math.Round(x0+contentW*miniBarWidths[i])), int(math.Round(y+barH)))
		paint.FillShape(gtx.Ops, c, clip.UniformRRect(r, r.Dy()/2).Op(gtx.Ops))
		y += barH + contentH*miniBarGap
	}
	if ring {
		insetBorder(gtx, size, gtx.Dp(ui.RadiusSM), ui.Over(p.Background, ui.Alpha(th.Foreground, cardMiniRing)))
	}
	return D{Size: size}
}

// systemMiniature is SystemMiniature: the light and the dark window cut
// down the middle, one edge around both.
func systemMiniature(gtx C, th *ui.Theme, light, dark ui.Palette, size image.Point) D {
	defer clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(ui.RadiusSM)).Push(gtx.Ops).Pop()
	half := size.X / 2
	for i, p := range []ui.Palette{light, dark} {
		x := i * half
		w := half
		if i == 1 {
			w = size.X - half
		}
		t := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
		themeMiniature(gtx, th, p, image.Pt(w, size.Y), false)
		t.Pop()
		edge := clip.Rect{Min: image.Pt(x, 0), Max: image.Pt(x+w, size.Y)}.Push(gtx.Ops)
		insetBorder(gtx, size, gtx.Dp(ui.RadiusSM), ui.Over(p.Background, ui.Alpha(th.Foreground, cardMiniRing)))
		edge.Pop()
	}
	return D{Size: size}
}

// dashedBorder strokes a 1px dashed line just inside a rounded box: dashes
// along the straight edges, the corners' arcs whole.
func dashedBorder(gtx C, size image.Point, r int, c color.NRGBA) {
	w := gtx.Dp(1)
	dash, gap := gtx.Dp(importDash), gtx.Dp(importDashGap)
	for _, corner := range []image.Rectangle{
		{Max: image.Pt(r, r)}, {Min: image.Pt(size.X-r, 0), Max: image.Pt(size.X, r)},
		{Min: image.Pt(0, size.Y-r), Max: image.Pt(r, size.Y)}, {Min: size.Sub(image.Pt(r, r)), Max: size},
	} {
		cl := clip.Rect(corner).Push(gtx.Ops)
		insetRing(gtx, size, r, w, c)
		cl.Pop()
	}
	for x := r; x < size.X-r; x += dash + gap {
		end := min(x+dash, size.X-r)
		paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(x, 0), Max: image.Pt(end, w)}.Op())
		paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(x, size.Y-w), Max: image.Pt(end, size.Y)}.Op())
	}
	for y := r; y < size.Y-r; y += dash + gap {
		end := min(y+dash, size.Y-r)
		paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(0, y), Max: image.Pt(w, end)}.Op())
		paint.FillShape(gtx.Ops, c, clip.Rect{Min: image.Pt(size.X-w, y), Max: image.Pt(size.X, end)}.Op())
	}
}

// ghostTextButton is a ghost xs button in ink: h-6 px-2, the hover fill
// muted/50.
func ghostTextButton(gtx C, th *ui.Theme, click *widget.Clickable, label string, ink color.NRGBA) D {
	return click.Layout(gtx, func(gtx C) D {
		macro := op.Record(gtx.Ops)
		gtx.Constraints.Min = image.Pt(0, gtx.Dp(ui.Space(6)))
		d := layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
			return layout.Center.Layout(gtx, th.Text(ui.TextXS, label).In(ink).Weight(font.Medium).Layout)
		})
		call := macro.Stop()
		if click.Hovered() {
			box := gtx
			box.Constraints = layout.Exact(d.Size)
			ui.Fill(box, th.Over(ui.Alpha(th.Muted, 0.5)), ui.RadiusMD)
		}
		call.Add(gtx.Ops)
		return d
	})
}

// ruleAbove is border-t: a 1px line of border across gtx.
func ruleAbove(gtx C, th *ui.Theme) D {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(1))
	paint.FillShape(gtx.Ops, th.Over(th.Border), clip.Rect{Max: size}.Op())
	return D{Size: size}
}
