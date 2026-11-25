package main

/*
#include <stdlib.h>
#include "browser_linux.h"
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

var mouseKinds = map[string]C.int{browserwire.MouseMove: 0, browserwire.MouseDown: 1, browserwire.MouseUp: 2, browserwire.MouseWheel: 3}

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

func (b *browser) mouse(o browserwire.MouseParams) (*page, string, error) {
	p, err := b.page(o.Page)
	if err != nil {
		return nil, "", err
	}
	kind, ok := mouseKinds[o.Kind]
	if !ok {
		return nil, "", fmt.Errorf("no mouse event %q", o.Kind)
	}
	// shortcut: Clicks is not passed on, WebKit counts a double click by the
	// time and distance between presses as it does a person's; a backend
	// that needs a count apart from timing needs a WebKit that takes one.
	button := gdkButton(o.Button)
	state := modState(o.Mods) | p.buttons
	if o.Kind == browserwire.MouseDown {
		C.lich_tab_focus(p.front.c)
	}
	C.lich_tab_mouse(p.front.c, kind, C.double(o.X), C.double(o.Y), C.guint(button), C.guint(state), C.double(o.DX), C.double(o.DY))
	switch o.Kind {
	case browserwire.MouseDown:
		p.buttons |= buttonMask(button)
	case browserwire.MouseUp:
		p.buttons &^= buttonMask(button)
	}
	return p, mouseEvents[o.Kind], nil
}

func unicodeKeyval(r rune) uint32 {
	return uint32(C.lich_keyval_for(C.gunichar(r)))
}

func (b *browser) key(o browserwire.KeyParams) (*page, string, error) {
	p, err := b.page(o.Page)
	if err != nil {
		return nil, "", err
	}
	v := keyval(o.Key, unicodeKeyval)
	if v == 0 {
		return nil, "", fmt.Errorf("no key %q", o.Key)
	}
	event, ok := keyEvents[o.Kind]
	if !ok {
		return nil, "", fmt.Errorf("no key event %q", o.Kind)
	}
	press := C.int(0)
	if o.Kind == browserwire.KeyDown {
		press = 1
	}
	C.lich_tab_focus(p.front.c)
	C.lich_tab_key(p.front.c, press, C.guint(v), C.guint(modState(o.Mods)))
	return p, event, nil
}

func (b *browser) text(o browserwire.TextParams) (*page, string, error) {
	p, err := b.page(o.Page)
	if err != nil {
		return nil, "", err
	}
	text := C.CString(o.Text)
	defer C.free(unsafe.Pointer(text))
	C.lich_tab_focus(p.front.c)
	C.lich_tab_text(p.front.c, text)
	return p, "", nil
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
	script := C.CString(o.Script)
	defer C.free(unsafe.Pointer(script))
	answer := func(r evaluated) {
		if r.err != nil {
			done(nil, r.err)
			return
		}
		done(browserwire.EvalResult{JSON: r.json}, nil)
	}
	C.lich_tab_eval(p.front.c, script, C.uintptr_t(cgo.NewHandle(answer)))
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

func (b *browser) snapshot(o browserwire.PageParams, done func(any, error)) {
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
	C.lich_tab_snapshot(p.front.c, C.uintptr_t(cgo.NewHandle(answer)))
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
