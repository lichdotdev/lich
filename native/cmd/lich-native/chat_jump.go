package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var icChatLatest = icons.Lucide("arrow-down")

// chatJumpGap is the room between the pill and the list's foot, the composer
// under it.
var chatJumpGap = ui.Space(3)

// jumpToLatest draws, over the foot of a list of height h scrolled up from the
// end it follows, the pill that runs it back there: "Latest", or "New
// activity" with a dot while the agent works below. It is gone at the end.
func (v *chatView) jumpToLatest(gtx C, th *ui.Theme, h int, live bool) {
	if v.jump.Clicked(gtx) {
		v.toEnd(gtx.Now)
	}
	if !v.list.ScrollToEnd || !v.list.Position.BeforeEnd || !v.run.at.IsZero() {
		return
	}
	m := op.Record(gtx.Ops)
	gtx.Constraints.Min = image.Point{}
	d := v.jump.Layout(gtx, func(gtx C) D { return chatJumpPill(gtx, th, v.jump.Hovered(), live) })
	call := m.Stop()
	x, y := (gtx.Constraints.Max.X-d.Size.X)/2, h-d.Size.Y-gtx.Dp(chatJumpGap)
	defer op.Offset(image.Pt(x, y)).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
}

func chatJumpPill(gtx C, th *ui.Theme, hovered, live bool) D {
	fg := th.MutedForeground
	if hovered {
		fg = th.Foreground
	}
	words := th.Text(ui.TextXS, i18n.T("chat.view.latest")).In(fg)
	if live {
		words = th.Text(ui.TextXS, i18n.T("chat.view.newActivity")).In(fg).Shimmer()
	}
	row := []layout.FlexChild{}
	if live {
		row = append(row, layout.Rigid(func(gtx C) D {
			px := gtx.Dp(ui.Space(1.5))
			paint.FillShape(gtx.Ops, th.Primary, clip.Ellipse{Max: image.Pt(px, px)}.Op(gtx.Ops))
			return D{Size: image.Pt(px, px)}
		}), ui.Gap(1.5))
	}
	row = append(row,
		layout.Rigid(func(gtx C) D { return icChatLatest.Layout(gtx, ui.Space(3.5), fg) }),
		ui.Gap(1.5),
		layout.Rigid(words.Layout),
	)
	m := op.Record(gtx.Ops)
	d := layout.Inset{Left: ui.Space(2.5), Right: ui.Space(3), Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
	})
	content := m.Stop()
	r := d.Size.Y / 2
	paint.FillShape(gtx.Ops, th.Popover, clip.UniformRRect(image.Rectangle{Max: d.Size}, r).Op(gtx.Ops))
	ui.InsetBorder(gtx, d.Size, r, th.Border)
	content.Add(gtx.Ops)
	return d
}
