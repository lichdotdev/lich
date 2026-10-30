package main

import (
	"context"
	"path/filepath"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// The window's side of a session browser: the card's Browser item, the toast
// that offers a browser the agent opened, and the one that names a download.

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

// askIdleBrowser asks whether to close session's browser, which nobody used
// for the backend's idle window, or takes the question down once it is no
// longer idle. It stays up until answered: swiping it away keeps the browser,
// since only Close closes it.
func (u *window) askIdleBrowser(st api.BrowserState) {
	id := idleBrowserToastID(st.Owner)
	if !st.Open || !st.Idle {
		u.duplicates.Dismiss(id)
		return
	}
	label, project := u.model.sessionPlace(st.Owner)
	if label == "" {
		return
	}
	u.duplicates.Show(idleBrowserToast(st, label, project, func() { u.answerIdleBrowser(st.Owner, u.client.KeepBrowser) },
		func() { u.answerIdleBrowser(st.Owner, u.client.CloseBrowser) }))
}

func idleBrowserToastID(session string) string { return "browser-idle:" + session }

func idleBrowserToast(st api.BrowserState, label, project string, keep, closeIt func()) ui.Toast {
	t := ui.Toast{
		ID:        idleBrowserToastID(st.Owner),
		Title:     i18n.T("layouts.browser.idle", i18n.Args{"label": label}),
		Icon:      icBrowser,
		Duration:  ui.ToastForever,
		Cancel:    &ui.ToastButton{Label: i18n.T("layouts.browser.keepOpen"), OnClick: keep},
		Action:    &ui.ToastButton{Label: i18n.T("layouts.browser.closeIdle"), OnClick: closeIt},
		OnDismiss: keep,
	}
	if url := displayURL(st.URL); url != "" {
		t.Lines = append(t.Lines, ui.ToastLine{Text: url})
	}
	if project != "" {
		t.Lines = append(t.Lines, ui.ToastLine{Icon: icFolder, Text: project})
	}
	return t
}

// answerIdleBrowser sends the person's answer about session's idle browser,
// KeepBrowser or CloseBrowser. A failure is logged: the browser it answers
// about is gone, or the next idle window asks again.
func (u *window) answerIdleBrowser(session string, answer func(context.Context, string) error) {
	go func() { u.logErr("answer about an idle browser", answer(u.ctx, session)) }()
}

// tellDownload names a file a session's page downloaded and the folder it
// went to, which the toast's button opens.
func (u *window) tellDownload(d api.BrowserDownload) {
	u.toaster.Show(downloadToast(d, func(dir string) {
		go func() {
			if err := u.client.OpenFolder(u.ctx, dir); err != nil {
				u.toaster.Show(ui.Toast{Variant: ui.ToastError, Title: i18n.T("sidebar.sessionCard.openFolderFailed", i18n.Args{"error": backend.ErrorText(err)})})
			}
		}()
	}))
}

func downloadToast(d api.BrowserDownload, openFolder func(dir string)) ui.Toast {
	dir := filepath.Dir(d.Path)
	return ui.Toast{
		Title:  i18n.T("layouts.browser.downloaded", i18n.Args{"name": filepath.Base(d.Path)}),
		Icon:   icDownload,
		Lines:  []ui.ToastLine{{Icon: icFolder, Text: dir}},
		Action: &ui.ToastButton{Label: i18n.T("layouts.browser.showFolder"), OnClick: func() { openFolder(dir) }},
	}
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
