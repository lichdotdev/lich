//go:build linux || windows || darwin

package main

/*
#include <stdlib.h>
#include "browser.h"
*/
import "C"

import (
	"errors"
	"unsafe"

	"github.com/lichdotdev/lich/browserwire"
)

var errNoDialog = errors.New("no dialog is open")

// The dialog types the engine's C side numbers, in its order.
var dialogTypes = [...]string{browserwire.DialogAlert, browserwire.DialogConfirm, browserwire.DialogPrompt, browserwire.DialogBeforeUnload}

// lichDialog holds a dialog, which the C side keeps alive, until the wire
// answers it. A page shows one at a time; another of its tabs asking
// meanwhile is dismissed.
//
//export lichDialog
func lichDialog(key C.uintptr_t, d *C.LichDialog, kind C.int, message, text *C.char) C.int {
	t := the.tabs[uintptr(key)]
	if t == nil || t.page.dialog != nil || int(kind) >= len(dialogTypes) {
		C.lich_dialog_answer(d, 0, nil)
		return 1
	}
	t.page.dialog = d
	the.wire.event(browserwire.EventDialog, browserwire.DialogEvent{Page: t.page.id, Dialog: &browserwire.JSDialog{
		Type: dialogTypes[kind], Message: C.GoString(message), Default: C.GoString(text),
	}})
	return 1
}

func (b *browser) answerDialog(o browserwire.DialogParams) (any, error) {
	p, err := b.page(o.Page)
	if err != nil {
		return nil, err
	}
	if p.dialog == nil {
		return nil, errNoDialog
	}
	text := C.CString(o.Text)
	defer C.free(unsafe.Pointer(text))
	accept := C.int(0)
	if o.Accept {
		accept = 1
	}
	C.lich_dialog_answer(p.dialog, accept, text)
	p.dialog = nil
	b.wire.event(browserwire.EventDialog, browserwire.DialogEvent{Page: p.id})
	return nil, nil
}
