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
	parent   uint64
	display  *C.Display
	initOnce sync.Once
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

func hasParent() bool { return display != nil }

func embed(xid uint64, at image.Point) {
	C.lich_embed(display, C.ulong(parent), C.ulong(xid), C.int(at.X), C.int(at.Y))
}

func place(xid uint64, r image.Rectangle) {
	C.lich_embed_place(display, C.ulong(xid), C.int(r.Min.X), C.int(r.Min.Y), C.int(r.Dx()), C.int(r.Dy()))
}

// shape cuts the page with X SHAPE, bounding and input.
func shape(xid uint64, size image.Point, cuts []image.Rectangle) {
	rects := make([]C.XRectangle, len(cuts))
	for i, c := range cuts {
		rects[i] = C.XRectangle{x: C.short(c.Min.X), y: C.short(c.Min.Y), width: C.ushort(c.Dx()), height: C.ushort(c.Dy())}
	}
	var first *C.XRectangle
	if len(rects) > 0 {
		first = &rects[0]
	}
	C.lich_embed_shape(display, C.ulong(xid), C.int(size.X), C.int(size.Y), first, C.int(len(rects)))
}

func pointerTo(xid uint64, page bool) {
	on := C.int(0)
	if page {
		on = 1
	}
	C.lich_embed_pointer(display, C.ulong(xid), on)
}

func hide(xid uint64) { C.lich_embed_hide(display, C.ulong(xid)) }

// release reparents the page to the root, where lich-browser puts it off
// screen.
func release(xid uint64) { C.lich_embed_release(display, C.ulong(xid)) }

func keyboardToWindow() { C.lich_embed_keyboard(display, C.ulong(parent)) }

func paintGround(c color.NRGBA) {
	pixel := C.ulong(0xff)<<24 | C.ulong(c.R)<<16 | C.ulong(c.G)<<8 | C.ulong(c.B)
	C.lich_embed_ground(display, C.ulong(parent), pixel)
}
