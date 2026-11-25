package main

import (
	"context"
	"errors"
	"image"
	"log"
	"slices"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/op/clip"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/internal/webview"
	"github.com/lichdotdev/lich/native/ui"
)

// frontXID is the X window of st's front tab in lich-browser, zero where the
// window shows it by screencast.
func frontXID(tabs []api.BrowserTab) uint64 {
	for _, t := range tabs {
		if t.Front {
			return t.XID
		}
	}
	return 0
}

// placeNativePage shows the session's front tab, lich-browser's X window,
// over the pane's page area, and says whether it does: a tab with no window
// is drawn by screencast. A tab no longer in front goes back to lich-browser.
func (u *window) placeNativePage(gtx C, p *browserPaneUI, st api.BrowserState) bool {
	xid := frontXID(st.Tabs)
	if p.native != nil && p.native.XID() != xid {
		p.native.Release()
		p.native = nil
	}
	if xid == 0 {
		return false
	}
	if p.native == nil {
		p.native = webview.Embed(xid)
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
			r := image.Rectangle{Min: at, Max: at.Add(p.nativeSize)}
			if p.native.Place(r, pageCuts(r, p.notes.cuts, u.covers)) {
				u.win.Invalidate()
			}
		}
	}
}

// coverPages marks r, in the window's pixels, as drawn over everything in the
// frame being laid out: a layer floating over the content (a dock or sidebar
// sliding in, a menu) calls it each frame it draws, and every page the window
// draws itself is cut there, so the layer shows over the page and the page
// keeps going around it.
func (u *window) coverPages(r image.Rectangle) {
	u.covers = append(u.covers, r)
}

// cutNativePages cuts each shown page, where it stands, before the frame is
// handed to the window: a cut made after it showed the window's background
// for a frame where a sliding layer had just moved onto the page, a band on 20
// of 288 frames over 4 slides, and none made before (Xvfb, 60 fps, 2026-11-19).
func (u *window) cutNativePages() {
	for _, b := range u.floatingBoxes() {
		u.coverPages(b)
	}
	for session, p := range u.browserPanes {
		if p.native == nil || !p.wasOnScreen {
			continue
		}
		if r := p.native.Placed(); !r.Empty() {
			p.native.Place(r, pageCuts(r, p.notes.cuts, u.covers))
		}
		o := pageOverlayOf(&p.notes)
		o.Cursor = p.pageCursor
		if script := overlayScript(o); script != p.overlaySent {
			p.overlaySent = script
			drawOverPage(session, script)
		}
	}
}

// pageCuts are a page's cuts at r: its own (local to it) and the window's
// covers (in the window's pixels) where they fall on it.
func pageCuts(r image.Rectangle, own, covers []image.Rectangle) []image.Rectangle {
	cuts := slices.Clone(own)
	for _, c := range covers {
		if on := c.Intersect(r); !on.Empty() {
			cuts = append(cuts, on.Sub(r.Min))
		}
	}
	return cuts
}

// floatingBoxes are where the collapsed sidebar and dock float over the
// stage this frame, and the toasts, dialogs, menus and tooltips over everything, in
// window pixels. A page is a window of the system's
// that nothing the window draws covers, so these are what a page under them
// has to cut out of itself (cutNativePages).
func (u *window) floatingBoxes() []image.Rectangle {
	var boxes []image.Rectangle
	for _, b := range []image.Rectangle{u.sidebar.peek.box, u.dock.peek.box} {
		if !b.Empty() {
			boxes = append(boxes, b)
		}
	}
	for _, covers := range [][]ui.Cover{u.toaster.Covers(), u.theme.Overlays.Boxes(), u.root.Popups()} {
		for _, c := range covers {
			boxes = append(boxes, c.Rects()...)
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
	p.native.Release()
	p.native = nil
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

// errNoPageRoute is the window asking lich-browser's page for what no
// backend RPC carries yet.
var errNoPageRoute = errors.New("lich-browser's page: the backend has no call for this yet")

// drawOverPage would hand the page the notes' and the agent's layer to draw
// over itself (pageOverlayScript).
//
// shortcut: no backend RPC runs a window's script in the page without it
// counting as the agent's eval, so nothing is drawn over a page lich-browser
// shows; a browser.Draw call in api brings it back.
func drawOverPage(string, string) {}

// shootPage is the page as it shows, without the layer the notes draw over
// it, for the sketch's picture.
//
// shortcut: no backend RPC hands the window a snapshot of the page, so a
// sketch over a page lich-browser shows cannot be sent; a browser.Snapshot
// call in api brings it back.
func shootPage(context.Context, string) (*image.RGBA, error) {
	return nil, errNoPageRoute
}

// nativeNotesInput takes, over a page the window draws itself, only what the
// notes need: the pointer while picking or drawing, which the page lets
// through then, and the notes' keys. The page takes everything else itself.
func (u *window) nativeNotesInput(gtx C, p *browserPaneUI, session string, scale float32) {
	tool := p.notes.picking || p.notes.drawing
	if tool {
		p.native.PointerToWindow()
		// Esc and the notes' other keys come to the pane when Select or Draw
		// turns on, the page having held the keyboard. Only then: taken every
		// frame, no other field in the window could keep it.
		if !p.notesTool && !p.notes.open && !gtx.Focused(&p.tag) {
			u.layoutStage.focus = browserPaneOf(session)
			gtx.Execute(key.FocusCmd{Tag: &p.tag})
		}
	} else {
		p.native.PointerToPage()
	}
	p.notesTool = tool
	filters := []event.Filter{
		pointer.Filter{Target: &p.tag, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel | pointer.Move | pointer.Leave | pointer.Scroll,
			ScrollX: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}, ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}},
		key.FocusFilter{Target: &p.tag},
		key.Filter{Focus: &p.tag, Name: key.NameEscape},
		key.Filter{Focus: &p.tag, Name: "Z", Required: key.ModShortcut},
		key.Filter{Focus: &p.tag, Name: "X", Required: key.ModShortcut | key.ModShift},
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
			switch {
			case e.Kind == pointer.Scroll && len(p.notes.strokes) > 0:
				// The strokes stand on what the pane shows, as the
				// screencast's do (browserPointer).
			case e.Kind == pointer.Scroll:
				u.scrollNativePage(gtx, p, session, x, y, e.Scroll.Div(scale))
			case p.notes.drawing:
				u.sketchPointer(gtx, p, session, x, y, e)
			case p.notes.picking:
				u.notesPointer(gtx, p, session, x, y, e)
			}
		}
	}
}

// scrollNativePage scrolls the page by d, CSS pixels, for the wheel turned
// while picking, which the page does not get then, and looks again under the
// pointer and for the noted elements.
func (u *window) scrollNativePage(gtx C, p *browserPaneUI, session string, x, y float64, d f32.Point) {
	p.notes.nextLocate, p.notes.followUntil = time.Time{}, gtx.Now.Add(noteFollowFor)
	p.notes.hover = api.BrowserElement{}
	m := api.BrowserMouseOptions{ID: session, Type: "wheel", X: x, Y: y, DX: float64(d.X), DY: float64(d.Y)}
	p.queue.push(browserCall{mouse: &m, send: func(ctx context.Context) error { return u.client.BrowserMouse(ctx, m) }})
	u.probeElement(p, session, x, y, false)
}

// windowWait bounds telling the backend the window's X window: the last one,
// zero, goes out as the window closes, and must not hold it open.
const windowWait = 2 * time.Second

// setBrowserWindow tells the backend the window's X window when it changed,
// so lich-browser sends the keys its pages get only for sitting under the
// pointer there. It waits for the answer: the zero that says the window is
// gone comes just before the process may end.
func (u *window) setBrowserWindow(xid uint64) {
	if xid == u.browserWindow {
		return
	}
	u.browserWindow = xid
	ctx, cancel := context.WithTimeout(u.ctx, windowWait)
	defer cancel()
	if err := u.client.SetBrowserWindow(ctx, xid); err != nil {
		log.Printf("browser window: %v", err)
	}
}
