package main

import (
	"image"
	"image/color"
	"math"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// A card's cover, the window at a glance in the theme's own colors, from the
// approved mockup's .mw (temas-catalogo, frame 1).
var (
	coverPad       = unit.Dp(5) // .mw padding 5px
	coverGap       = unit.Dp(3) // .mw gap 3px
	coverRadius    = ui.RadiusSM
	coverSidePadY  = unit.Dp(6) // .ms padding 6px 5px
	coverSidePadX  = unit.Dp(5)
	coverRowH      = unit.Dp(5) // .ms i height 5px
	coverRowGap    = ui.Space(1)
	coverRowRadius = unit.Dp(2)
	coverMainPadY  = unit.Dp(7) // .mm padding 7px 8px
	coverMainPadX  = ui.Space(2)
	coverLineGap   = ui.Space(1)
	coverBubblePad = layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2), Left: unit.Dp(6), Right: unit.Dp(6)}
	coverCodePad   = layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1), Left: unit.Dp(5), Right: unit.Dp(5)}
	coverCodeR     = unit.Dp(3)
	coverStripH    = unit.Dp(5) // .term b height 5px
	coverStripGap  = unit.Dp(2)
	coverStripR    = unit.Dp(1)
)

const (
	coverText     = unit.Sp(10) // .mm font 10px/1.3
	coverLeading  = unit.Sp(13)
	coverSideCut  = 0.28 // .ms width 28%
	coverRowMuted = 0.4  // the rows after the first, muted-foreground at 40%
)

// coverRowWidths are the sidebar's rows, as wide as these shares of it; the
// first is the selected one, in the accent.
var coverRowWidths = []float64{0.92, 0.70, 0.82, 0.60, 0.74}

// coverANSI are the terminal colors the cover's strip shows: red to cyan.
var coverANSI = []int{1, 2, 3, 4, 5, 6}

// inkSpan is a run of code in one color.
type inkSpan struct {
	text string
	ink  color.NRGBA
}

// themeCover is the cover, 16:10 across the card: a sidebar of rows, and a
// pane with a bubble in the accent, a code block in the theme's syntax, the
// Done line in its tones and a strip of the terminal's colors.
func themeCover(gtx C, th *ui.Theme, t themeLook) D {
	w := gtx.Constraints.Max.X
	size := image.Pt(w, w*catalogCoverH/catalogCoverW)
	gtx.Constraints = layout.Exact(size)
	p := t.palette
	paint.FillShape(gtx.Ops, p.Background, clip.Rect{Max: size}.Op())
	pad, gap := gtx.Dp(coverPad), gtx.Dp(coverGap)
	inner := image.Rect(pad, pad, size.X-pad, size.Y-pad)
	side := int(math.Round(float64(inner.Dx()) * coverSideCut))
	coverSidebar(gtx, p, image.Rect(inner.Min.X, inner.Min.Y, inner.Min.X+side, inner.Max.Y))
	coverMain(gtx, th, t, image.Rect(inner.Min.X+side+gap, inner.Min.Y, inner.Max.X, inner.Max.Y))
	return D{Size: size}
}

// coverSidebar is the sidebar's fill and its rows.
func coverSidebar(gtx C, p ui.Palette, box image.Rectangle) {
	paint.FillShape(gtx.Ops, p.Sidebar, clip.UniformRRect(box, gtx.Dp(coverRadius)).Op(gtx.Ops))
	padX, y := gtx.Dp(coverSidePadX), box.Min.Y+gtx.Dp(coverSidePadY)
	room := float64(box.Dx() - 2*padX)
	for i, share := range coverRowWidths {
		ink := ui.MixOKLab(p.Sidebar, p.MutedForeground, coverRowMuted)
		if i == 0 {
			ink = p.Accent
		}
		r := image.Rect(box.Min.X+padX, y, box.Min.X+padX+int(math.Round(room*share)), y+gtx.Dp(coverRowH))
		paint.FillShape(gtx.Ops, ink, clip.UniformRRect(r, gtx.Dp(coverRowRadius)).Op(gtx.Ops))
		y += gtx.Dp(coverRowH) + gtx.Dp(coverRowGap)
	}
}

// coverMain is the pane: the bubble, the code, the Done line, and the strip
// along its foot.
func coverMain(gtx C, th *ui.Theme, t themeLook, box image.Rectangle) {
	p := t.palette
	defer op.Offset(box.Min).Push(gtx.Ops).Pop()
	defer clip.Rect{Max: box.Size()}.Push(gtx.Ops).Pop()
	gtx.Constraints = layout.Exact(box.Size())
	layout.Inset{Top: coverMainPadY, Bottom: coverMainPadY, Left: coverMainPadX, Right: coverMainPadX}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				return layout.E.Layout(gtx, func(gtx C) D {
					return inkBox(gtx, p.Accent, coverRadius, coverBubblePad, th.Text(coverText, i18n.T("settings.themesPage.preview.askShort")).In(p.Foreground).Leading(coverLeading).Layout)
				})
			}),
			layout.Rigid(layout.Spacer{Height: coverLineGap}.Layout),
			layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return inkBox(gtx, p.Card, coverCodeR, coverCodePad, func(gtx C) D { return codeLines(gtx, th, coverCode(t), coverText, coverLeading) })
			}),
			layout.Rigid(layout.Spacer{Height: coverLineGap}.Layout),
			layout.Rigid(func(gtx C) D { return doneLine(gtx, th, p, coverText, coverLeading) }),
			layout.Flexed(1, func(gtx C) D {
				return layout.S.Layout(gtx, func(gtx C) D { return coverStrip(gtx, t) })
			}),
		)
	})
}

// coverCode is the cover's four lines of Go in the theme's syntax.
func coverCode(t themeLook) [][]inkSpan {
	fg, k := t.palette.Foreground, t.ink
	return [][]inkSpan{
		{{"for", k.keyword}, {" i := ", fg}, {"0", k.number}, {"; i < ", fg}, {"n", k.variable}, {" {", fg}},
		{{"  r := ", fg}, {"fetch", k.fn}, {"(", fg}, {`"api"`, k.str}, {")", fg}},
		{{"  ", fg}, {"// back off", k.comment}},
		{{"}", fg}},
	}
}

// doneLine is a turn's end as the chat writes it: Done, then the lines it
// added and removed in the theme's tones.
func doneLine(gtx C, th *ui.Theme, p ui.Palette, size, leading unit.Sp) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(th.Text(size, i18n.T("chat.done.done")).In(p.Foreground).Leading(leading).Layout),
		layout.Rigid(th.Text(size, " +18").In(p.TonePass).Leading(leading).Layout),
		layout.Rigid(th.Text(size, " −4").In(p.Destructive).Leading(leading).Layout),
	)
}

// coverStrip is six bars of the terminal's colors, red to cyan, across the
// pane.
func coverStrip(gtx C, t themeLook) D {
	w, h, gap := gtx.Constraints.Max.X, gtx.Dp(coverStripH), gtx.Dp(coverStripGap)
	n := len(coverANSI)
	bar := (w - (n-1)*gap) / n
	for i, c := range coverANSI {
		r := image.Rect(i*(bar+gap), 0, i*(bar+gap)+bar, h)
		paint.FillShape(gtx.Ops, ansiOf(t, c), clip.UniformRRect(r, gtx.Dp(coverStripR)).Op(gtx.Ops))
	}
	return D{Size: image.Pt(w, h)}
}

// ansiOf is the theme's terminal color i, its foreground where the theme
// leaves that one to libghostty.
func ansiOf(t themeLook, i int) color.NRGBA {
	if c := t.terminal.ANSI[i]; c.A != 0 {
		return c
	}
	return t.terminal.Foreground
}

// inkBox is w on a rounded fill of bg, padded.
func inkBox(gtx C, bg color.NRGBA, radius unit.Dp, pad layout.Inset, w layout.Widget) D {
	macro := op.Record(gtx.Ops)
	d := pad.Layout(gtx, w)
	call := macro.Stop()
	paint.FillShape(gtx.Ops, bg, clip.UniformRRect(image.Rectangle{Max: d.Size}, gtx.Dp(radius)).Op(gtx.Ops))
	call.Add(gtx.Ops)
	return d
}

// codeLines is lines of mono code, each a row of colored runs.
func codeLines(gtx C, th *ui.Theme, lines [][]inkSpan, size, leading unit.Sp) D {
	rows := make([]layout.FlexChild, len(lines))
	for i, line := range lines {
		rows[i] = layout.Rigid(func(gtx C) D {
			runs := make([]layout.FlexChild, len(line))
			for j, s := range line {
				runs[j] = layout.Rigid(th.Text(size, s.text).Mono().In(s.ink).Leading(leading).Layout)
			}
			gtx.Constraints.Min.Y = gtx.Sp(leading)
			return layout.Flex{}.Layout(gtx, runs...)
		})
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}
