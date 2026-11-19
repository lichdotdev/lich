package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"log"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/op/clip"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/internal/webview"
	"github.com/lichdotdev/lich/pagescript"
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
	p.native.SetZoom(float64(u.pageScale(gtx)))
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
			if p.native.Place(image.Rectangle{Min: at, Max: at.Add(p.nativeSize)}, p.notes.cuts) {
				u.win.Invalidate()
			}
		}
	}
}

// floatingBoxes are where the collapsed sidebar and dock float over the
// stage this frame, in window pixels. A page is a window of the system's
// that nothing the window draws covers, so these are what a page under them
// has to cut out of itself, once internal/webview can (placeNativePages).
func (u *window) floatingBoxes() []image.Rectangle {
	var boxes []image.Rectangle
	for _, b := range []image.Rectangle{u.sidebar.peek.box, u.dock.peek.box} {
		if !b.Empty() {
			boxes = append(boxes, b)
		}
	}
	return boxes
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

// nativeEvalWait bounds a read of the page the window draws itself, as
// inputWait bounds one of the backend's.
const nativeEvalWait = 2 * time.Second

// elementAt is the element under x,y, CSS pixels, of the session's page: read
// in the page the window draws itself when there is one, where the person
// sees and scrolls it, and in the backend's otherwise.
func (u *window) elementAt(ctx context.Context, page *webview.Page, session string, x, y float64) (api.BrowserElement, error) {
	if page == nil {
		return u.client.BrowserElementAt(ctx, session, x, y)
	}
	ctx, cancel := context.WithTimeout(ctx, nativeEvalWait)
	defer cancel()
	out, err := page.Eval(ctx, pagescript.ElementAt(x, y))
	if err != nil {
		return api.BrowserElement{}, err
	}
	var el *api.BrowserElement
	if err := json.Unmarshal([]byte(out), &el); err != nil || el == nil {
		return api.BrowserElement{}, err
	}
	return *el, nil
}

// locateIn is where the elements selectors find stand, read where elementAt
// reads, first revealing reveal's element.
func (u *window) locateIn(ctx context.Context, page *webview.Page, session string, selectors []string, reveal string) ([]api.BrowserRect, error) {
	if page == nil {
		return u.client.BrowserLocate(ctx, session, selectors, reveal)
	}
	script, err := pagescript.Locate(selectors, reveal)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, nativeEvalWait)
	defer cancel()
	out, err := page.Eval(ctx, script)
	if err != nil {
		return nil, err
	}
	boxes := []api.BrowserRect{}
	return boxes, json.Unmarshal([]byte(out), &boxes)
}

// nativeNotesInput takes, over a page the window draws itself, only what the
// notes need: the pointer while picking, which the page lets through then,
// and the notes' keys. The page takes everything else itself.
func (u *window) nativeNotesInput(gtx C, p *browserPaneUI, session string, scale float32) {
	if p.notes.picking {
		p.native.PointerToWindow()
		// Esc and the pick's other keys come to the pane while it picks.
		if !p.notes.open && !gtx.Focused(&p.tag) {
			u.layoutStage.focus = browserPaneOf(session)
			gtx.Execute(key.FocusCmd{Tag: &p.tag})
		}
	} else {
		p.native.PointerToPage()
	}
	filters := []event.Filter{
		pointer.Filter{Target: &p.tag, Kinds: pointer.Press | pointer.Drag | pointer.Move | pointer.Leave | pointer.Scroll,
			ScrollX: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}, ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}},
		key.FocusFilter{Target: &p.tag},
		key.Filter{Focus: &p.tag, Name: key.NameEscape},
		key.Filter{Focus: &p.tag, Name: "E", Required: key.ModShortcut | key.ModShift},
		key.Filter{Focus: &p.tag, Name: key.NameReturn, Required: key.ModShortcut},
		key.Filter{Focus: &p.tag, Name: key.NameEnter, Required: key.ModShortcut},
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		switch e := ev.(type) {
		case key.Event:
			u.notesKey(p, session, e)
		case pointer.Event:
			x, y := float64(e.Position.X/scale), float64(e.Position.Y/scale)
			if e.Kind == pointer.Scroll {
				u.scrollNativePage(gtx, p, session, x, y, e.Scroll.Div(scale))
			} else if p.notes.picking {
				u.notesPointer(gtx, p, session, x, y, e)
			}
		}
	}
}

// scrollNativePage scrolls the page by d, CSS pixels, for the wheel turned
// while picking, which the page does not get then, and looks again under the
// pointer and for the noted elements.
func (u *window) scrollNativePage(gtx C, p *browserPaneUI, session string, x, y float64, d f32.Point) {
	page := p.native
	p.notes.nextLocate, p.notes.followUntil = time.Time{}, gtx.Now.Add(noteFollowFor)
	p.notes.hover = api.BrowserElement{}
	p.queue.push(browserCall{send: func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, nativeEvalWait)
		defer cancel()
		_, err := page.Eval(ctx, fmt.Sprintf("scrollBy(%g, %g)", d.X, d.Y))
		return err
	}})
	u.probeElement(p, session, x, y, false)
}
