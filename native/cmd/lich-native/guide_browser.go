package main

import driver "github.com/lichdotdev/gio-driver"

const browserGuideID = "browser"

// The widgets the Browser guide points at.
const (
	tourBrowserPane  = "browser.pane"
	tourBrowserPick  = "browser.pick"
	tourBrowserPage  = "browser.page"
	tourBrowserNotes = "browser.notes"
)

var browserTour = []tourStep{
	{anchor: tourSession, side: driver.Right, until: (*window).tourBrowserOpen,
		text: tourText{"tourBrowser.open.title", "tourBrowser.open.body", "tourBrowser.open.hint"}},
	{anchor: tourBrowserPane, side: driver.Left, text: tourText{"tourBrowser.pane.title", "tourBrowser.pane.body", ""}},
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

// tourBrowserOpen counts a browser opened in its own window too, which never
// gets a tab: the steps after it then center.
func (u *window) tourBrowserOpen() bool {
	id := u.model.shownSession()
	if id == "" {
		return false
	}
	st, ok := u.model.browserState(id)
	return u.model.browserTabOf(id) != "" || ok && st.Open
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
