package main

import (
	"image"
	"image/color"
	"slices"
	"strings"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

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

// The chips inside the box are the provider's: the attach "+", the model with
// its effort, the mode. A slot with one of these ids is drawn by chatChips.
const (
	chipModel = "option:model"
	chipMode  = "option:mode"
)

var (
	// chatFootPad is the composer's inset around its chips, so their
	// RadiusMD hover sits concentric in the box's RadiusLG.
	chatFootPad = ui.Space(1.5)
	// chatCrumbsPad is the room over the line, between the box and it, and
	// under it, between it and the pane's island's bottom edge (over the
	// composer's own pad). chatCrumbGlyph sizes the line's glyphs, a step
	// under the footer's, on a TextXS line.
	chatCrumbsPad  = ui.Space(2)
	chatCrumbGlyph = ui.Space(3)
	// chatChangesPadX and chatChangesPadY are the room the changes item's hover
	// fill keeps round its text: a ghost button's, which the line's left side has
	// none of to follow. The fill grows outward; the text does not move.
	chatChangesPadX = ui.Space(1.5)
	chatChangesPadY = ui.Space(1)
)

// chatCrumbGap is the room between the line's items: the project, the
// branch, its changes and its pull request, one rhythm along the line.
const chatCrumbGap = 3

// optionSlots are the chips of the options session id's provider offers: the
// model's, which also holds the effort, and the mode's. A category it does not
// offer has none.
func (c *chatStore) optionSlots(id string) []footerSlot {
	var out []footerSlot
	_, model := c.optionOf(id, api.OptionModel)
	_, effort := c.optionOf(id, api.OptionEffort)
	if model || effort {
		out = append(out, footerSlot{ID: chipModel})
	}
	if _, ok := c.optionOf(id, api.OptionMode); ok {
		out = append(out, footerSlot{ID: chipMode})
	}
	return out
}

// chatFooter is the footer's items a chat shows of the user's layout: the
// attach "+" and the context ring inside the box, the checkout and the changes
// under it. The rest
// of the footer is not drawn in a chat.
type chatFooter struct {
	attach, changes, checkout, context *footerSlot
}

func chatFootSlots(l footerLayout, in footerInput) chatFooter {
	var f chatFooter
	for _, s := range footerSlots(append(slices.Clone(l.Left), l.Right...), in) {
		switch s.ID {
		case "attach":
			f.attach = &s
		case "changes":
			f.changes = &s
		case "checkout":
			f.checkout = &s
		case "context":
			f.context = &s
		}
	}
	return f
}

// chatChips is the foot of the composer's box: the attach "+" when the footer
// has it, then the provider's chips, on one line, and Send or Stop at the
// right. A chip that does not fit is left out with the ones after it. In
// shell mode the chips, which do not apply to a command, give their place to
// the mode's name.
func (u *window) chatChips(th *ui.Theme, p *chatPane, sv sessionView, working bool, ft chatFooter) func(gtx C) D {
	f, in := u.footer, u.footer.shown
	var chips []footerSlot
	if ft.attach != nil {
		chips = append(chips, *ft.attach)
	}
	chips = append(chips, u.model.chat.optionSlots(sv.ID)...)
	if n, monitors := runningShells(u.model.chat.shellsOf(sv.ID)); n+monitors > 0 {
		chips = append(chips, footerSlot{ID: chipShells})
	}
	if ft.context != nil {
		chips = append(chips, *ft.context)
	}
	return func(gtx C) D {
		pad, gap, itemH := gtx.Dp(chatFootPad), gtx.Dp(balloonGap), gtx.Dp(balloonItemHeight)
		width := gtx.Constraints.Max.X
		gtx.Constraints.Min = image.Point{}
		m := op.Record(gtx.Ops)
		action := u.chatAction(gtx, th, p, working)
		actionCall := m.Stop()
		// while messages wait in the queue the way back to them is named
		// before the action
		var hint D
		var hintCall op.CallOp
		_, shell := p.shellLine(sv.Kind)
		if !shell && len(u.model.chat.queued(sv.ID)) > 0 {
			m = op.Record(gtx.Ops)
			hint = th.Text(ui.TextXS, i18n.T("chat.queue.edit")).Muted().Layout(gtx)
			hintCall = m.Stop()
			hint.Size.X += gap
		}
		draw := func(gtx C, s footerSlot) D {
			switch s.ID {
			case "attach":
				return f.attachButton(gtx, th, in, icPlus)
			case chipModel:
				return u.modelChip(gtx, th, p, sv, working)
			case chipMode:
				return u.modeChip(gtx, th, p, sv, working)
			case chipShells:
				return u.shellsChip(gtx, th, p, sv)
			}
			return f.slot(gtx, th, s, in)
		}
		room := width - 2*pad - action.Size.X - gap - hint.Size.X
		h := max(itemH, action.Size.Y)
		// while Ctrl+R searches its field takes the chips' place, and in shell
		// mode the mode's name does, lined up with the text over it
		var lead func(gtx C) D
		leadX := pad
		switch {
		case p.search.open:
			lead = func(gtx C) D { return chatSearchLine(gtx, th, &p.search) }
		case shell:
			lead, leadX = th.Text(ui.TextXS, i18n.T("chat.composer.shell")).In(th.ToneWait).Truncated().Layout, gtx.Dp(chatComposerPadX)
		}
		if lead != nil {
			w := max(room-leadX+pad, 0)
			g := gtx
			g.Constraints = layout.Constraints{Min: image.Pt(w, 0), Max: image.Pt(w, h)}
			m = op.Record(gtx.Ops)
			line := lead(g)
			call := m.Stop()
			t := op.Offset(image.Pt(leadX, (h-line.Size.Y)/2)).Push(gtx.Ops)
			call.Add(gtx.Ops)
			t.Pop()
			chips = nil
		}
		items, _ := f.fit(gtx, chips, room, gap, itemH, draw)
		for _, it := range items {
			t := op.Offset(it.at.Add(image.Pt(pad, (h-itemH)/2))).Push(gtx.Ops)
			it.call.Add(gtx.Ops)
			t.Pop()
		}
		t := op.Offset(image.Pt(width-pad-action.Size.X, (h-action.Size.Y)/2)).Push(gtx.Ops)
		actionCall.Add(gtx.Ops)
		t.Pop()
		if hint.Size.X > 0 {
			t = op.Offset(image.Pt(width-pad-action.Size.X-hint.Size.X, (h-hint.Size.Y)/2)).Push(gtx.Ops)
			hintCall.Add(gtx.Ops)
			t.Pop()
		}
		return D{Size: image.Pt(width, h+pad)}
	}
}

// chatAction is Send, or Stop while the agent works: an icon under a tooltip
// that names it.
func (u *window) chatAction(gtx C, th *ui.Theme, p *chatPane, working bool) D {
	if working {
		b := th.Button(&p.stop, "")
		b.Variant, b.Size, b.Icon = ui.VariantOutline, ui.SizeIconSM, icChatStop
		return th.Tooltip(&u.root, &p.stopTip, i18n.T("chat.composer.stop")).Layout(gtx, b.Layout)
	}
	b := th.Button(&p.send, "")
	b.Size, b.Icon = ui.SizeIconSM, icChatSend
	return th.Tooltip(&u.root, &p.sendTip, i18n.T("chat.composer.send")).Layout(gtx, b.Layout)
}

// chatCrumbs is the line under the composer's box, with no fill: at the left
// the project, a slash and the branch, which opens the Git panel as its
// balloon does, then the changes item, which opens Review, and the branch's
// pull request.
func (u *window) chatCrumbs(gtx C, th *ui.Theme, sv sessionView, in footerInput, ft chatFooter, size unit.Sp) D {
	f := u.footer
	// The changes are there when there are some.
	if in.git.Files == 0 {
		ft.changes = nil
	}
	// A pull request waits for its checkout's forge, so no wrong logo flashes.
	if ft.checkout == nil && ft.changes == nil && (sv.pr == nil || sv.forge == "") {
		return D{}
	}
	// The footer's clicks (the Git panel, Review, the pull request) act on the
	// session it follows. Another pane's items are the same text under widgets
	// of their own, which take no click, so that pane cannot open the other's.
	followed := sv.ID == f.shown.sessionID
	suffix := ""
	if !followed {
		suffix = "@" + sv.ID
		for _, s := range []*footerSlot{ft.checkout, ft.changes} {
			if s != nil {
				s.Pane = suffix
			}
		}
	}
	tail, reserve := u.crumbsTail(gtx, th, sv, in, ft, followed, size)
	var left []layout.FlexChild
	if ft.checkout != nil {
		names, _ := u.model.projectNamesAndPaths()
		if name := names[sv.projectID]; name != "" {
			left = append(left, layout.Rigid(func(gtx C) D { return crumbText(gtx, th, icFolder, name, size) }), ui.Gap(chatCrumbGap))
		}
		ic := icFolder
		if in.git.Branch != "" {
			ic = icBranch
		}
		link := followed && f.git != nil && in.inRepository()
		left = append(left, layout.Rigid(func(gtx C) D {
			// the branch truncates first, so what follows it is never cut
			gtx.Constraints.Max.X = max(0, gtx.Constraints.Max.X-reserve)
			return f.crumb(gtx, th, "checkout"+suffix, ic, *ft.checkout, link, size)
		}))
	}
	left = append(left, tail...)
	return layout.Inset{Top: chatCrumbsPad, Bottom: chatCrumbsPad, Left: chatFootPad, Right: chatFootPad}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, left...)
	})
}

// crumbsTail is what follows the branch, the changes' line counts and the pull
// request, laid out ahead of it, and the room they take with their gaps.
func (u *window) crumbsTail(gtx C, th *ui.Theme, sv sessionView, in footerInput, ft chatFooter, followed bool, size unit.Sp) ([]layout.FlexChild, int) {
	var items []layout.Widget
	if ft.changes != nil {
		items = append(items, func(gtx C) D { return u.footer.crumbChanges(gtx, th, *ft.changes, in.git, followed, size) })
	}
	if sv.pr != nil && sv.forge != "" {
		items = append(items, func(gtx C) D { return u.chatPRBadge(gtx, th, sv, followed, size) })
	}
	gtx.Constraints.Min = image.Point{}
	var tail []layout.FlexChild
	room := 0
	for i, item := range items {
		m := op.Record(gtx.Ops)
		d := item(gtx)
		call := m.Stop()
		if i > 0 || ft.checkout != nil {
			tail, room = append(tail, ui.Gap(chatCrumbGap)), room+gtx.Dp(ui.Space(chatCrumbGap))
		}
		tail, room = append(tail, layout.Rigid(func(gtx C) D { call.Add(gtx.Ops); return d })), room+d.Size.X
	}
	return tail, room
}

// chatPRBadge is the session's pull request on the composer's line: a click
// opens it where its row's menu does, under a tooltip of its title. Another
// pane's takes no click.
func (u *window) chatPRBadge(gtx C, th *ui.Theme, sv sessionView, followed bool, size unit.Sp) D {
	w := u.footer.w("pr@" + sv.ID)
	logo := th.MutedForeground
	badge := func(gtx C) D { return prBadge(gtx, th, sv.pr, sv.forge, logo, size) }
	if followed {
		if w.click.Clicked(gtx) {
			u.openPR(prTarget(sv.pr, sv.shown, sv.path))
		}
		if w.click.Hovered() {
			logo = th.Foreground
		}
		draw := badge
		badge = func(gtx C) D { return w.click.Layout(gtx, draw) }
	}
	if sv.pr.Title == "" {
		return badge(gtx)
	}
	return th.Tooltip(u.footer.root, &w.tip, sv.pr.Title).Layout(gtx, badge)
}

// crumb is a footer item drawn as a line of text under a tooltip, muted and
// lit on hover when it is a link.
func (f *footer) crumb(gtx C, th *ui.Theme, id string, ic icons.Glyph, s footerSlot, link bool, size unit.Sp) D {
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
			layout.Rigid(func(gtx C) D { return th.Text(size, s.Text).In(fg).Truncated().Layout(gtx) }),
		)
	}
	if !link {
		return th.Tooltip(f.root, &w.tip, s.Hint).Layout(gtx, body)
	}
	return f.tour.Anchor(gtx, worktreesBranch, func(gtx C) D {
		return th.Tooltip(f.root, &w.tip, s.Hint).Layout(gtx, func(gtx C) D { return w.click.Layout(gtx, body) })
	})
}

// crumbChanges is the changes after the branch: the line counts in size, with
// no fill at rest and the kit's hover fill under the pointer. Its click opens
// Review, for the session the footer follows. It is never drawn pressed, as the Review button of the footer is
// while the dock shows Review: on this line a lit item reads as a button
// switched on for no reason.
func (f *footer) crumbChanges(gtx C, th *ui.Theme, s footerSlot, git backend.DiffStats, followed bool, size unit.Sp) D {
	w := f.w(s.ID + s.Pane)
	return th.Tooltip(f.root, &w.tip, i18n.T("shell.footerBar.reviewChanges")).Layout(gtx, func(gtx C) D {
		// The hover fill grows outward from the text: the item takes the room
		// of its text alone, so the text stands where the left side's does,
		// chatFootPad in from the box's edge.
		padX, padY := gtx.Dp(chatChangesPadX), gtx.Dp(chatChangesPadY)
		m := op.Record(gtx.Ops)
		out := op.Offset(image.Pt(-padX, -padY)).Push(gtx.Ops)
		d := w.click.Layout(gtx, func(gtx C) D {
			var bg color.NRGBA
			if w.click.Hovered() {
				bg = th.Over(ui.Alpha(th.Accent, ui.GhostHoverFill))
			}
			return layout.Background{}.Layout(gtx,
				func(gtx C) D {
					if followed {
						area := clip.Rect{Max: gtx.Constraints.Min}.Push(gtx.Ops)
						pointer.CursorPointer.Add(gtx.Ops)
						area.Pop()
					}
					return ui.FillIf(gtx, bg, ui.RadiusMD)
				},
				func(gtx C) D {
					return layout.Inset{Left: chatChangesPadX, Right: chatChangesPadX, Top: chatChangesPadY, Bottom: chatChangesPadY}.Layout(gtx, func(gtx C) D {
						stat := th.DiffStat(git.Added, git.Deleted)
						stat.Size = size
						return stat.Layout(gtx)
					})
				},
			)
		})
		out.Pop()
		call := m.Stop()
		call.Add(gtx.Ops)
		return D{Size: image.Pt(d.Size.X-2*padX, d.Size.Y-2*padY), Baseline: d.Baseline - padY}
	})
}

// crumbText is an icon and a name in muted text, with no action.
func crumbText(gtx C, th *ui.Theme, ic icons.Glyph, text string, size unit.Sp) D {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return ic.Layout(gtx, chatCrumbGlyph, th.MutedForeground) }),
		ui.Gap(1.5),
		layout.Rigid(th.Text(size, text).Muted().Layout),
	)
}

// modelChip is the chip of the model and its effort: the two choices in
// force, and the panel that sets either (chat_picker.go). While the agent
// works it takes no click and says why.
func (u *window) modelChip(gtx C, th *ui.Theme, p *chatPane, sv sessionView, working bool) D {
	var opts []backend.ChatOption
	var names []string
	for _, cat := range []string{api.OptionModel, api.OptionEffort} {
		if o, ok := u.model.chat.optionOf(sv.ID, cat); ok {
			cur, _ := currentChoice(o)
			opts, names = append(opts, o), append(names, choiceLabel(cur))
		}
	}
	if len(opts) == 0 {
		return D{}
	}
	var glyph icons.Glyph = icEffort
	title := optionTitle(opts[0])
	if opts[0].Category == api.OptionModel {
		glyph = providerIcon(sv.Kind)
	}
	if len(opts) == 2 {
		title = i18n.T("chat.option.modelEffort")
	}
	content := optionContent(glyph, strings.Join(names, " · "))
	w := u.footer.w(chipModel)
	if working {
		return th.Tooltip(&u.root, &w.tip, i18n.T("chat.option.busy")).Layout(gtx, func(gtx C) D {
			return buttonFace(gtx, th, true, false, true, content)
		})
	}
	pk := &p.picker
	if !pk.panel.Opened() {
		pk.fresh = true
	}
	width := pickerModelW + pickerEffortW + pickerChrome
	if len(opts) == 1 && opts[0].Category == api.OptionEffort {
		width = pickerEffortW + pickerChrome - 1
	}
	panel := th.DropdownPanel(&u.root, &pk.panel, width, func(gtx C, pth *ui.Theme) D { return u.modelPanel(gtx, pth, p, sv) })
	panel.MaxHeight = pickerMaxH
	panel = panel.Above()
	return th.Tooltip(&u.root, &w.tip, title).Layout(gtx, func(gtx C) D {
		return panel.Layout(gtx, func(gtx C) D { return buttonFace(gtx, th, true, pk.panel.Opened(), false, content) })
	})
}

// modeChip is the chip of the mode in force, under lich's name for the
// option, with a bolt and its name in the rainbow while it is a dangerous one.
func (u *window) modeChip(gtx C, th *ui.Theme, p *chatPane, sv sessionView, working bool) D {
	opt, ok := u.model.chat.optionOf(sv.ID, api.OptionMode)
	if !ok {
		return D{}
	}
	cur, _ := currentChoice(opt)
	content := optionContent(icMode, modeLabel(sv.Kind, cur))
	if cur.Danger {
		content = rainbowContent(modeLabel(sv.Kind, cur))
	}
	return u.optionChip(gtx, th, p, chipMode, i18n.T("chat.option.mode"), working, content, func() []ui.MenuEntry {
		return u.modeEntries(sv, opt)
	})
}

// optionChip draws content as a chip under a tooltip naming it, opening a menu
// of entries on a click; disabled, with the reason as its tooltip, while the
// agent works.
func (u *window) optionChip(gtx C, th *ui.Theme, p *chatPane, id, title string, working bool, content footerContent, entries func() []ui.MenuEntry) D {
	w := u.footer.w(id)
	if working {
		return th.Tooltip(&u.root, &w.tip, i18n.T("chat.option.busy")).Layout(gtx, func(gtx C) D {
			return buttonFace(gtx, th, true, false, true, content)
		})
	}
	menu := p.optionMenu(id)
	return th.Tooltip(&u.root, &w.tip, title).Layout(gtx, func(gtx C) D {
		return th.DropdownMenu(&u.root, menu, entries()).Layout(gtx, func(gtx C) D {
			return buttonFace(gtx, th, true, menu.Opened(), false, content)
		})
	})
}

// modeEntries is the mode's menu: each choice, the one in force checked, a
// dangerous one in the rainbow, with a bolt when it is not checked.
func (u *window) modeEntries(sv sessionView, opt backend.ChatOption) []ui.MenuEntry {
	entries := make([]ui.MenuEntry, len(opt.Choices))
	for i, c := range opt.Choices {
		e := ui.MenuEntry{Kind: ui.MenuItem, Label: modeLabel(sv.Kind, c), Inset: true, Rainbow: c.Danger, OnClick: func() { u.setChatOption(sv.ID, opt.ID, c.ID) }}
		switch {
		case c.ID == opt.Current:
			e.Icon, e.Inset = icCheck, false
		case c.Danger:
			e.Lead, e.Inset = rainbowBolt(u.theme), false
		}
		entries[i] = e
	}
	return entries
}

func (u *window) setChatOption(id, optionID, value string) {
	go func() { u.chatFailed(u.client.ChatSetOption(u.ctx, id, optionID, value)) }()
}

// optionContent is a glyph, the choice's name cut at modelMaxWidth and a
// chevron.
func optionContent(glyph icons.Glyph, text string) footerContent {
	return func(gtx C, th *ui.Theme, fg color.NRGBA) D {
		return chipFace(gtx, fg, func(gtx C) D { return glyph.Layout(gtx, footerGlyph, fg) }, th.Text(ui.TextXS, text).In(fg))
	}
}

// chipFace lays out a choice's chip: glyph, name cut at modelMaxWidth and a
// chevron in fg.
func chipFace(gtx C, fg color.NRGBA, glyph layout.Widget, name ui.LabelStyle) D {
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(modelMaxWidth))
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(glyph),
		ui.Gap(1.5),
		layout.Rigid(name.Truncated().Layout),
		ui.Gap(1),
		layout.Rigid(func(gtx C) D { return icPullDown.Layout(gtx, chatCrumbGlyph, fg) }),
	)
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
	case "acceptedits":
		return i18n.T("chat.choice.acceptEdits")
	case "dontask":
		return i18n.T("chat.choice.preApproved")
	case "plan":
		return i18n.T("chat.choice.plan")
	case "auto":
		return i18n.T("chat.choice.auto")
	case "bypasspermissions", "yolo", ":danger-full-access":
		return i18n.T("chat.choice.yolo")
	case ":read-only":
		return i18n.T("chat.choice.readOnly")
	case ":workspace":
		return i18n.T("chat.choice.workspace")
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
