package main

import (
	"image"
	"image/color"
	"math"

	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
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

// A miniature's edge is ring-foreground/20, one pixel.
const cardMiniRing = 0.2

var (
	icRefresh     = icons.Lucide("refresh-cw")
	icTrash       = icons.Lucide("trash-2")
	icImportTheme = icons.Lucide("plus")
)

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
		ui.InsetBorder(gtx, size, gtx.Dp(ui.RadiusSM), ui.Over(p.Background, ui.Alpha(th.Foreground, cardMiniRing)))
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
		ui.InsetBorder(gtx, size, gtx.Dp(ui.RadiusSM), ui.Over(p.Background, ui.Alpha(th.Foreground, cardMiniRing)))
		edge.Pop()
	}
	return D{Size: size}
}
