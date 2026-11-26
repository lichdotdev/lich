// Package webview shows lich-browser's pages in the window's own place
// (docs/lich-browser.md): on macOS each page is a CAContext of lich-browser's,
// which the window shows in a CALayerHost over its pane, placed, cut and
// hidden as the pane needs. The page takes no input there: the pane sends it
// over the wire. Everything is called from the window's goroutine.
package webview

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit -framework QuartzCore
#include "host_darwin.h"
*/
import "C"

import (
	"image"
	"image/color"
	"slices"

	"gioui.org/app"
)

// PrepareWindow leaves the window as it opens: any window holds a page here.
func PrepareWindow() {}

var (
	// view is the NSView Gio draws into, zero while there is none.
	view uintptr
	// held are the pages in the window.
	held []*Page
)

// SetParent records the view pages go over, from the window's ViewEvent, and
// answers its window's number, zero for none: the backend tells lich-browser.
func SetParent(e app.ViewEvent) uint64 {
	v, ok := e.(app.AppKitViewEvent)
	if !ok || !v.Valid() {
		ReleaseAll()
		view = 0
		return 0
	}
	if view != v.View {
		ReleaseAll()
	}
	view = v.View
	return uint64(C.lich_host_window(C.uintptr_t(view)))
}

// ReleaseAll takes every page out of the window.
func ReleaseAll() {
	for _, p := range slices.Clone(held) {
		p.Release()
	}
}

// SetGround does nothing: a layer leaves nothing behind it to paint.
func SetGround(color.NRGBA) {}

// KeyboardToWindow does nothing: the window keeps the keyboard here.
func KeyboardToWindow() {}

// Page is one of lich-browser's tab layers, as the window shows it.
type Page struct {
	xid    uint64
	host   *C.LichHost
	placed image.Rectangle
	shown  bool
	cuts   []image.Rectangle
}

// Embed is the page whose CAContext is xid, out of sight until it is placed.
func Embed(xid uint64) *Page {
	return &Page{xid: xid}
}

// XID is the page's CAContext id.
func (p *Page) XID() uint64 { return p.xid }

// Place shows the page over r, in the window's pixels, left out of cuts, local
// to r, where the window draws over it; it says whether the cuts changed.
func (p *Page) Place(r image.Rectangle, cuts []image.Rectangle) bool {
	if view == 0 {
		return false
	}
	if p.host == nil {
		p.host = C.lich_host_new(C.uint32_t(p.xid))
		held = append(held, p)
	}
	cut := !slices.Equal(cuts, p.cuts) || r.Size() != p.placed.Size()
	if p.shown && r == p.placed && !cut {
		return false
	}
	shown := visible(image.Rectangle{Max: r.Size()}, cuts)
	rects := make([]C.LichRect, max(len(shown), 1))
	for i, s := range shown {
		rects[i] = cRect(s)
	}
	C.lich_host_place(p.host, C.uintptr_t(view), cRect(r), &rects[0], C.int(len(shown)))
	p.placed, p.shown, p.cuts = r, true, slices.Clone(cuts)
	return cut
}

func cRect(r image.Rectangle) C.LichRect {
	return C.LichRect{x: C.int(r.Min.X), y: C.int(r.Min.Y), width: C.int(r.Dx()), height: C.int(r.Dy())}
}

// Placed is where the page was last placed, in the window's pixels.
func (p *Page) Placed() image.Rectangle {
	return p.placed
}

// PointerToWindow and PointerToPage do nothing: the pane has the pointer
// here, and hands it to the page over the wire.
func (p *Page) PointerToWindow() {}

func (p *Page) PointerToPage() {}

// Hide puts the page out of sight; it keeps running.
func (p *Page) Hide() {
	if !p.shown || p.host == nil {
		return
	}
	p.shown = false
	C.lich_host_hide(p.host)
}

// Release takes the page out of the window; lich-browser keeps it.
func (p *Page) Release() {
	if p.host == nil {
		return
	}
	held = slices.DeleteFunc(held, func(h *Page) bool { return h == p })
	C.lich_host_free(p.host)
	p.host, p.shown, p.cuts = nil, false, nil
}
