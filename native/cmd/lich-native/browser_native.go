package main

import (
	"image"
	"log"

	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/op/clip"

	"github.com/lichdotdev/lich/native/internal/webview"
)

// placeNativePage shows the session's page in a web view the window holds
// itself, over the pane's page area, and says whether it does: where the
// system gives the window none, the pane draws the screencast.
func (u *window) placeNativePage(gtx C, p *browserPaneUI, session, url string) bool {
	if url == "" {
		url = "about:blank"
	}
	if p.native == nil {
		page, err := webview.Open(url, func(l webview.Load) { go u.nativeLoaded(session, l) })
		if err != nil {
			return false
		}
		p.native, p.nativeURL = page, url
	}
	// A page the person took elsewhere is followed by the backend, whose
	// URL then arrives where the page already is.
	if url != p.nativeURL {
		p.nativeURL = url
		if url != p.native.URL() {
			p.native.Navigate(url)
		}
	}
	p.nativeSize = gtx.Constraints.Max
	webview.SetGround(u.theme.Background)
	return true
}

// placeNativePages moves each shown page over its pane's page area where the
// frame just handed to the window drew it: placed during layout from the
// frame before, a page trailed a sliding pane by a frame.
func (u *window) placeNativePages(src input.Source) {
	for _, p := range u.browserPanes {
		if p.native == nil || !p.wasOnScreen {
			continue
		}
		if origin, ok := src.Origin(&p.tag); ok {
			at := origin.Round()
			p.native.Place(image.Rectangle{Min: at, Max: at.Add(p.nativeSize)})
		}
	}
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

// windowPress is the tag of the area over the whole window that hears every
// press the window gets.
var windowPress = new(int)

// keyboardFromPages takes the keyboard back from a page that holds it when
// the person presses anywhere in the window: a press in a page never reaches
// the window, so any press it hears is outside them.
func keyboardFromPages(gtx C) {
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: windowPress, Kinds: pointer.Press}); !ok {
			break
		}
		webview.KeyboardToWindow()
	}
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	defer pointer.PassOp{}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, windowPress)
}

// nativeLoaded tells the backend about a load of session's own page, which
// shows it on the tab and follows a page the person took elsewhere.
func (u *window) nativeLoaded(session string, l webview.Load) {
	if err := u.client.NativeLoadBrowser(u.ctx, session, l.Loading, l.URL); err != nil {
		log.Printf("browser of %s: page load: %v", session, err)
	}
}
