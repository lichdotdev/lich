// Package webview shows a web page in the window's own place: on Linux a
// WebKitGTK view in an X11 window made a child of the window Gio draws into.
// GTK runs on a thread of its own, started with the first page, and every
// call into a page goes to it in the order it was made.
package webview

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1 x11
#include <stdlib.h>
#include "webview_linux.h"
*/
import "C"

import (
	"errors"
	"image"
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
}

// Open makes a page showing url, out of sight until it is placed.
func Open(url string) (*Page, error) {
	into := parent.Load()
	if into == 0 {
		return nil, errNoWindow
	}
	if err := startGTK(); err != nil {
		return nil, err
	}
	p := &Page{}
	onGTK(func() {
		u := C.CString(url)
		defer C.free(unsafe.Pointer(u))
		p.page = C.lich_webview_open(C.ulong(into), u)
	})
	return p, nil
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
	onGTK(func() {
		u := C.CString(url)
		defer C.free(unsafe.Pointer(u))
		C.lich_webview_navigate(p.page, u)
	})
}

// Close ends the page.
func (p *Page) Close() {
	onGTK(func() { C.lich_webview_close(p.page) })
}
