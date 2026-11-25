package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/lichdotdev/lich/api"
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
	// layoutBalloonBand is the band under a layout's panes, which are islands
	// of their own: the balloon on the stage's foot and a gutter over it.
	layoutBalloonBand = balloonItemHeight + 2*balloonPad + gutter
)

// balloonsShown is whether the stage's foot carries the footer's balloons:
// a session's terminal, a layout holding an agent's terminal whichever pane
// has the focus, and a project with no session. A plain terminal, a service's
// console, the stage editor and Home carry none, nor does a stage the Git
// panel or an extension's panel rises over, a session alone as a chat or a
// layout of chats: a chat's footer stands in its composer.
func (u *window) balloonsShown(v stageView) bool {
	switch {
	case v.home, v.hasConsole, v.hasEditor, u.panelShown(v):
		return false
	case v.hasLayout:
		for _, sv := range v.panes {
			if sv.Kind != "shell" && sv.View != api.ViewChat {
				return true
			}
		}
		return false
	case v.hasActive && v.active.View == api.ViewChat:
		return false
	case v.hasActive:
		return v.active.Kind != "shell"
	}
	return true
}

// ptyBand is the height a terminal on the stage leaves the balloons at its
// foot, 0 where there are none. It follows the session's kind rather than
// the agent live in its PTY, which comes and goes, so the grid is not resized
// when an agent starts in a plain terminal. A layout's band does not follow
// its focus, so moving the focus resizes no pane.
func (u *window) ptyBand(gtx C, v stageView) int {
	switch {
	case (!v.hasLayout && !v.hasActive) || !u.balloonsShown(v):
		return 0
	case v.hasLayout:
		return gtx.Dp(layoutBalloonBand)
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

// drawBalloons lays the balloons over the foot of gtx's area, the stage at
// islandAt, inset from its sides and foot, when the stage carries them, and
// stands the toasts over the right one.
func (u *window) drawBalloons(gtx C, v stageView, inset int) {
	if !u.balloonsShown(v) {
		return
	}
	u.footer.layoutBalloons(gtx, u.theme, inset)
	u.toastFloor = gtx.Dp(gutter) + inset + gtx.Dp(balloonItemHeight) + 2*gtx.Dp(balloonPad)
	u.toastRight = u.islandAt.X + gtx.Constraints.Max.X - inset
}

// layoutBalloons draws the layout's left side in a balloon at the bottom-left
// corner of gtx's area and its right side at the bottom-right, inset from
// the sides and the foot, from what frame kept. Each is one line: the right side is measured first, the left
// takes the room left, and an item that does not fit is left out with the
// ones after it. A right press on either opens the menu that leads to
// Settings › Appearance › Footer.
func (f *footer) layoutBalloons(gtx C, th *ui.Theme, inset int) {
	in := f.shown
	f.press(in)
	l, _ := f.snapshot()
	size := gtx.Constraints.Max
	y := size.Y - inset - gtx.Dp(balloonItemHeight) - 2*gtx.Dp(balloonPad)
	on := th.On(th.Sidebar)
	right, rw := f.balloon(gtx, on, footerSlots(l.Right, in), in, size.X-2*inset)
	left, lw := f.balloon(gtx, on, withDelegates(footerSlots(l.Left, in), in), in, size.X-2*inset-gtx.Dp(balloonInset)-rw)
	f.placeBalloon(gtx, th, 0, left, image.Pt(inset, y), lw)
	f.placeBalloon(gtx, th, 1, right, image.Pt(size.X-inset-rw, y), rw)
}

// balloon records slots on a pill at most maxW wide, and answers its width,
// 0 when not one item fits.
func (f *footer) balloon(gtx C, th *ui.Theme, slots []footerSlot, in footerInput, maxW int) (op.CallOp, int) {
	pad, itemH := gtx.Dp(balloonPad), gtx.Dp(balloonItemHeight)
	items, iw := f.fit(gtx, slots, maxW-2*pad, gtx.Dp(balloonGap), itemH, func(gtx C, s footerSlot) D { return f.slot(gtx, th, s, in) })
	return balloonPill(gtx, th, items, iw)
}

// balloonPill records fitted items, iw wide, on the balloon's pill, and
// answers its width, 0 when there are none.
func balloonPill(gtx C, th *ui.Theme, items []fitted, iw int) (op.CallOp, int) {
	if len(items) == 0 {
		return op.CallOp{}, 0
	}
	pad, itemH := gtx.Dp(balloonPad), gtx.Dp(balloonItemHeight)
	w := iw + 2*pad
	m := op.Record(gtx.Ops)
	pill := image.Rectangle{Max: image.Pt(w, itemH+2*pad)}
	// it floats over the terminal's text, where it would read as one of its lines
	ui.ElevationLow.Paint(gtx, th, pill.Max, gtx.Dp(ui.RadiusLG))
	paint.FillShape(gtx.Ops, th.Sidebar, clip.UniformRRect(pill, gtx.Dp(ui.RadiusLG)).Op(gtx.Ops))
	for _, it := range items {
		t := op.Offset(it.at.Add(image.Pt(pad, pad))).Push(gtx.Ops)
		it.call.Add(gtx.Ops)
		t.Pop()
	}
	return m.Stop(), w
}

// fitted is one recorded item and where it stands, centred in a line itemH
// tall.
type fitted struct {
	call op.CallOp
	at   image.Point
}

// fit records the slots drawn with draw side by side, gap apart, on one line
// room wide: an item that does not fit is left out with the ones after it.
// It answers them and their width, 0 when not one fits.
func (f *footer) fit(gtx C, slots []footerSlot, room, gap, itemH int, draw func(gtx C, s footerSlot) D) ([]fitted, int) {
	var items []fitted
	x := 0
	for _, s := range slots {
		m := op.Record(gtx.Ops)
		cg := gtx
		cg.Constraints = layout.Constraints{Max: image.Pt(max(room-x, 0), itemH)}
		d := draw(cg, s)
		call := m.Stop()
		if x+d.Size.X > room {
			break
		}
		items = append(items, fitted{call, image.Pt(x, (itemH-d.Size.Y)/2)})
		x += d.Size.X + gap
	}
	if len(items) == 0 {
		return nil, 0
	}
	return items, x - gap
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
