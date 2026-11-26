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

// On macOS a page is lich-browser's CAContext, shown in a CALayerHost over
// the pane and masked where the window draws over it. The layer takes no
// input of its own: the pane sends the page its pointer and keys over the
// wire, so the pointer and keyboard calls do nothing here.

// PrepareWindow leaves the window as it opens: any window holds a page here.
func PrepareWindow() {}

// view is the NSView Gio draws into, zero while there is none.
var view uintptr

// host is one page's CALayerHost, where it stands and what of it shows.
type host struct {
	c    *C.LichHost
	at   image.Rectangle
	cuts []image.Rectangle
}

var hosts = map[uint64]*host{}

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

func hasParent() bool { return view != 0 }

func embed(xid uint64, _ image.Point) {
	hosts[xid] = &host{c: C.lich_host_new(C.uint32_t(xid))}
}

func place(xid uint64, r image.Rectangle) {
	if h := hosts[xid]; h != nil {
		h.at = r
		h.show()
	}
}

func shape(xid uint64, _ image.Point, cuts []image.Rectangle) {
	if h := hosts[xid]; h != nil {
		h.cuts = slices.Clone(cuts)
		h.show()
	}
}

// show places the host at its rectangle, showing only what its cuts leave.
func (h *host) show() {
	shown := visible(image.Rectangle{Max: h.at.Size()}, h.cuts)
	rects := make([]C.LichRect, max(len(shown), 1))
	for i, s := range shown {
		rects[i] = cRect(s)
	}
	C.lich_host_place(h.c, C.uintptr_t(view), cRect(h.at), &rects[0], C.int(len(shown)))
}

func cRect(r image.Rectangle) C.LichRect {
	return C.LichRect{x: C.int(r.Min.X), y: C.int(r.Min.Y), width: C.int(r.Dx()), height: C.int(r.Dy())}
}

func pointerTo(uint64, bool) {}

func hide(xid uint64) {
	if h := hosts[xid]; h != nil {
		C.lich_host_hide(h.c)
	}
}

func release(xid uint64) {
	if h := hosts[xid]; h != nil {
		C.lich_host_free(h.c)
		delete(hosts, xid)
	}
}

func keyboardToWindow() {}

// paintGround does nothing: a layer leaves nothing behind it to paint.
func paintGround(color.NRGBA) {}
