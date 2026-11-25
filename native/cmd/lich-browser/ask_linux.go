package main

/*
#include <stdlib.h>
#include "browser_linux.h"
*/
import "C"

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"unsafe"

	"github.com/lichdotdev/lich/browserwire"
)

var (
	errNoDialog  = errors.New("no dialog is open")
	errNoChooser = errors.New("no file chooser is open")
)

// WebKitScriptDialogType's values, in its order.
var dialogTypes = [...]string{browserwire.DialogAlert, browserwire.DialogConfirm, browserwire.DialogPrompt, browserwire.DialogBeforeUnload}

// lichDialog holds a dialog until the wire answers it. A page shows one at a
// time; another of its tabs asking meanwhile is dismissed.
//
//export lichDialog
func lichDialog(key C.uintptr_t, d *C.WebKitScriptDialog, kind C.int, message, text *C.char) C.int {
	C.webkit_script_dialog_ref(d)
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

//export lichFiles
func lichFiles(key C.uintptr_t, r *C.WebKitFileChooserRequest, multiple C.int) {
	C.g_object_ref(C.gpointer(r))
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

//export lichTLS
func lichTLS(key C.uintptr_t, uri, chain *C.char) {
	t := the.tabs[uintptr(key)]
	if t == nil {
		return
	}
	u, err := url.Parse(C.GoString(uri))
	if err != nil || t.repaired[u.Hostname()] {
		return
	}
	go t.completeChain(u, []byte(C.GoString(chain)))
}

// completeChain fetches the issuers a server left out and, once its chain
// verifies, trusts its leaf for the host and loads the page again.
func (t *tab) completeChain(u *url.URL, chain []byte) {
	host := u.Hostname()
	leaf, err := completeChain(context.Background(), http.DefaultClient, nil, host, chain)
	if err != nil {
		log.Printf("%s: certificate chain: %v", host, err)
		return
	}
	onGTK(func() {
		if the.tabs[t.key] != t || t.repaired[host] {
			return
		}
		if t.repaired == nil {
			t.repaired = map[string]bool{}
		}
		t.repaired[host] = true
		l, h, uri := C.CString(string(leaf)), C.CString(host), C.CString(u.String())
		defer C.free(unsafe.Pointer(l))
		defer C.free(unsafe.Pointer(h))
		defer C.free(unsafe.Pointer(uri))
		C.lich_tab_allow(t.c, l, h, uri)
	})
}
