// Package webview shows a web page in the window's own place: on Linux a
// WebKitGTK view in an X11 window made a child of the window Gio draws into.
// GTK runs on a thread of its own, started with the first page, and every
// call into a page goes to it in the order it was made.
package webview

/*
#cgo pkg-config: gtk+-3.0 webkit2gtk-4.1 x11 xext
#include <stdlib.h>
#include "webview_linux.h"
*/
import "C"

import (
	"context"
	"errors"
	"image"
	"image/color"
	"runtime"
	"runtime/cgo"
	"slices"
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
// Wayland, or before the window exists; display is Gio's connection to it,
// which Gio opened thread-safe (XInitThreads).
var (
	parent  atomic.Uint64
	display atomic.Pointer[byte]
)

// SetParent records the window pages go in, from the window's ViewEvent.
func SetParent(e app.ViewEvent) {
	x, ok := e.(app.X11ViewEvent)
	if !ok || !x.Valid() {
		parent.Store(0)
		display.Store(nil)
		return
	}
	parent.Store(uint64(x.Window))
	display.Store((*byte)(x.Display))
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
	cuts   []image.Rectangle
	// pointerOff: the pointer goes through the page to the window.
	pointerOff bool
	zoom       float64
	// child is the page's X window, once the GTK thread made it.
	child atomic.Uint64
	// closed is touched only on the GTK thread: a call queued after Close
	// finds the page gone.
	closed bool
	loads  loads
	// onLoad hears the page's loads, on the GTK thread.
	onLoad func(Load)
	self   cgo.Handle
	// drawing is touched only on the GTK thread.
	drawing latest
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
		p.child.Store(uint64(p.page.child))
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

// Place shows the page over r, in the window's pixels, left out of cuts, local
// to r, where the window draws over it; it says whether the cuts changed, since
// a cut shows the window's drawing only from its next frame. It moves the page
// only when r moved: a resize makes WebKit lay the page out again.
func (p *Page) Place(r image.Rectangle, cuts []image.Rectangle) bool {
	moved := !p.shown || r != p.placed
	cut := !slices.Equal(cuts, p.cuts) || r.Size() != p.placed.Size()
	if !moved && !cut {
		return false
	}
	if moved {
		onGTK(func() {
			C.lich_webview_place(p.page, C.int(r.Min.X), C.int(r.Min.Y), C.int(max(r.Dx(), 1)), C.int(max(r.Dy(), 1)))
		})
	}
	p.placed, p.shown = r, true
	if cut && p.shape(r.Size(), cuts) {
		p.cuts = slices.Clone(cuts)
	}
	return cut
}

// shape cuts the page's window on the window's own X connection (see
// lich_webview_shape), or reports that it cannot yet: the GTK thread has not
// made the page's window.
func (p *Page) shape(size image.Point, cuts []image.Rectangle) bool {
	x, child := (*C.Display)(unsafe.Pointer(display.Load())), p.child.Load()
	if x == nil || child == 0 {
		return false
	}
	rects := make([]C.XRectangle, len(cuts))
	for i, c := range cuts {
		rects[i] = C.XRectangle{x: C.short(c.Min.X), y: C.short(c.Min.Y), width: C.ushort(c.Dx()), height: C.ushort(c.Dy())}
	}
	var first *C.XRectangle
	if len(rects) > 0 {
		first = &rects[0]
	}
	C.lich_webview_shape(x, C.ulong(child), C.int(size.X), C.int(size.Y), first, C.int(len(rects)))
	return true
}

// Placed is where the page was last placed, in the window's pixels.
func (p *Page) Placed() image.Rectangle {
	return p.placed
}

// PointerToWindow lets the pointer through the page to the window, which
// draws and answers over it while a tool of its own is on.
func (p *Page) PointerToWindow() {
	if p.pointerOff {
		return
	}
	p.pointerOff = true
	onGTK(func() { C.lich_webview_pointer_to_window(p.page) })
}

// PointerToPage gives the pointer back to the page.
func (p *Page) PointerToPage() {
	if !p.pointerOff {
		return
	}
	p.pointerOff = false
	onGTK(func() { C.lich_webview_pointer_to_page(p.page) })
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

// Reload loads the page again, past the cache, as the window's own load.
func (p *Page) Reload() {
	p.loads.ask(p.loads.url())
	onGTK(func() { C.lich_webview_reload(p.page) })
}

// Inspect opens WebKit's Web Inspector on the page, in a window of its own.
func (p *Page) Inspect() {
	onGTK(func() {
		if !p.closed {
			C.lich_webview_inspect(p.page)
		}
	})
}

// Close ends the page.
func (p *Page) Close() {
	onGTK(func() {
		p.closed = true
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

var errClosed = errors.New("webview: the page is closed")

type evaluated struct {
	json string
	err  error
}

// Eval runs script in the page and answers its value as JSON.
func (p *Page) Eval(ctx context.Context, script string) (string, error) {
	done := make(chan evaluated, 1)
	onGTK(func() {
		if p.closed {
			done <- evaluated{err: errClosed}
			return
		}
		s := C.CString(script)
		defer C.free(unsafe.Pointer(s))
		C.lich_webview_eval(p.page, s, C.uintptr_t(cgo.NewHandle(done)))
	})
	select {
	case r := <-done:
		return r.json, r.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

//export lichWebviewEvaluated
func lichWebviewEvaluated(handle C.uintptr_t, json, message *C.char) {
	h := cgo.Handle(handle)
	done := h.Value().(chan evaluated)
	h.Delete()
	if message != nil {
		done <- evaluated{err: errors.New(C.GoString(message))}
		return
	}
	done <- evaluated{json: C.GoString(json)}
}

type snapshot struct {
	img *image.RGBA
	err error
}

// Snapshot is the part of the page in sight, as WebKit draws it.
func (p *Page) Snapshot(ctx context.Context) (*image.RGBA, error) {
	done := make(chan snapshot, 1)
	onGTK(func() {
		if p.closed {
			done <- snapshot{err: errClosed}
			return
		}
		C.lich_webview_snapshot(p.page, C.uintptr_t(cgo.NewHandle(done)))
	})
	select {
	case r := <-done:
		return r.img, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

//export lichWebviewSnapshot
func lichWebviewSnapshot(handle C.uintptr_t, data *C.uchar, width, height, stride C.int, message *C.char) {
	h := cgo.Handle(handle)
	done := h.Value().(chan snapshot)
	h.Delete()
	if message != nil {
		done <- snapshot{err: errors.New(C.GoString(message))}
		return
	}
	pixels := unsafe.Slice((*byte)(unsafe.Pointer(data)), int(stride)*int(height))
	done <- snapshot{img: fromARGB32(pixels, int(width), int(height), int(stride))}
}

// Run runs script in the page for what it does, not waiting for it.
func (p *Page) Run(script string) {
	onGTK(func() {
		if p.closed {
			return
		}
		s := C.CString(script)
		defer C.free(unsafe.Pointer(s))
		C.lich_webview_run(p.page, s)
	})
}

// Draw runs script in the page to draw over it, a newer drawing taking the
// place of one not yet sent while the page still runs the last.
func (p *Page) Draw(script string) {
	onGTK(func() {
		if !p.closed && p.drawing.offer(script) {
			p.sendDrawing(script)
		}
	})
}

func (p *Page) sendDrawing(script string) {
	s := C.CString(script)
	defer C.free(unsafe.Pointer(s))
	C.lich_webview_draw(p.page, s, C.uintptr_t(cgo.NewHandle(p)))
}

//export lichWebviewDrawn
func lichWebviewDrawn(handle C.uintptr_t) {
	h := cgo.Handle(handle)
	p := h.Value().(*Page)
	h.Delete()
	if script, ok := p.drawing.done(); ok && !p.closed {
		p.sendDrawing(script)
	}
}

// SetZoom draws the page at scale device pixels per CSS pixel.
func (p *Page) SetZoom(scale float64) {
	if scale == p.zoom {
		return
	}
	p.zoom = scale
	onGTK(func() { C.lich_webview_zoom(p.page, C.double(scale)) })
}
