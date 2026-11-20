package main

import (
	"context"
	"fmt"
	"image/color"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// pageURL is the address the pane shows for st's page: lich's new-tab page is a
// blank one, its own URL is the backend's and never the person's to read.
func pageURL(st api.BrowserState) string {
	if st.NewTab {
		return "about:blank"
	}
	return st.URL
}

// pageTitle is st's page title, none for the new-tab page.
func pageTitle(st api.BrowserState) string {
	if st.NewTab {
		return ""
	}
	return st.Title
}

// shownState is st as the pane shows it: a page the window draws itself
// (nativeURL, empty for none) that left lich's new-tab page is on its own page
// at once, while the backend's still waits for the agent's to load there,
// seconds in which the Select and Draw bar stayed away.
func shownState(st api.BrowserState, nativeURL string) api.BrowserState {
	if st.NewTab && nativeURL != "" && nativeURL != st.URL {
		st.NewTab, st.URL = false, nativeURL
	}
	return st
}

// notesOnPage is whether the Select and Draw bar is over st's page: not over
// DevTools, whose page is Chrome's own, nor over lich's new-tab page, which
// has nothing to note.
func notesOnPage(st api.BrowserState) bool {
	_, inspecting := devtoolsOf(st.Tabs)
	return !inspecting && !st.NewTab
}

// cssHex is the opaque c as the CSS hex color the new-tab page is drawn with.
func cssHex(c color.NRGBA) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// newTabLook is the new-tab page's theme and words as the window draws them:
// the page is the backend's, but only the window knows the theme it resolved
// (a followed system scheme, a theme's pair) and the interface language..
func newTabLook(th *ui.Theme) api.BrowserNewTabPageOptions {
	bg := th.Background
	// The page paints opaque colors on the background, as a translucent token
	// of a custom theme looks there.
	css := func(c color.NRGBA) string { return cssHex(ui.Over(bg, c)) }
	return api.BrowserNewTabPageOptions{
		Lang: string(i18n.Current()),
		Colors: api.BrowserNewTabColors{
			Background: cssHex(bg), Card: css(th.Card), Foreground: css(th.Foreground),
			Muted: css(th.MutedForeground), Border: css(th.Border),
			Accent: css(th.Accent), Pass: css(th.TonePass),
		},
		Text: api.BrowserNewTabText{
			Title:        i18n.T("layouts.browser.blankTab"),
			Address:      i18n.T("layouts.browser.newTabAddress"),
			AddressError: i18n.T("layouts.browser.newTabAddressError"),
			Running:      i18n.T("layouts.browser.newTabRunning"),
			Worktree:     i18n.T("layouts.browser.newTabWorktree"),
			Artifacts:    i18n.T("layouts.browser.newTabArtifacts"),
			Recent:       i18n.T("layouts.browser.newTabRecent"),
		},
	}
}

// syncNewTabPage hands the backend the page's look when the theme or the
// language changed since it last did, which a page opened after reads.
func (u *window) syncNewTabPage() {
	want := newTabLook(u.theme)
	if want == u.newTabLook {
		return
	}
	u.newTabLook = want
	u.browserViews.push(browserCall{merge: "newtab", send: func(ctx context.Context) error {
		return u.client.SetBrowserNewTabPage(ctx, want)
	}})
}
