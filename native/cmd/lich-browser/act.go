//go:build linux || windows

package main

/*
#include <stdlib.h>
#include "browser.h"
*/
import "C"

import (
	"unsafe"

	"github.com/lichdotdev/lich/browserwire"
)

func (b *browser) draw(o browserwire.DrawParams) (any, error) {
	p, err := b.page(o.Page)
	if err != nil {
		return nil, err
	}
	if p.drawing.offer(o.Script) {
		p.sendDrawing(o.Script)
	}
	return nil, nil
}

func (p *page) sendDrawing(script string) {
	s := C.CString(script)
	defer C.free(unsafe.Pointer(s))
	C.lich_tab_draw(p.front.c, s, C.uintptr_t(p.key))
}

//export lichDrawn
func lichDrawn(key C.uintptr_t) {
	p := the.byKey[uintptr(key)]
	if p == nil {
		return
	}
	if script, ok := p.drawing.done(); ok {
		p.sendDrawing(script)
	}
}

// layerVisibility shows or hides the window's layer.
const layerVisibility = `(h => h && (h.style.visibility = %q))(document.getElementById(%q))`

func (t *tab) run(script string) {
	s := C.CString(script)
	defer C.free(unsafe.Pointer(s))
	C.lich_tab_run(t.c, s)
}

