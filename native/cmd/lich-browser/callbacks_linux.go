package main

/*
#include <stdlib.h>
#include "browser_linux.h"
*/
import "C"

import (
	"log"
	"strconv"
	"unsafe"

	"github.com/lichdotdev/lich/browserwire"
)

// WebKitLoadEvent's values; a failed load ends with its finish too.
const (
	loadStarted   = 0
	loadCommitted = 2
	loadFinished  = 3
)

//export lichLoad
func lichLoad(key C.uintptr_t, step C.int, uri *C.char) {
	t := the.tabs[uintptr(key)]
	if t == nil {
		return
	}
	var s string
	switch step {
	case loadStarted:
		t.failed = false
		s = browserwire.LoadStarted
	case loadCommitted:
		s = browserwire.LoadCommitted
	case loadFinished:
		if t.failed {
			return
		}
		s = browserwire.LoadFinished
	default:
		return
	}
	t.sendLoad(s, C.GoString(uri))
}

//export lichLoadFailed
func lichLoadFailed(key C.uintptr_t, uri, message *C.char, download C.int) {
	t := the.tabs[uintptr(key)]
	if t == nil || t.failed {
		return
	}
	t.failed = true
	the.wire.event(browserwire.EventLoad, browserwire.LoadEvent{
		Page: t.page.id, Tab: t.id, Step: browserwire.LoadFailed, URL: C.GoString(uri), At: the.at(),
		Error: C.GoString(message), Download: download != 0,
	})
}

func (t *tab) sendLoad(step, url string) {
	the.wire.event(browserwire.EventLoad, browserwire.LoadEvent{Page: t.page.id, Tab: t.id, Step: step, URL: url, At: the.at()})
}

//export lichTabChanged
func lichTabChanged(key C.uintptr_t, icon C.int) {
	t := the.tabs[uintptr(key)]
	if t == nil || t.c == nil {
		return
	}
	if icon != 0 {
		t.icon = iconOf(t)
	}
	the.sendTabs(t.page)
}

func iconOf(t *tab) []byte {
	var n C.int
	png := C.lich_tab_icon(t.c, &n)
	if png == nil {
		return nil
	}
	defer C.g_free(C.gpointer(png))
	return C.GoBytes(unsafe.Pointer(png), n)
}

// lichTabCreate is the key of the tab a page of opener's opens, 0 to refuse
// it.
//
//export lichTabCreate
func lichTabCreate(opener C.uintptr_t) C.uintptr_t {
	t := the.tabs[uintptr(opener)]
	if t == nil {
		return 0
	}
	return C.uintptr_t(the.newTab(t.page).key)
}

//export lichTabCreated
func lichTabCreated(key C.uintptr_t, c *C.LichTab) {
	if t := the.tabs[uintptr(key)]; t != nil {
		the.placeTab(t, c)
	}
}

// lichTabClosing is a page closing its own tab (window.close).
//
//export lichTabClosing
func lichTabClosing(key C.uintptr_t) {
	if t := the.tabs[uintptr(key)]; t != nil {
		_ = the.closeTab(t)
	}
}

// lichRequestStarted tells a request of key's front tab starting, and
// answers the id its finish names, 0 for one of a tab not in front, which is
// not told.
//
//export lichRequestStarted
func lichRequestStarted(key C.uintptr_t, uri, accept *C.char, document C.int) C.int {
	t := the.tabs[uintptr(key)]
	if t == nil || t != t.page.front {
		return 0
	}
	kind := browserwire.RequestDocument
	if document == 0 {
		kind = requestType(C.GoString(uri), C.GoString(accept))
	}
	the.requests++
	id := the.requests
	the.wire.event(browserwire.EventRequest, browserwire.RequestEvent{
		Page: t.page.id, Tab: t.id, ID: strconv.Itoa(id), Type: kind, Started: true, At: the.at(),
	})
	return C.int(id)
}

//export lichRequestFinished
func lichRequestFinished(key C.uintptr_t, id C.int) {
	if t := the.tabs[uintptr(key)]; t != nil {
		the.wire.event(browserwire.EventRequest, browserwire.RequestEvent{
			Page: t.page.id, Tab: t.id, ID: strconv.Itoa(int(id)), Finished: true, At: the.at(),
		})
	}
}

//export lichCrashed
func lichCrashed(key C.uintptr_t) {
	if t := the.tabs[uintptr(key)]; t != nil {
		the.wire.event(browserwire.EventCrashed, browserwire.PageParams{Page: t.page.id})
	}
}

//export lichConsole
func lichConsole(key C.uintptr_t, text *C.char) {
	if p := the.byKey[uintptr(key)]; p != nil {
		the.wire.event(browserwire.EventConsole, browserwire.ConsoleEvent{Page: p.id, Text: C.GoString(text)})
	}
}

// lichDownloadPath is where a download of the page's lands, reserved, NULL
// to cancel it.
//
//export lichDownloadPath
func lichDownloadPath(key C.uintptr_t, suggested *C.char) *C.char {
	p := the.byKey[uintptr(key)]
	if p == nil {
		return nil
	}
	path, err := reserve(p.downloads, C.GoString(suggested))
	if err != nil {
		log.Printf("download: %v", err)
		return nil
	}
	return C.CString(path)
}

var downloadStates = [...]string{browserwire.DownloadInProgress, browserwire.DownloadCompleted, browserwire.DownloadCanceled}

//export lichDownload
func lichDownload(key C.uintptr_t, guid C.int, url, name, path *C.char, state C.int, received, total C.gint64) {
	p := the.byKey[uintptr(key)]
	if p == nil || int(state) >= len(downloadStates) {
		return
	}
	the.wire.event(browserwire.EventDownload, browserwire.DownloadEvent{
		Page: p.id, GUID: strconv.Itoa(int(guid)), URL: C.GoString(url), Name: C.GoString(name), Path: C.GoString(path),
		State: downloadStates[state], Received: int64(received), Total: int64(total),
	})
}
