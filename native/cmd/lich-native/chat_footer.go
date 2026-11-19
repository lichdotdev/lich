package main

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	// chatFootPad is the composer's inset around its chips, so their
	// RadiusMD hover sits concentric in the box's RadiusLG.
	chatFootPad = ui.Space(1.5)
	// chatCrumbsTop and chatCrumbGlyph size the line under the box: a TextXS
	// line with glyphs a step under the footer's.
	chatCrumbsTop  = ui.Space(1.5)
	chatCrumbGlyph = ui.Space(3)
)

// chatCrumbGap is the space either side of the line's slash, in ui.Space units.
const chatCrumbGap = 2

// chatFootSlots splits the footer's items for a chat into the chips of the
// composer, the layout's left side then its right, and the items that stand
// in the line under it, the checkout and the path.
func chatFootSlots(l footerLayout, in footerInput) (chips []footerSlot, checkout, path *footerSlot) {
	all := append(withDelegates(footerSlots(l.Left, in), in), footerSlots(l.Right, in)...)
	for i := range all {
		switch all[i].ID {
		case "checkout":
			checkout = &all[i]
		case "path":
			path = &all[i]
		default:
			chips = append(chips, all[i])
		}
	}
	return chips, checkout, path
}

// chatChips is the foot of the composer's box: the chips on the left, on one
// line, then Send or Stop at the right. A chip that does not fit is left out
// with the ones after it.
func (u *window) chatChips(th *ui.Theme, p *chatPane, working bool, chips []footerSlot) func(gtx C) D {
	f, in := u.footer, u.footer.shown
	return func(gtx C) D {
		pad, gap, itemH := gtx.Dp(chatFootPad), gtx.Dp(balloonGap), gtx.Dp(balloonItemHeight)
		width := gtx.Constraints.Max.X
		gtx.Constraints.Min = image.Point{}
		m := op.Record(gtx.Ops)
		action := chatAction(gtx, th, p, working)
		actionCall := m.Stop()
		draw := func(gtx C, s footerSlot) D {
			if s.ID == "attach" {
				return f.attachButton(gtx, th, in, icPlus)
			}
			return f.slot(gtx, th, s, in)
		}
		items, _ := f.fit(gtx, chips, width-2*pad-action.Size.X-gap, gap, itemH, draw)
		h := max(itemH, action.Size.Y)
		for _, it := range items {
			t := op.Offset(it.at.Add(image.Pt(pad, (h-itemH)/2))).Push(gtx.Ops)
			it.call.Add(gtx.Ops)
			t.Pop()
		}
		t := op.Offset(image.Pt(width-pad-action.Size.X, (h-action.Size.Y)/2)).Push(gtx.Ops)
		actionCall.Add(gtx.Ops)
		t.Pop()
		return D{Size: image.Pt(width, h+pad)}
	}
}

// chatAction is Send, or Stop while the agent works.
func chatAction(gtx C, th *ui.Theme, p *chatPane, working bool) D {
	if working {
		b := th.Button(&p.stop, i18n.T("chat.composer.stop"))
		b.Variant, b.Size, b.Icon = ui.VariantOutline, ui.SizeSM, icChatStop
		return b.Layout(gtx)
	}
	b := th.Button(&p.send, i18n.T("chat.composer.send"))
	b.Size, b.Icon = ui.SizeSM, icChatSend
	return b.Layout(gtx)
}

// chatCrumbs is the line under the composer's box, with no fill: the
// project, a slash and the branch, which opens the Git panel as its balloon
// does, then the path where the layout has it.
func (u *window) chatCrumbs(gtx C, th *ui.Theme, sv sessionView, checkout, path *footerSlot) D {
	f, in := u.footer, u.footer.shown
	var row []layout.FlexChild
	sep := func() {
		row = append(row, ui.Gap(chatCrumbGap), layout.Rigid(th.Text(ui.TextXS, "/").In(th.Over(ui.Alpha(th.MutedForeground, separatorOpacity))).Layout), ui.Gap(chatCrumbGap))
	}
	if checkout != nil {
		names, _ := u.model.projectNamesAndPaths()
		if name := names[sv.projectID]; name != "" {
			row = append(row, layout.Rigid(func(gtx C) D { return crumbText(gtx, th, icFolder, name) }))
			sep()
		}
		ic := icFolder
		if in.git.Branch != "" {
			ic = icBranch
		}
		link := f.git != nil && in.inRepository()
		row = append(row, layout.Rigid(func(gtx C) D { return f.crumb(gtx, th, "checkout", ic, *checkout, link) }))
	}
	if path != nil {
		if checkout != nil {
			row = append(row, ui.Gap(chatCrumbGap))
		}
		row = append(row, layout.Rigid(func(gtx C) D { return f.crumb(gtx, th, "path", icFolder, *path, false) }))
	}
	if len(row) == 0 {
		return D{}
	}
	return layout.Inset{Top: chatCrumbsTop, Left: chatFootPad, Right: chatFootPad}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
	})
}

// crumb is a footer item drawn as a line of text under a tooltip, muted and
// lit on hover when it is a link.
func (f *footer) crumb(gtx C, th *ui.Theme, id string, ic icons.Glyph, s footerSlot, link bool) D {
	w := f.w(id)
	body := func(gtx C) D {
		fg := th.MutedForeground
		if link && w.click.Hovered() {
			fg = th.Foreground
		}
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(checkoutMaxWidth))
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return ic.Layout(gtx, chatCrumbGlyph, fg) }),
			ui.Gap(1.5),
			layout.Rigid(func(gtx C) D { return th.Text(ui.TextXS, s.Text).In(fg).Truncated().Layout(gtx) }),
		)
	}
	if !link {
		return th.Tooltip(f.root, &w.tip, s.Hint).Layout(gtx, body)
	}
	return f.tour.Anchor(gtx, worktreesBranch, func(gtx C) D {
		return th.Tooltip(f.root, &w.tip, s.Hint).Layout(gtx, func(gtx C) D { return w.click.Layout(gtx, body) })
	})
}

// crumbText is an icon and a name in muted text, with no action.
func crumbText(gtx C, th *ui.Theme, ic icons.Glyph, text string) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return ic.Layout(gtx, chatCrumbGlyph, th.MutedForeground) }),
		ui.Gap(1.5),
		layout.Rigid(th.Text(ui.TextXS, text).Muted().Layout),
	)
}
