package main

import (
	"context"
	"image"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var icTabNew = icons.Lucide("plus")

// A browser's tabs on its pane's header: with one tab, the pane's name and a
// new-tab button; with more, the tabs take the name's place.

// Tab sizes: a tab shrinks to share the header, down to browserTabMin, and never
// grows past browserTabMax.
var (
	browserTabMax    = ui.Space(50)
	browserTabMin    = ui.Space(16)
	browserTabHeight = ui.Space(6)
)

// browserTabsUI is one pane's tab widgets, by Chrome's tab id, and where each
// was last drawn, in the header's coordinates, for the agent's cursor.
type browserTabsUI struct {
	pick, close map[string]*widget.Clickable
	newTab      iconActionState
	closeTip    map[string]*ui.Tooltip
	at, closeAt map[string]image.Rectangle
	newAt       image.Rectangle
}

func (t *browserTabsUI) widgets(id string) (pick, closer *widget.Clickable, tip *ui.Tooltip) {
	if t.pick == nil {
		t.pick, t.close, t.closeTip = map[string]*widget.Clickable{}, map[string]*widget.Clickable{}, map[string]*ui.Tooltip{}
		t.at, t.closeAt = map[string]image.Rectangle{}, map[string]image.Rectangle{}
	}
	if t.pick[id] == nil {
		t.pick[id], t.close[id], t.closeTip[id] = new(widget.Clickable), new(widget.Clickable), new(ui.Tooltip)
	}
	return t.pick[id], t.close[id], t.closeTip[id]
}

// tabsOrigin is where the tabs start on the header: past its inset, the
// browser icon and the gap after it, as browserHeader lays them out.
func tabsOrigin(gtx C) int {
	return gtx.Dp(ui.Space(2.5)) + gtx.Dp(ui.Space(3.5)) + gtx.Dp(ui.Space(2))
}

// browserTabs is the header's middle: the tabs, or the pane's name, then the
// new-tab button. It handles their clicks first.
func (u *window) browserTabs(gtx C, p *browserPaneUI, session string, st api.BrowserState, focused bool) D {
	u.tabClicks(gtx, p, session, st.Tabs)
	if len(st.Tabs) < 2 {
		return u.paneNameAndNewTab(gtx, p, focused)
	}
	th := u.theme
	t := &p.tabs
	plus := gtx.Dp(ui.Space(6))
	gap := gtx.Dp(ui.Space(0.5))
	w := (gtx.Constraints.Max.X - plus - gap*len(st.Tabs)) / len(st.Tabs)
	w = min(max(w, gtx.Dp(browserTabMin)), gtx.Dp(browserTabMax))
	h := gtx.Dp(browserTabHeight)
	y := (gtx.Constraints.Max.Y - h) / 2
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	x := 0
	for _, tab := range st.Tabs {
		pick, closer, tip := t.widgets(tab.ID)
		t.at[tab.ID] = image.Rect(x, y, x+w, y+h)
		off := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		tgtx := gtx
		tgtx.Constraints = layout.Exact(image.Pt(w, h))
		closeAt := u.tabLayout(tgtx, p, tab, pick, closer, tip)
		t.closeAt[tab.ID] = closeAt.Add(image.Pt(x, y))
		off.Pop()
		x += w + gap
	}
	t.newAt = image.Rect(x, y, x+plus, y+plus)
	off := op.Offset(image.Pt(x, (gtx.Constraints.Max.Y-plus)/2)).Push(gtx.Ops)
	iconAction(gtx, th, &u.root, &t.newTab, icTabNew, i18n.T("layouts.browser.newTab"), p.tabFlash(gtx, "new"))
	off.Pop()
	return D{Size: gtx.Constraints.Max}
}

// paneNameAndNewTab is the header's middle with one tab: the pane's name, as
// before tabs, and the new-tab button after it.
func (u *window) paneNameAndNewTab(gtx C, p *browserPaneUI, focused bool) D {
	th := u.theme
	fg, weight := th.MutedForeground, font.Normal
	if focused {
		fg, weight = th.Foreground, font.Medium
	}
	name := th.Text(ui.TextXS, i18n.T("layouts.browser.title")).In(fg).Weight(weight)
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			d := name.Layout(gtx)
			side, x := gtx.Dp(ui.Space(6)), d.Size.X+gtx.Dp(ui.Space(1))
			y := (gtx.Constraints.Max.Y - side) / 2
			p.tabs.newAt = image.Rect(x, y, x+side, y+side)
			return d
		}),
		ui.Gap(1),
		layout.Rigid(func(gtx C) D {
			return iconAction(gtx, th, &u.root, &p.tabs.newTab, icTabNew, i18n.T("layouts.browser.newTab"), p.tabFlash(gtx, "new"))
		}),
	)
}

// tabLayout draws one tab over gtx's exact size and returns its close
// button's box, in the tab's coordinates.
func (u *window) tabLayout(gtx C, p *browserPaneUI, tab api.BrowserTab, pick, closer *widget.Clickable, tip *ui.Tooltip) image.Rectangle {
	th := u.theme
	size := gtx.Constraints.Max
	fg := th.MutedForeground
	switch {
	case tab.Front || p.tabFlash(gtx, tab.ID):
		ui.Fill(gtx, th.Accent, ui.RadiusMD)
		fg = th.Foreground
	case pick.Hovered():
		ui.Fill(gtx, th.Over(ui.Alpha(th.Accent, 0.6)), ui.RadiusMD)
	}
	pick.Layout(gtx, func(gtx C) D { return D{Size: size} })
	side := gtx.Dp(16)
	right := gtx.Dp(ui.Space(1.5))
	box := image.Rect(size.X-right-side, (size.Y-side)/2, size.X-right, (size.Y+side)/2)
	label := gtx
	label.Constraints = layout.Exact(image.Pt(max(box.Min.X-gtx.Dp(ui.Space(2.5))-gtx.Dp(ui.Space(1)), 0), size.Y))
	off := op.Offset(image.Pt(gtx.Dp(ui.Space(2.5)), 0)).Push(gtx.Ops)
	layout.W.Layout(label, th.Text(ui.TextXS, tabLabel(tab)).In(fg).Layout)
	off.Pop()
	off = op.Offset(box.Min).Push(gtx.Ops)
	closeTip := th.Tooltip(&u.root, tip, i18n.T("layouts.browser.closeTab"))
	closeTip.Layout(gtx, func(gtx C) D { return cornerButton(gtx, th, closer, icTabClose, p.tabFlash(gtx, "close "+tab.ID)) })
	off.Pop()
	return box
}

// tabLabel names a tab by its title, else its address, else as a blank one.
func tabLabel(tab api.BrowserTab) string {
	switch {
	case tab.Title != "":
		return tab.Title
	case tab.URL != "" && tab.URL != "about:blank":
		return displayURL(tab.URL)
	}
	return i18n.T("layouts.browser.blankTab")
}

// tabClicks runs the tab clicks of the last frame. A new tab puts the focus
// in the address field, to type where it goes.
func (u *window) tabClicks(gtx C, p *browserPaneUI, session string, tabs []api.BrowserTab) {
	nav := func(action, tab string) {
		u.sendBrowser(session, "tab", func(ctx context.Context) error { return u.client.BrowserTabNav(ctx, session, action, tab) })
	}
	if p.tabs.newTab.click.Clicked(gtx) {
		nav(api.TabNew, "")
		gtx.Execute(key.FocusCmd{Tag: &p.url})
	}
	for _, tab := range tabs {
		pick, closer, _ := p.tabs.widgets(tab.ID)
		if closer.Clicked(gtx) {
			nav(api.TabClose, tab.ID)
			continue
		}
		if pick.Clicked(gtx) && !tab.Front {
			nav(api.TabSelect, tab.ID)
		}
	}
}

// headerActions are the agent's actions the cursor plays on the pane's
// header's tabs.
var headerActions = map[string]bool{"tab-new": true, "tab-select": true, "tab-close": true}

// tabPoint is where on the header the cursor plays a tab action: the middle
// of the new-tab button, of the tab, or of its close button, where they were
// last drawn. A tab already gone keeps where it was.
func tabPoint(gtx C, p *browserPaneUI, m browserMark, head int) f32.Point {
	origin := image.Pt(tabsOrigin(gtx), 0)
	r := p.tabs.newAt
	switch m.action {
	case "tab-select":
		r = p.tabs.at[m.tab]
	case "tab-close":
		r = p.tabs.closeAt[m.tab]
	}
	if r.Empty() {
		return f32.Pt(float32(origin.X), float32(head/2))
	}
	c := r.Add(origin).Min.Add(r.Size().Div(2))
	return f32.Pt(float32(c.X), float32(c.Y))
}

// tabFlash reports whether the agent's cursor lit what: "new", a tab's id,
// or "close " and its id.
func (p *browserPaneUI) tabFlash(gtx C, what string) bool {
	return p.flashing(gtx, "tab:"+what)
}
