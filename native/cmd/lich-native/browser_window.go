package main

import (
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// The window's side of a session browser: the card's Browser item, and the
// toast that offers a browser the agent opened.

// applyBrowser opens the browser of the card the menu was opened on.
func (u *window) applyBrowser(act sidebarAction) {
	if act.browser != "" {
		u.openBrowser(act.browser)
	}
}

// openBrowser puts session's browser on screen: in its tab, asked about
// first when the session sits in another layout, or, for a browser moved to
// its own window, by bringing that window forward.
func (u *window) openBrowser(session string) {
	if st, ok := u.model.browserState(session); ok && st.Open && st.Headed {
		go func() {
			if err := u.client.ShowBrowser(u.ctx, session); err != nil {
				u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("sidebar.sessionCard.browserFailed", i18n.Args{"error": backend.ErrorText(err)})})
			}
		}()
		return
	}
	if from := u.model.layoutOf(session); from != "" && u.model.browserTabOf(session) == "" {
		u.askMoveToBrowser(session, from)
		return
	}
	u.model.openBrowserTab(u.ctx, session, false)
	u.model.invalidate()
}

// offerBrowser tells the person the agent of a session opened a browser,
// with Open to put it on screen.
func (u *window) offerBrowser(st api.BrowserState) {
	label := u.model.sessionLabel(st.Owner)
	if label == "" {
		return
	}
	session := st.Owner
	u.duplicates.Show(ui.Toast{
		ID:          "browser:" + session,
		Title:       i18n.T("layouts.browser.opened", i18n.Args{"label": label}),
		Description: displayURL(st.URL),
		Icon:        icBrowser,
		Cancel:      &ui.ToastButton{Label: i18n.T("layouts.browser.dismiss")},
		Action:      &ui.ToastButton{Label: i18n.T("layouts.browser.open"), OnClick: func() { u.openBrowser(session) }},
	})
}

// browserTabOf is the id of session's browser tab, "" for none.
func (m *model) browserTabOf(session string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if i := m.layouts.browserTab(session); i >= 0 {
		return m.layouts.tabs[i].ID
	}
	return ""
}
