//go:build linux || windows || darwin

// Package webview shows lich-browser's pages in the window's own place
// (docs/lich-browser.md): each page is a native surface of lich-browser's (an
// X window on Linux, an HWND on Windows, which the window makes a child of
// the one Gio draws into; on macOS a CAContext it shows in a CALayerHost) that
// the window places, cuts and shows over its pane. Everything is called from
// the window's goroutine.
package webview

import (
	"image"
	"image/color"
	"slices"
)

var (
	// held are the pages in the window, handed back to lich-browser before
	// it goes.
	held   []*Page
	ground color.NRGBA
)

// ReleaseAll hands every page in the window back to lich-browser.
func ReleaseAll() {
	for _, p := range slices.Clone(held) {
		p.Release()
	}
}

// Page is one of lich-browser's tab windows, as the window shows it.
type Page struct {
	xid      uint64
	embedded bool
	placed   image.Rectangle
	shown    bool
	cuts     []image.Rectangle
	// pointerOff: the pointer goes through the page to the window.
	pointerOff bool
}

// Embed is the page whose native window is xid, out of sight until it is
// placed.
func Embed(xid uint64) *Page {
	return &Page{xid: xid}
}

// XID is the page's native window.
func (p *Page) XID() uint64 { return p.xid }

// Place shows the page over r, in the window's pixels, left out of cuts, local
// to r, where the window draws over it; it says whether the cuts changed, since
// a cut shows the window's drawing only from its next frame. It moves the page
// only when r moved: a resize makes the engine lay the page out again.
func (p *Page) Place(r image.Rectangle, cuts []image.Rectangle) bool {
	if !hasParent() {
		return false
	}
	if !p.embedded {
		embed(p.xid, r.Min)
		p.embedded, p.shown, p.cuts = true, false, nil
		held = append(held, p)
		applyGround()
		p.applyPointer()
	}
	moved := !p.shown || r != p.placed
	cut := !slices.Equal(cuts, p.cuts) || r.Size() != p.placed.Size()
	if !moved && !cut {
		return false
	}
	if moved {
		place(p.xid, image.Rectangle{Min: r.Min, Max: r.Min.Add(image.Pt(max(r.Dx(), 1), max(r.Dy(), 1)))})
	}
	p.placed, p.shown = r, true
	if cut {
		shape(p.xid, r.Size(), cuts)
		p.cuts = slices.Clone(cuts)
	}
	return cut
}

// Placed is where the page was last placed, in the window's pixels.
func (p *Page) Placed() image.Rectangle {
	return p.placed
}

// PointerToWindow lets the pointer through the page to the window, which
// draws and answers over it while a tool of its own is on.
func (p *Page) PointerToWindow() { p.pointer(false) }

// PointerToPage gives the pointer back to the page.
func (p *Page) PointerToPage() { p.pointer(true) }

func (p *Page) pointer(toPage bool) {
	if p.pointerOff == !toPage {
		return
	}
	p.pointerOff = !toPage
	p.applyPointer()
}

// applyPointer sets whether the page takes the pointer, once it is in the
// window.
func (p *Page) applyPointer() {
	if !p.embedded || !hasParent() {
		return
	}
	pointerTo(p.xid, !p.pointerOff)
}

// Hide puts the page out of sight; it keeps running.
func (p *Page) Hide() {
	if !p.shown || !hasParent() {
		return
	}
	p.shown = false
	hide(p.xid)
}

// Release hands the page back to lich-browser, off screen.
func (p *Page) Release() {
	if !p.embedded {
		return
	}
	p.embedded, p.shown = false, false
	held = slices.DeleteFunc(held, func(h *Page) bool { return h == p })
	if hasParent() {
		release(p.xid)
	}
}

// KeyboardToWindow gives the keyboard back to the window, from a page that
// took it with a click: the window calls it on every press it gets, since a
// press in a page never reaches it.
func KeyboardToWindow() {
	if len(held) == 0 || !hasParent() {
		return
	}
	keyboardToWindow()
}

// SetGround is the color behind the pages, shown where a page leaves the
// window until the window's next frame: the pane's own background.
func SetGround(c color.NRGBA) {
	if c == ground {
		return
	}
	ground = c
	applyGround()
}

func applyGround() {
	if len(held) == 0 || !hasParent() {
		return
	}
	paintGround(ground)
}
