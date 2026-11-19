package main

import (
	"image"

	"gioui.org/op"

	"github.com/lichdotdev/lich/native/internal/webview"
)

// placeNativePage shows the session's page in a web view the window holds
// itself, over the pane's page area, and says whether it does: where the
// system gives the window none, the pane draws the screencast.
func (u *window) placeNativePage(gtx C, p *browserPaneUI, url string) bool {
	if url == "" {
		url = "about:blank"
	}
	if p.native == nil {
		page, err := webview.Open(url)
		if err != nil {
			return false
		}
		p.native, p.nativeURL = page, url
	}
	if url != p.nativeURL {
		p.native.Navigate(url)
		p.nativeURL = url
	}
	// Origin is where the page area sat in the last frame: a move shows a
	// frame late, so a frame that moves the page asks for one more.
	origin, ok := gtx.Source.Origin(&p.tag)
	if !ok {
		gtx.Execute(op.InvalidateCmd{})
		return true
	}
	at := origin.Round()
	if p.native.Place(image.Rectangle{Min: at, Max: at.Add(gtx.Constraints.Max)}) {
		gtx.Execute(op.InvalidateCmd{})
	}
	return true
}

// putNativePageAway hides the page of a pane no longer drawn, and ends it
// once the session's browser is closed.
func (u *window) putNativePageAway(session string, p *browserPaneUI) {
	if st, ok := u.model.browserState(session); ok && st.Open {
		p.native.Hide()
		return
	}
	p.native.Close()
	p.native, p.nativeURL = nil, ""
}
