// Package webview shows a web page in the window's own place: on Linux a
// WebKitGTK view in an X11 window made a child of the window Gio draws into.
// GTK runs on a thread of its own, started with the first page, and every
// call into a page goes to it in the order it was made.
package webview

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1 x11 gbm
#include <stdlib.h>
#include "webview_linux.h"
*/
import "C"

import (
	"errors"
	"image"
	"image/color"
	"runtime"
	"runtime/cgo"
	"sync"
	"sync/atomic"
	"unsafe"

	"gioui.org/app"
)

var (
	errNoWindow = errors.New("webview: the window is not an X11 window")
	errNoGTK    = errors.New("webview: GTK cannot open the X11 display")
)

// PrepareWindow opens the window on X11 where it can, XWayland included:
// only an X11 window can hold a page. Call it before the window opens.
func PrepareWindow() {
	app.PreferX11()
}

// parent is the X11 window Gio draws into, zero while there is none: on
// Wayland, or before the window exists.
var parent atomic.Uint64

// SetParent records the window pages go in, from the window's ViewEvent.
func SetParent(e app.ViewEvent) {
	x, ok := e.(app.X11ViewEvent)
	if !ok || !x.Valid() {
		parent.Store(0)
		return
	}
	parent.Store(uint64(x.Window))
}

var (
	gtkOnce sync.Once
	gtkErr  error
	// gtkUp is whether GTK runs: calls wait for its thread to take them.
	gtkUp atomic.Bool
)

func startGTK() error {
	gtkOnce.Do(func() {
		started := make(chan error, 1)
		go func() {
			runtime.LockOSThread()
			if C.lich_webview_init() == 0 {
				started <- errNoGTK
				return
			}
			started <- nil
			C.lich_webview_main()
		}()
		gtkErr = <-started
		gtkUp.Store(gtkErr == nil)
	})
	return gtkErr
}

func onGTK(work func()) {
	C.lich_webview_invoke(C.uintptr_t(cgo.NewHandle(work)))
}

//export lichWebviewRun
func lichWebviewRun(handle C.uintptr_t) {
	h := cgo.Handle(handle)
	work := h.Value().(func())
	h.Delete()
	work()
}

// Page is one web page. Its methods are called from the window's goroutine.
type Page struct {
	// page is touched only on the GTK thread.
	page   *C.LichPage
	placed image.Rectangle
	shown  bool
	loads  loads
	// onLoad hears the page's loads, on the GTK thread.
	onLoad func(Load)
	self   cgo.Handle
}

// Open makes a page showing url, out of sight until it is placed. onLoad
// hears its main frame's loads, on a thread of its own.
func Open(url string, onLoad func(Load)) (*Page, error) {
	into := parent.Load()
	if into == 0 {
		return nil, errNoWindow
	}
	if err := startGTK(); err != nil {
		return nil, err
	}
	p := &Page{onLoad: onLoad}
	p.self = cgo.NewHandle(p)
	p.loads.ask(url)
	onGTK(func() {
		u := C.CString(url)
		defer C.free(unsafe.Pointer(u))
		p.page = C.lich_webview_open(C.ulong(into), u, C.uintptr_t(p.self))
	})
	return p, nil
}

//export lichWebviewLoad
func lichWebviewLoad(owner C.uintptr_t, step C.int, uri *C.char) {
	p := cgo.Handle(owner).Value().(*Page)
	if l, ok := p.loads.step(int(step), C.GoString(uri)); ok {
		p.onLoad(l)
	}
}

// URL is where the page is: the last URL the window asked for, or where a
// load the page started itself landed.
func (p *Page) URL() string {
	return p.loads.url()
}

// Place shows the page over r, in the window's pixels, and says whether that
// moved it.
func (p *Page) Place(r image.Rectangle) bool {
	if p.shown && r == p.placed {
		return false
	}
	p.placed, p.shown = r, true
	onGTK(func() {
		C.lich_webview_place(p.page, C.int(r.Min.X), C.int(r.Min.Y), C.int(max(r.Dx(), 1)), C.int(max(r.Dy(), 1)))
	})
	return true
}

// Hide puts the page out of sight; it keeps running.
func (p *Page) Hide() {
	if !p.shown {
		return
	}
	p.shown = false
	onGTK(func() { C.lich_webview_hide(p.page) })
}

// Navigate loads url in the page.
func (p *Page) Navigate(url string) {
	p.loads.ask(url)
	onGTK(func() {
		u := C.CString(url)
		defer C.free(unsafe.Pointer(u))
		C.lich_webview_navigate(p.page, u)
	})
}

// Close ends the page.
func (p *Page) Close() {
	onGTK(func() {
		C.lich_webview_close(p.page)
		p.self.Delete()
	})
}

// KeyboardToWindow gives the keyboard back to the window, from a page that
// took it with a click: the window calls it on every press it gets, since a
// press in a page never reaches it.
func KeyboardToWindow() {
	if !gtkUp.Load() {
		return
	}
	onGTK(func() { C.lich_webview_keyboard_to_window() })
}

var ground color.NRGBA

// SetGround is the color behind the pages, shown where a page leaves the
// window until the window's next frame: the pane's own background.
func SetGround(c color.NRGBA) {
	if c == ground || !gtkUp.Load() {
		return
	}
	ground = c
	pixel := C.ulong(0xff)<<24 | C.ulong(c.R)<<16 | C.ulong(c.G)<<8 | C.ulong(c.B)
	onGTK(func() { C.lich_webview_set_ground(pixel) })
}
