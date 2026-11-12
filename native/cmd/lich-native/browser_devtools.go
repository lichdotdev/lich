package main

import (
	"context"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icDevTools       = icons.Lucide("code-xml")
	icDevToolsPaused = icons.Lucide("pause")
)

// devtoolsOf is what the pane holds of DevTools for the tab in front: open,
// that tab has a DevTools tab; shown, the pane draws it in the page's place.
func devtoolsOf(tabs []api.BrowserTab) (open, shown bool) {
	for i, t := range tabs {
		if !t.Front {
			continue
		}
		if t.DevTools {
			return true, true
		}
		return i+1 < len(tabs) && tabs[i+1].DevTools, false
	}
	return false, false
}

// openDevTools opens or selects the DevTools of session's front tab.
func (u *window) openDevTools(session string) {
	u.sendBrowser(session, "devtools", func(ctx context.Context) error { return u.client.BrowserDevTools(ctx, session) })
}
