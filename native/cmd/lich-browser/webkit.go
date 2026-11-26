//go:build linux || darwin

package main

/*
#include <stdlib.h>
#include "browser.h"
*/
import "C"

import (
	"bytes"
	"errors"
	"fmt"
	"image/png"
	"math"
	"runtime/cgo"
	"unsafe"

	"github.com/lichdotdev/lich/browserwire"
)

// WebKit's engines, WebKitGTK and WKWebView, share what lies above their C
// side: the same requests, the input that waits for the page's DOM event, the
// console's eval, the snapshot and the file chooser.

// osBrowser is what a WebKit engine adds to the browser.
type osBrowser struct {
	// inputs are the input requests not answered yet, the first one acting.
	inputs []func(next func())
}

// postToHandler posts a message to one of the page's WebKit message handlers.
const postToHandler = "(handler, message) => webkit.messageHandlers[handler].postMessage(message)"

var osHandlers = map[string]handler{
	browserwire.Mouse:    awaited((*browser).mouse),
	browserwire.Key:      awaited((*browser).key),
	browserwire.Text:     awaited((*browser).text),
	browserwire.Files:    now((*browser).answerFiles),
	browserwire.Eval:     later((*browser).eval),
	browserwire.Snapshot: later((*browser).snapshot),
}

// mouseEvents and keyEvents are the DOM events a mouse or key event from the
// wire waits for, none for a move. WebKit queues mouse events apart from keys,
// each until the page took the one before, so a key sent just after a click
// reached the page first and typed nowhere. Answered once the page has its
// event, as Chrome answers one, a click is in before what follows it.
var (
	mouseEvents = map[string]string{browserwire.MouseDown: "mousedown", browserwire.MouseUp: "mouseup", browserwire.MouseWheel: "wheel"}
	keyEvents   = map[string]string{browserwire.KeyDown: "keydown", browserwire.KeyUp: "keyup"}
)

// awaited is a handler answering once the page has the DOM event act names,
// at once when it names none. It acts once every input event before it was
// answered, so the order holds however fast the requests come.
func awaited[P any](act func(b *browser, o P) (*page, string, error)) handler {
	return later(func(b *browser, o P, done func(any, error)) {
		b.inOrder(func(next func()) {
			answer := func(r any, err error) {
				done(r, err)
				next()
			}
			p, event, err := act(b, o)
			if err != nil || event == "" {
				answer(nil, err)
				return
			}
			p.await(event, answer)
		})
	})
}

var errNoChooser = errors.New("no file chooser is open")

func (p *page) cancelChooser() {
	if p.chooser != nil {
		C.lich_files_answer(p.chooser, nil, 0)
		p.chooser = nil
	}
}

//export lichFiles
func lichFiles(key C.uintptr_t, r *C.LichChooser, multiple C.int) {
	t := the.tabs[uintptr(key)]
	if t == nil || t.page.chooser != nil {
		C.lich_files_answer(r, nil, 0)
		return
	}
	t.page.chooser = r
	the.wire.event(browserwire.EventFiles, browserwire.FilesEvent{Page: t.page.id, Multiple: multiple != 0})
}

func (b *browser) answerFiles(o browserwire.FilesParams) (any, error) {
	p, err := b.page(o.Page)
	if err != nil {
		return nil, err
	}
	if p.chooser == nil {
		return nil, errNoChooser
	}
	// NULL-terminated, as WebKit reads it.
	paths := make([]*C.char, len(o.Paths)+1)
	for i, path := range o.Paths {
		paths[i] = C.CString(path)
		defer C.free(unsafe.Pointer(paths[i]))
	}
	C.lich_files_answer(p.chooser, (**C.char)(unsafe.Pointer(&paths[0])), C.int(len(o.Paths)))
	p.chooser = nil
	return nil, nil
}

type evaluated struct {
	json string
	err  error
}

func (b *browser) eval(o browserwire.EvalParams, done func(any, error)) {
	p, err := b.page(o.Page)
	if err != nil {
		done(nil, err)
		return
	}
	answer := func(r evaluated) {
		if r.err != nil {
			done(nil, r.err)
			return
		}
		done(browserwire.EvalResult{JSON: r.json}, nil)
	}
	run, ok := consoleForm(o.Script, jsParses)
	if !ok {
		run = consoleRun{program: o.Script}
	}
	program, body := cOrNil(run.program, !run.async), cOrNil(run.body, run.body != "")
	defer C.free(unsafe.Pointer(program))
	defer C.free(unsafe.Pointer(body))
	C.lich_tab_eval(p.front.c, program, body, C.uintptr_t(cgo.NewHandle(answer)))
}

// cOrNil is s for C, NULL unless set.
func cOrNil(s string, set bool) *C.char {
	if !set {
		return nil
	}
	return C.CString(s)
}

func jsParses(code string) bool {
	c := C.CString(code)
	defer C.free(unsafe.Pointer(c))
	return C.lich_js_parses(c) != 0
}

//export lichEvaluated
func lichEvaluated(handle C.uintptr_t, json, message *C.char) {
	h := cgo.Handle(handle)
	answer := h.Value().(func(evaluated))
	h.Delete()
	if message != nil {
		answer(evaluated{err: errors.New(C.GoString(message))})
		return
	}
	answer(evaluated{json: C.GoString(json)})
}

type shot struct {
	pixels              []byte
	width, height, step int
	err                 error
}

func (b *browser) snapshot(o browserwire.SnapshotParams, done func(any, error)) {
	p, err := b.page(o.Page)
	if err != nil {
		done(nil, err)
		return
	}
	scale := p.scale
	answer := func(s shot) {
		if s.err != nil {
			done(nil, s.err)
			return
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, fromARGB32(s.pixels, s.width, s.height, s.step)); err != nil {
			done(nil, err)
			return
		}
		done(browserwire.SnapshotResult{PNG: buf.Bytes(), W: int(math.Round(float64(s.width) / scale)), H: int(math.Round(float64(s.height) / scale))}, nil)
	}
	// The page takes the hide, the shot and the show in that order.
	front := p.front
	if o.Bare {
		front.run(fmt.Sprintf(layerVisibility, "hidden", browserwire.LayerID))
		defer front.run(fmt.Sprintf(layerVisibility, "", browserwire.LayerID))
	}
	C.lich_tab_snapshot(front.c, C.uintptr_t(cgo.NewHandle(answer)))
}

//export lichSnapshot
func lichSnapshot(handle C.uintptr_t, data *C.uchar, width, height, stride C.int, message *C.char) {
	h := cgo.Handle(handle)
	answer := h.Value().(func(shot))
	h.Delete()
	if message != nil {
		answer(shot{err: errors.New(C.GoString(message))})
		return
	}
	answer(shot{pixels: C.GoBytes(unsafe.Pointer(data), stride*height), width: int(width), height: int(height), step: int(stride)})
}
