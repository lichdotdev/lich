package main

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/op/clip"

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
