package main

/*
#include "browser.h"
*/
import "C"

import (
	"encoding/json"
	"log"

	"github.com/lichdotdev/lich/browserwire"
)

//export lichNavigationFailed
func lichNavigationFailed(key C.uintptr_t, uri *C.char, status C.int) {
	if t := the.tabs[uintptr(key)]; t != nil {
		the.loadFailed(t, C.GoString(uri), navigationError(int(status)), false)
	}
}

//export lichMessage
func lichMessage(key C.uintptr_t, raw *C.char) {
	p := the.byKey[uintptr(key)]
	if p == nil {
		return
	}
	// lichInput is the Linux engine's (seen_linux.go): input here is
	// answered by the protocol.
	if handler, text, ok := pageMessage(C.GoString(raw)); ok && handler == "lich" {
		the.wire.event(browserwire.EventConsole, browserwire.ConsoleEvent{Page: p.id, Text: text})
	}
}

// cdpEvent is the part of a DevTools protocol event lichCDPEvent reads.
type cdpEvent struct {
	RequestID string `json:"requestId"`
	Type      string `json:"type"`
	// Redirect is set on a request that goes on under the same id.
	Redirect      json.RawMessage `json:"redirectResponse"`
	Mode          string          `json:"mode"`
	BackendNodeID int64           `json:"backendNodeId"`
}

//export lichCDPEvent
func lichCDPEvent(key C.uintptr_t, name, raw *C.char) {
	t := the.tabs[uintptr(key)]
	if t == nil {
		return
	}
	var e cdpEvent
	if err := json.Unmarshal([]byte(C.GoString(raw)), &e); err != nil {
		log.Printf("devtools event %s: %v", C.GoString(name), err)
		return
	}
	switch C.GoString(name) {
	case "Network.requestWillBeSent":
		the.cdpRequestStarted(t, e)
	case "Network.loadingFinished", "Network.loadingFailed":
		if id, ok := t.requests[e.RequestID]; ok {
			delete(t.requests, e.RequestID)
			the.requestFinished(t, id)
		}
	case "Page.fileChooserOpened":
		the.chooserOpened(t, e)
	}
}

func (b *browser) cdpRequestStarted(t *tab, e cdpEvent) {
	if _, going := t.requests[e.RequestID]; going || len(e.Redirect) > 0 || t != t.page.front {
		return
	}
	if t.requests == nil {
		t.requests = map[string]int{}
	}
	t.requests[e.RequestID] = b.requestStarted(t, requestKind(e.Type), "", "")
}

// chooserOpened holds a file input's ask for files until the wire answers
// it; another asking meanwhile is left unanswered, as a cancel.
func (b *browser) chooserOpened(t *tab, e cdpEvent) {
	p := t.page
	if p.chooser != 0 || e.BackendNodeID == 0 {
		return
	}
	p.chooser, p.chooserTab = e.BackendNodeID, t
	b.wire.event(browserwire.EventFiles, browserwire.FilesEvent{Page: p.id, Multiple: e.Mode == "selectMultiple"})
}
