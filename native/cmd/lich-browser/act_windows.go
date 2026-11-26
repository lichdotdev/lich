package main

/*
#include <stdlib.h>
#include "browser.h"
*/
import "C"

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"math"
	"runtime/cgo"
	"unsafe"

	"github.com/lichdotdev/lich/browserwire"
)

// postToHandler posts a message to the page's handler, which WebView2 has
// one of: chrome.webview carries [handler, message] (lichMessage).
const postToHandler = "(handler, message) => chrome.webview.postMessage([handler, String(message)])"

// osBrowser is what the WebView2 engine adds to the browser: nothing.
type osBrowser struct{}

// osPage is what the WebView2 engine adds to a page.
type osPage struct {
	// held is the DOM buttons held down, MouseEvent.buttons' bits.
	held int
	// chooser is the file input asking for files, the protocol's node id,
	// in chooserTab; zero for none.
	chooser    int64
	chooserTab *tab
}

// osTab is what the WebView2 engine adds to a tab: the wire's id of each of
// its requests under way, by the protocol's.
type osTab struct {
	requests map[string]int
}

// Input goes over the DevTools protocol, whose events the page sees as
// trusted, as Chrome's DevTools send them; each is answered once Chromium
// has handed it to the page, and in the order sent.
var osHandlers = map[string]handler{
	browserwire.Mouse:    later((*browser).mouse),
	browserwire.Key:      later((*browser).key),
	browserwire.Text:     later((*browser).text),
	browserwire.Files:    later((*browser).answerFiles),
	browserwire.Eval:     later((*browser).eval),
	browserwire.Snapshot: later((*browser).snapshot),
}

// cdp calls a DevTools protocol method of t's, answering done on the main
// thread.
func (t *tab) cdp(method string, params any, done func(json.RawMessage, error)) {
	raw, err := json.Marshal(params)
	if err != nil {
		done(nil, err)
		return
	}
	m, p := C.CString(method), C.CString(string(raw))
	defer C.free(unsafe.Pointer(m))
	defer C.free(unsafe.Pointer(p))
	C.lich_tab_cdp(t.c, m, p, C.uintptr_t(cgo.NewHandle(done)))
}

//export lichCDPDone
func lichCDPDone(handle C.uintptr_t, ok C.int, answer *C.char) {
	h := cgo.Handle(handle)
	done := h.Value().(func(json.RawMessage, error))
	h.Delete()
	s := C.GoString(answer)
	if ok == 0 {
		done(nil, cdpError(s))
		return
	}
	done(json.RawMessage(s), nil)
}

// answered is done once a call succeeded, with nothing to say.
func answered(done func(any, error)) func(json.RawMessage, error) {
	return func(_ json.RawMessage, err error) { done(nil, err) }
}

func (b *browser) mouse(o browserwire.MouseParams, done func(any, error)) {
	p, err := b.page(o.Page)
	if err != nil {
		done(nil, err)
		return
	}
	e, held, err := mouseEvent(o, p.held)
	if err != nil {
		done(nil, err)
		return
	}
	p.held = held
	p.front.cdp("Input.dispatchMouseEvent", e, answered(done))
}

func (b *browser) key(o browserwire.KeyParams, done func(any, error)) {
	p, err := b.page(o.Page)
	if err != nil {
		done(nil, err)
		return
	}
	e, err := keyEvent(o)
	if err != nil {
		done(nil, err)
		return
	}
	p.front.cdp("Input.dispatchKeyEvent", e, answered(done))
}

func (b *browser) text(o browserwire.TextParams, done func(any, error)) {
	p, err := b.page(o.Page)
	if err != nil {
		done(nil, err)
		return
	}
	p.front.cdp("Input.insertText", map[string]any{"text": o.Text}, answered(done))
}

func (b *browser) eval(o browserwire.EvalParams, done func(any, error)) {
	p, err := b.page(o.Page)
	if err != nil {
		done(nil, err)
		return
	}
	t := p.front
	answer := func(raw json.RawMessage, err error) {
		t.cdp("Runtime.releaseObjectGroup", map[string]any{"objectGroup": evalGroup}, func(json.RawMessage, error) {})
		if err == nil {
			var value string
			if value, err = evalAnswer(raw); err == nil {
				done(browserwire.EvalResult{JSON: value}, nil)
				return
			}
		}
		done(nil, err)
	}
	t.cdp("Runtime.evaluate", evaluate(o.Script), func(raw json.RawMessage, err error) {
		if method, params := evalNext(raw); err == nil && method != "" {
			t.cdp(method, params, answer)
			return
		}
		answer(raw, err)
	})
}

// framed waits for the page to draw twice, a change before it on screen.
const framed = "; new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r)))"

func (b *browser) snapshot(o browserwire.SnapshotParams, done func(any, error)) {
	p, err := b.page(o.Page)
	if err != nil {
		done(nil, err)
		return
	}
	front, scale := p.front, p.scale
	layer := func(visibility string) map[string]any {
		return map[string]any{"expression": fmt.Sprintf(layerVisibility, visibility, browserwire.LayerID) + framed, "awaitPromise": true}
	}
	answer := func(raw json.RawMessage, err error) {
		if err != nil {
			done(nil, err)
			return
		}
		shot, err := snapshotOf(raw, scale)
		done(shot, err)
	}
	capture := func(then func(json.RawMessage, error)) {
		front.cdp("Page.captureScreenshot", map[string]any{"format": "png", "fromSurface": true}, then)
	}
	if !o.Bare {
		capture(answer)
		return
	}
	// The layer is shown again before the answer: a snapshot right after
	// this one shows it.
	front.cdp("Runtime.evaluate", layer("hidden"), func(_ json.RawMessage, err error) {
		if err != nil {
			done(nil, err)
			return
		}
		capture(func(raw json.RawMessage, err error) {
			front.cdp("Runtime.evaluate", layer(""), func(json.RawMessage, error) { answer(raw, err) })
		})
	})
}

func snapshotOf(raw json.RawMessage, scale float64) (browserwire.SnapshotResult, error) {
	var r struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return browserwire.SnapshotResult{}, err
	}
	img, err := base64.StdEncoding.DecodeString(r.Data)
	if err != nil {
		return browserwire.SnapshotResult{}, err
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(img))
	if err != nil {
		return browserwire.SnapshotResult{}, err
	}
	return browserwire.SnapshotResult{PNG: img, W: int(math.Round(float64(cfg.Width) / scale)), H: int(math.Round(float64(cfg.Height) / scale))}, nil
}

func (p *page) cancelChooser() { p.chooser, p.chooserTab = 0, nil }

// answerFiles sets the input's files; none leaves it as it was, which is
// what a person cancelling the chooser leaves.
func (b *browser) answerFiles(o browserwire.FilesParams, done func(any, error)) {
	p, err := b.page(o.Page)
	if err != nil {
		done(nil, err)
		return
	}
	if p.chooser == 0 {
		done(nil, errNoChooser)
		return
	}
	node, t := p.chooser, p.chooserTab
	p.cancelChooser()
	if len(o.Paths) == 0 {
		done(nil, nil)
		return
	}
	t.cdp("DOM.setFileInputFiles", map[string]any{"files": o.Paths, "backendNodeId": node}, answered(done))
}
