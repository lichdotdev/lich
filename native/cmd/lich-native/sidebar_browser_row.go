package main

import (
	"image"
	"net/url"

	"gioui.org/font"
	"gioui.org/layout"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// A session's browser is a node of the tree, hanging first under its
// session for as long as the Chromium runs, on screen or minimized: it is
// where a browser nobody sees is still seen. It is no session, so it files
// nowhere and a drag leaves it where it was.

const browserRowPrefix = "__browser__:"

func browserRowKey(session string) string { return browserRowPrefix + session }

// webView is what a session's browser row shows: the page it is on, and
// where it is drawn. Neither shown nor headed is a browser minimized.
type webView struct {
	url, title string
	// shown is the page beside the session's terminal, focused when it takes
	// the keys; headed is the browser in a window of its own.
	shown, focused, headed bool
}

// label is the row's name: the page's host, or Browser on a blank page.
func (w webView) label() string {
	if u, err := url.Parse(w.url); err == nil && u.Host != "" {
		return u.Host
	}
	return i18n.T("layouts.browser.title")
}

// state is what the row's tooltip says of a browser off the stage, "" for a
// page on screen.
func (w webView) state() string {
	switch {
	case w.headed:
		return i18n.T("sidebar.browserRow.inWindow")
	case !w.shown:
		return i18n.T("sidebar.browserRow.minimized")
	}
	return ""
}

// browserMenu is the row's menu: Show for a browser off the stage, Minimize
// for a page on it, a window of its own, and Close.
func browserMenu(session string, w webView, act *sidebarAction) []ui.MenuEntry {
	first := ui.MenuEntry{Label: i18n.T("sidebar.browserRow.show"), OnClick: func() { act.browser = session }}
	if w.shown {
		first = ui.MenuEntry{Label: i18n.T("sidebar.browserRow.minimize"), OnClick: func() { act.browserMinimize = session }}
	}
	entries := []ui.MenuEntry{first}
	if !w.headed {
		entries = append(entries, ui.MenuEntry{Label: i18n.T("layouts.browser.openInWindow"), OnClick: func() { act.browserWindow = session }})
	}
	return append(entries, ui.MenuEntry{Kind: ui.MenuSeparator},
		ui.MenuEntry{Label: i18n.T("layouts.browser.close"), Variant: ui.VariantDestructive, OnClick: func() { act.browserClose = session }})
}

// browserTooltip is what the row's line cuts: the page's title, its address,
// and where the browser is when it is not on the stage.
func browserTooltip(th *ui.Theme, w webView) layout.Widget {
	th = th.On(th.Popover)
	var sections []layout.Widget
	if w.title != "" {
		sections = append(sections, para(th.Text(ui.TextXS, w.title).Weight(font.Medium)).Layout)
	}
	if address := displayURL(w.url); address != "" {
		sections = append(sections, para(th.Text(ui.TextXS, address).Muted().Mono()).Layout)
	}
	if state := w.state(); state != "" {
		sections = append(sections, th.Text(ui.TextXS, state).Muted().Layout)
	}
	if len(sections) == 0 {
		sections = append(sections, th.Text(ui.TextXS, w.label()).Layout)
	}
	return func(gtx C) D { return ui.Stack(1.5, sections...)(gtx) }
}

// browserRow is a session's browser in the tree: the globe in a neutral
// ring and the page's host, muted while the browser is off the stage. A
// click shows the page; active is its session on screen.
func (s *sidebar) browserRow(gtx C, th *ui.Theme, r sessionRow, b blockView, act *sidebarAction, tree, active, dragEnded bool) D {
	session, web := r.sv.ID, r.sv.web
	tip, ok := s.tips[r.key]
	if !ok {
		tip = new(ui.Tooltip)
		s.tips[r.key] = tip
	}
	tooltip := th.TooltipCard(s.root, tip, browserTooltip(th, web))
	tooltip.Side = ui.SideRight
	entries := func() []ui.MenuEntry { return browserMenu(session, web, act) }
	menu := s.menuState(s.menus, r.key)
	more := s.menuState(s.moreMenus, r.key)
	hover := clickable(s.checkoutRows, r.key)
	// A press on the row's ⋯ is the row's too, and opening the menu of a
	// minimized browser must not show its page.
	if hover.Clicked(gtx) && !dragEnded && !clickable(s.moreClicks, r.key).Hovered() {
		act.browser = session
	}
	hovered := hover.Hovered() || more.Opened()
	p := r.place(tree)
	p.ground = nodeGround(th, b)
	line := func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		name := th.Text(ui.TextSM, web.label())
		if !web.shown {
			name = name.Muted()
		}
		row := append(treeLead(p),
			layout.Rigid(func(gtx C) D { return checkoutNode(gtx, onGround(th, p.ground), icBrowser, p.entry) }),
			ui.Gap(1.5),
			layout.Flexed(1, name.Layout),
			ui.Gap(1),
			layout.Rigid(func(gtx C) D {
				size := gtx.Dp(cardSlotSize)
				gtx.Constraints = layout.Exact(image.Pt(size, size))
				if !hovered {
					return D{Size: gtx.Constraints.Min}
				}
				return layout.Center.Layout(gtx, func(gtx C) D {
					return th.DropdownMenu(s.root, more, entries()).Layout(gtx, func(gtx C) D {
						return th.CornerButton(clickable(s.moreClicks, r.key), icGroupMore, more.Opened()).Layout(gtx)
					})
				})
			}),
		)
		return layout.Inset{Top: ui.Space(1.5), Bottom: ui.Space(1.5), Left: rowInset, Right: ui.Space(1)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, row...)
		})
	}
	return th.ContextMenuLazy(s.root, menu, entries).Layout(gtx, func(gtx C) D {
		return tooltip.Layout(gtx, func(gtx C) D {
			return hover.Layout(gtx, func(gtx C) D {
				return layout.Background{}.Layout(gtx,
					func(gtx C) D { return ui.Fill(gtx, cardLayer(th, "", active && web.focused, hovered), ui.RadiusMD) },
					func(gtx C) D { return withLinks(gtx, th, p.sessionRowLinks, line) },
				)
			})
		})
	})
}
