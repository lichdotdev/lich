package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

var (
	// balloonItemHeight is a footer item's h-8.
	balloonItemHeight = ui.Space(8)
	// balloonPad keeps an item's RadiusMD hover concentric with the
	// balloon's RadiusLG.
	balloonPad = ui.Space(0.5)
	balloonGap = ui.Space(1)
	// balloonInset is the island's edge, which the balloons keep from its
	// sides and its foot.
	balloonInset = ui.Space(2)
	// balloonBand is what a terminal under the balloons gives up at its
	// foot: the balloon, the edge under it and as much over it, in whole
	// rows once the grid rounds it down.
	balloonBand = balloonInset + balloonItemHeight + 2*balloonPad + balloonInset
)

// balloonsShown is whether the stage's foot carries the footer's balloons:
// a session's terminal or a layout of them, and a project with no session.
// A plain terminal, a service's console and Home carry none, and nor does a
// stage the Git panel or an extension's panel rises over.
func (u *window) balloonsShown(v stageView) bool {
	switch {
	case v.home, v.hasConsole, u.panelShown(v):
		return false
	case v.hasLayout:
		for _, sv := range v.panes {
			if sv.Kind != "shell" {
				return true
			}
		}
		return false
	case v.hasActive:
		return v.active.Kind != "shell"
	}
	return true
}

// ptyBand is the height a terminal on the stage leaves the balloons at its
// foot, 0 where there are none. It follows the session's kind rather than
// the agent live in its PTY, which comes and goes, so the grid is not resized
// when an agent starts in a plain terminal.
func (u *window) ptyBand(gtx C, v stageView) int {
	if (!v.hasLayout && !v.hasActive) || !u.balloonsShown(v) {
		return 0
	}
	return gtx.Dp(balloonBand)
}

// panelShown is whether the Git panel or an extension's panel is up, rising
// from the stage's foot over the balloons' band.
func (u *window) panelShown(v stageView) bool {
	if u.git != nil && u.gitShown(v) {
		return true
	}
	if u.extPanel == nil {
		return false
	}
	_, shown := u.extPanelShown(v, v.in.placed)
	return shown
}

// drawBalloons lays the balloons over the foot of gtx's area, the island at
// islandAt, when the stage carries them, and stands the toasts over the right
// one.
func (u *window) drawBalloons(gtx C, v stageView) {
	if !u.balloonsShown(v) {
		return
	}
	u.footer.layoutBalloons(gtx, u.theme)
	u.toastFloor = gtx.Dp(gutter) + gtx.Dp(balloonInset) + gtx.Dp(balloonItemHeight) + 2*gtx.Dp(balloonPad)
	u.toastRight = u.islandAt.X + gtx.Constraints.Max.X - gtx.Dp(balloonInset)
}

// layoutBalloons draws the layout's left side in a balloon at the bottom-left
// corner of gtx's area and its right side at the bottom-right, from what
// frame kept. Each is one line: the right side is measured first, the left
// takes the room left, and an item that does not fit is left out with the
// ones after it. A right press on either opens the menu that leads to
// Settings › Appearance › Footer.
func (f *footer) layoutBalloons(gtx C, th *ui.Theme) {
	in := f.shown
	f.press(in)
	l, _ := f.snapshot()
	size := gtx.Constraints.Max
	inset := gtx.Dp(balloonInset)
	y := size.Y - inset - gtx.Dp(balloonItemHeight) - 2*gtx.Dp(balloonPad)
	on := th.On(th.Sidebar)
	right, rw := f.balloon(gtx, on, footerSlots(l.Right, in), in, size.X-2*inset)
	left, lw := f.balloon(gtx, on, withDelegates(footerSlots(l.Left, in), in), in, size.X-3*inset-rw)
	f.placeBalloon(gtx, th, 0, left, image.Pt(inset, y), lw)
	f.placeBalloon(gtx, th, 1, right, image.Pt(size.X-inset-rw, y), rw)
}

// balloon records slots on a pill at most maxW wide, and answers its width,
// 0 when not one item fits.
func (f *footer) balloon(gtx C, th *ui.Theme, slots []footerSlot, in footerInput, maxW int) (op.CallOp, int) {
	pad, gap, itemH := gtx.Dp(balloonPad), gtx.Dp(balloonGap), gtx.Dp(balloonItemHeight)
	room := maxW - 2*pad
	type item struct {
		call op.CallOp
		at   image.Point
	}
	var items []item
	x := 0
	for _, s := range slots {
		m := op.Record(gtx.Ops)
		cg := gtx
		cg.Constraints = layout.Constraints{Max: image.Pt(max(room-x, 0), itemH)}
		d := f.slot(cg, th, s, in)
		call := m.Stop()
		if x+d.Size.X > room {
			break
		}
		items = append(items, item{call, image.Pt(pad+x, pad+(itemH-d.Size.Y)/2)})
		x += d.Size.X + gap
	}
	if len(items) == 0 {
		return op.CallOp{}, 0
	}
	w := x - gap + 2*pad
	m := op.Record(gtx.Ops)
	pill := image.Rectangle{Max: image.Pt(w, itemH+2*pad)}
	paint.FillShape(gtx.Ops, th.Sidebar, clip.UniformRRect(pill, gtx.Dp(ui.RadiusLG)).Op(gtx.Ops))
	for _, it := range items {
		t := op.Offset(it.at).Push(gtx.Ops)
		it.call.Add(gtx.Ops)
		t.Pop()
	}
	return m.Stop(), w
}

// placeBalloon draws a recorded balloon w wide at at, under its side's menu.
func (f *footer) placeBalloon(gtx C, th *ui.Theme, side int, call op.CallOp, at image.Point, w int) {
	if w == 0 {
		return
	}
	defer op.Offset(at).Push(gtx.Ops).Pop()
	entries := []ui.MenuEntry{{
		Label:   i18n.T("footer.footerMenu.customize"),
		OnClick: func() { f.customize() },
	}}
	size := image.Pt(w, gtx.Dp(balloonItemHeight)+2*gtx.Dp(balloonPad))
	gtx.Constraints = layout.Exact(size)
	th.ContextMenu(f.root, &f.menus[side], entries).Layout(gtx, func(gtx C) D {
		call.Add(gtx.Ops)
		return D{Size: size}
	})
}
