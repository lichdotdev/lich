// Package webview shows lich-browser's pages in the window's own place
// (docs/lich-browser.md): on Linux each page is an X window of lich-browser's,
// which the window reparents into the one Gio draws into and places, cuts and
// shows over its pane, on Gio's own X connection. Everything is called from
// the window's goroutine.
package webview

/*
#cgo pkg-config: x11 xext
#include <stdlib.h>
#include "embed_linux.h"
*/
import "C"

import (
	"image"
	"image/color"
	"slices"
	"sync"

	"gioui.org/app"
)

// PrepareWindow opens the window on X11 where it can, XWayland included:
// only an X11 window can hold a page. Call it before the window opens.
func PrepareWindow() {
	app.PreferX11()
}

// parent is the X11 window Gio draws into, zero while there is none: on
// Wayland, or before the window exists; display is Gio's connection to it,
// which Gio opened thread-safe (XInitThreads).
var (
	parent  uint64
	display *C.Display
	// held are the pages in the window, handed back to the root before it
	// goes.
	held     []*Page
	initOnce sync.Once
	ground   color.NRGBA
)

// SetParent records the window pages go in, from the window's ViewEvent, and
// answers its X window, zero for none: the backend tells lich-browser. Gio
// sends the empty ViewEvent before it destroys its window, which would take
// the pages with it, so they go back to the root then.
func SetParent(e app.ViewEvent) uint64 {
	x, ok := e.(app.X11ViewEvent)
	if !ok || !x.Valid() {
		ReleaseAll()
		parent, display = 0, nil
		return 0
	}
	initOnce.Do(func() { C.lich_embed_init() })
	if parent != uint64(x.Window) {
		ReleaseAll()
	}
	parent, display = uint64(x.Window), (*C.Display)(x.Display)
	return parent
}

// ReleaseAll hands every page in the window back to the root.
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

// Embed is the page whose X window is xid, out of sight until it is placed.
func Embed(xid uint64) *Page {
	return &Page{xid: xid}
}

// XID is the page's X window.
func (p *Page) XID() uint64 { return p.xid }

// Place shows the page over r, in the window's pixels, left out of cuts, local
// to r, where the window draws over it; it says whether the cuts changed, since
// a cut shows the window's drawing only from its next frame. It moves the page
// only when r moved: a resize makes WebKit lay the page out again.
func (p *Page) Place(r image.Rectangle, cuts []image.Rectangle) bool {
	if display == nil {
		return false
	}
	if !p.embedded {
		C.lich_embed(display, C.ulong(parent), C.ulong(p.xid), C.int(r.Min.X), C.int(r.Min.Y))
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
		C.lich_embed_place(display, C.ulong(p.xid), C.int(r.Min.X), C.int(r.Min.Y), C.int(max(r.Dx(), 1)), C.int(max(r.Dy(), 1)))
	}
	p.placed, p.shown = r, true
	if cut {
		p.shape(r.Size(), cuts)
		p.cuts = slices.Clone(cuts)
	}
	return cut
}

func (p *Page) shape(size image.Point, cuts []image.Rectangle) {
	rects := make([]C.XRectangle, len(cuts))
	for i, c := range cuts {
		rects[i] = C.XRectangle{x: C.short(c.Min.X), y: C.short(c.Min.Y), width: C.ushort(c.Dx()), height: C.ushort(c.Dy())}
	}
	var first *C.XRectangle
	if len(rects) > 0 {
		first = &rects[0]
	}
	C.lich_embed_shape(display, C.ulong(p.xid), C.int(size.X), C.int(size.Y), first, C.int(len(rects)))
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

// applyPointer sets the page's input shape to pointerOff, once it is in the
// window.
func (p *Page) applyPointer() {
	if !p.embedded || display == nil {
		return
	}
	on := C.int(1)
	if p.pointerOff {
		on = 0
	}
	C.lich_embed_pointer(display, C.ulong(p.xid), on)
}

// Hide puts the page out of sight; it keeps running.
func (p *Page) Hide() {
	if !p.shown || display == nil {
		return
	}
	p.shown = false
	C.lich_embed_hide(display, C.ulong(p.xid))
}

// Release hands the page back to lich-browser, which puts it off screen.
func (p *Page) Release() {
	if !p.embedded {
		return
	}
	p.embedded, p.shown = false, false
	held = slices.DeleteFunc(held, func(h *Page) bool { return h == p })
	if display != nil {
		C.lich_embed_release(display, C.ulong(p.xid))
	}
}

// KeyboardToWindow gives the keyboard back to the window, from a page that
// took it with a click: the window calls it on every press it gets, since a
// press in a page never reaches it.
func KeyboardToWindow() {
	if len(held) == 0 || display == nil {
		return
	}
	C.lich_embed_keyboard(display, C.ulong(parent))
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
	if len(held) == 0 || display == nil {
		return
	}
	pixel := C.ulong(0xff)<<24 | C.ulong(ground.R)<<16 | C.ulong(ground.G)<<8 | C.ulong(ground.B)
	C.lich_embed_ground(display, C.ulong(parent), pixel)
}
