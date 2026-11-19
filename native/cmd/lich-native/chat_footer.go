package main

import (
	"image"
	"image/color"
	"slices"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icEffort = icons.Lucide("brain")
	icMode   = icons.Lucide("shield-check")
	icDanger = icons.Lucide("zap")
)

// optionPrefix marks a chip slot that is one of the session's options, whose
// category follows it.
const optionPrefix = "option:"

// chatOptionCategories are the options the composer offers, in the order
// their chips stand.
var chatOptionCategories = []string{api.OptionModel, api.OptionEffort, api.OptionMode}

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

// optionSlots are the chips of the options session id's provider offers: a
// category it does not offer has none.
func (c *chatStore) optionSlots(id string) []footerSlot {
	var out []footerSlot
	for _, cat := range chatOptionCategories {
		if _, ok := c.optionOf(id, cat); ok {
			out = append(out, footerSlot{ID: optionPrefix + cat})
		}
	}
	return out
}

// chatFootSlots splits the footer's items for a chat into the chips of the
// composer, the layout's left side, then the session's option chips, then its
// right side, and the items that stand in the line under it, the checkout and
// the path. The model reading gives way to the model's chip when the provider
// offers one.
func chatFootSlots(l footerLayout, in footerInput, options []footerSlot) (chips []footerSlot, checkout, path *footerSlot) {
	offersModel := slices.ContainsFunc(options, func(s footerSlot) bool { return s.ID == optionPrefix+api.OptionModel })
	left := withDelegates(footerSlots(l.Left, in), in)
	all := append(slices.Clone(left), options...)
	all = append(all, footerSlots(l.Right, in)...)
	for i := range all {
		switch id := all[i].ID; {
		case id == "checkout":
			checkout = &all[i]
		case id == "path":
			path = &all[i]
		case id == "model" && offersModel:
		default:
			chips = append(chips, all[i])
		}
	}
	return chips, checkout, path
}

// chatChips is the foot of the composer's box: the chips on the left, on one
// line, then Send or Stop at the right. A chip that does not fit is left out
// with the ones after it.
func (u *window) chatChips(th *ui.Theme, p *chatPane, sv sessionView, working bool, chips []footerSlot) func(gtx C) D {
	f, in := u.footer, u.footer.shown
	return func(gtx C) D {
		pad, gap, itemH := gtx.Dp(chatFootPad), gtx.Dp(balloonGap), gtx.Dp(balloonItemHeight)
		width := gtx.Constraints.Max.X
		gtx.Constraints.Min = image.Point{}
		m := op.Record(gtx.Ops)
		action := chatAction(gtx, th, p, working)
		actionCall := m.Stop()
		draw := func(gtx C, s footerSlot) D {
			if cat, ok := strings.CutPrefix(s.ID, optionPrefix); ok {
				return u.optionChip(gtx, th, p, sv, cat, working)
			}
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

// optionChip is the chip of session sv's option in category cat: the choice in
// force, and a menu of the others that sets one. While the agent works it
// takes no click and says why.
func (u *window) optionChip(gtx C, th *ui.Theme, p *chatPane, sv sessionView, cat string, working bool) D {
	opt, ok := u.model.chat.optionOf(sv.ID, cat)
	if !ok {
		return D{}
	}
	cur, _ := currentChoice(opt)
	var glyph icons.Glyph = icEffort
	switch {
	case cat == api.OptionModel:
		glyph = providerIcon(sv.Kind)
	case cat == api.OptionMode:
		glyph = icMode
	}
	if cur.Danger {
		glyph = icDanger
	}
	content := optionContent(glyph, choiceLabel(cur), cur.Danger)
	w := u.footer.w(optionPrefix + cat)
	if working {
		return th.Tooltip(u.footer.root, &w.tip, i18n.T("chat.option.busy")).Layout(gtx, func(gtx C) D {
			return buttonFace(gtx, th, true, false, true, content)
		})
	}
	menu := p.optionMenu(cat)
	return th.Tooltip(u.footer.root, &w.tip, optionTitle(opt)).Layout(gtx, func(gtx C) D {
		return th.DropdownMenu(u.footer.root, menu, u.optionEntries(sv.ID, opt)).Layout(gtx, func(gtx C) D {
			return buttonFace(gtx, th, true, menu.Opened(), false, content)
		})
	})
}

// optionEntries is the menu of option opt: each choice, the one in force
// checked, a dangerous one with a bolt.
func (u *window) optionEntries(id string, opt backend.ChatOption) []ui.MenuEntry {
	entries := make([]ui.MenuEntry, len(opt.Choices))
	for i, c := range opt.Choices {
		e := ui.MenuEntry{Kind: ui.MenuItem, Label: choiceLabel(c), Inset: true, OnClick: func() { u.setChatOption(id, opt.ID, c.ID) }}
		switch {
		case c.ID == opt.Current:
			e.Icon, e.Inset = icCheck, false
		case c.Danger:
			e.Icon, e.Inset = icDanger, false
		}
		entries[i] = e
	}
	return entries
}

func (u *window) setChatOption(id, optionID, value string) {
	go func() { u.chatFailed(u.client.ChatSetOption(u.ctx, id, optionID, value)) }()
}

// optionContent is a glyph, the choice's name cut at modelMaxWidth and a
// chevron; a dangerous choice is drawn in ToneWait.
func optionContent(glyph icons.Glyph, text string, danger bool) footerContent {
	return func(gtx C, th *ui.Theme, fg color.NRGBA) D {
		if danger {
			fg = th.ToneWait
		}
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(modelMaxWidth))
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return glyph.Layout(gtx, footerGlyph, fg) }),
			ui.Gap(1.5),
			layout.Rigid(func(gtx C) D { return th.Text(ui.TextXS, text).In(fg).Truncated().Layout(gtx) }),
			ui.Gap(1),
			layout.Rigid(func(gtx C) D { return icPullDown.Layout(gtx, chatCrumbGlyph, fg) }),
		)
	}
}

// currentChoice is the choice of opt in force; with none listed under that
// id, one named by the id alone.
func currentChoice(opt backend.ChatOption) (backend.ChatChoice, bool) {
	if i := slices.IndexFunc(opt.Choices, func(c backend.ChatChoice) bool { return c.ID == opt.Current }); i >= 0 {
		return opt.Choices[i], true
	}
	return backend.ChatChoice{ID: opt.Current}, false
}

// choiceLabel is the provider's name for a choice, or for one it sends by id
// alone the name lich gives the ids it knows.
func choiceLabel(c backend.ChatChoice) string {
	if c.Label != "" {
		return c.Label
	}
	switch strings.ToLower(c.ID) {
	case "default", "manual":
		return i18n.T("chat.choice.manual")
	case "plan":
		return i18n.T("chat.choice.plan")
	case "auto":
		return i18n.T("chat.choice.auto")
	case "bypasspermissions", "yolo":
		return i18n.T("chat.choice.yolo")
	}
	return c.ID
}

// optionTitle is the chip's tooltip: the provider's name for the option, or
// the category's.
func optionTitle(opt backend.ChatOption) string {
	if opt.Label != "" {
		return opt.Label
	}
	switch opt.Category {
	case api.OptionModel:
		return i18n.T("chat.option.model")
	case api.OptionEffort:
		return i18n.T("chat.option.effort")
	}
	return i18n.T("chat.option.mode")
}
