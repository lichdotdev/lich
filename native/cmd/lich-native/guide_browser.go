package main

import (
	"gioui.org/layout"
	driver "github.com/lichdotdev/gio-driver"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

const browserGuideID = "browser"

// The widgets the Browser guide points at.
const (
	tourBrowserItem  = "browser.item"
	tourBrowserPane  = "browser.pane"
	tourBrowserAddr  = "browser.address"
	tourBrowserPick  = "browser.pick"
	tourBrowserPage  = "browser.page"
	tourBrowserNotes = "browser.notes"
)

var browserTour = []tourStep{
	{anchor: tourSession, side: driver.Right, until: (*window).tourCardMenuOpen,
		text: tourText{"tourBrowser.open.title", "tourBrowser.open.body", "tourBrowser.open.hint"}},
	{anchor: tourBrowserItem, side: driver.Right, layer: 1, until: (*window).tourBrowserOpen,
		text: tourText{"tourBrowser.item.title", "tourBrowser.item.body", "tourBrowser.item.hint"}},
	{anchor: tourBrowserPane, side: driver.Left, text: tourText{"tourBrowser.pane.title", "tourBrowser.pane.body", ""}},
	// A new browser shows no page, and notes need one.
	{anchor: tourBrowserAddr, side: driver.Bottom, until: (*window).tourBrowserHasPage,
		text: tourText{"tourBrowser.address.title", "tourBrowser.address.body", "tourBrowser.address.hint"}},
	{anchor: tourBrowserPick, side: driver.Bottom, until: (*window).tourBrowserPicking,
		text: tourText{"tourBrowser.pick.title", "tourBrowser.pick.body", "tourBrowser.pick.hint"}},
	{anchor: tourBrowserPage, side: driver.Left, until: (*window).tourBrowserNoted,
		text: tourText{"tourBrowser.note.title", "tourBrowser.note.body", "tourBrowser.note.hint"}},
	{anchor: tourBrowserNotes, side: driver.Top, text: tourText{"tourBrowser.send.title", "tourBrowser.send.body", ""}},
}

func init() {
	registerGuide(&guide{id: browserGuideID, order: 50, label: "tourBrowser.label", steps: browserTour,
		ready: (*window).tourSessionOpen, unavailable: "tourBrowser.unavailable"})
}

// tourCardMenuOpen is whether the shown session's card menu is open, from a
// right click or its corner button.
func (u *window) tourCardMenuOpen() bool {
	id := u.model.shownSession()
	for _, m := range []*ui.MenuState{u.sidebar.menus[id], u.sidebar.moreMenus[id]} {
		if m != nil && m.Opened() {
			return true
		}
	}
	return false
}

// tourBrowserSpot makes the Browser entry of a card menu the tour's anchor.
func tourBrowserSpot(entries []ui.MenuEntry, tour *driver.Tour) {
	label := i18n.T("sidebar.sessionCard.browser")
	for i := range entries {
		if entries[i].Label == label {
			entries[i].Spot = func(gtx C, row layout.Widget) D { return tour.Anchor(gtx, tourBrowserItem, row) }
		}
	}
}

// tourBrowserOpen counts a browser opened in its own window too, which has
// no pane: the steps after it then center.
func (u *window) tourBrowserOpen() bool {
	id := u.model.shownSession()
	if id == "" {
		return false
	}
	st, ok := u.model.browserState(id)
	return u.model.pageShown(id) || ok && st.Open
}

// tourBrowserHasPage is whether the shown session's browser shows a page,
// not the blank one it opens on.
func (u *window) tourBrowserHasPage() bool {
	st, ok := u.model.browserState(u.model.shownSession())
	return ok && displayURL(st.URL) != ""
}

// tourBrowserNotes is the active session's notes, nil before its pane drew;
// it reads the map, since browserPane would start a pane's queue.
func (u *window) tourBrowserNotes() *browserNotes {
	if p, ok := u.browserPanes[u.model.shownSession()]; ok {
		return &p.notes
	}
	return nil
}

func (u *window) tourBrowserPicking() bool {
	n := u.tourBrowserNotes()
	return n != nil && (n.picking || len(n.list) > 0)
}

func (u *window) tourBrowserNoted() bool {
	n := u.tourBrowserNotes()
	return n != nil && len(n.list) > 0
}
