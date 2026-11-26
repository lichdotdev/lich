//go:build linux || windows || darwin

package main

/*
#include <stdlib.h>
#include "browser.h"
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"maps"
	"time"

	"github.com/lichdotdev/lich/browserwire"
)

// the is the one browser, touched only on GTK's thread: the callbacks WebKit
// makes there reach it by the keys they carry.
var the *browser

type browser struct {
	wire  *wire
	start time.Time
	pages map[string]*page
	// byKey and tabs are the pages and tabs by the keys their C side carries;
	// a callback for one already closed finds none.
	byKey map[uintptr]*page
	tabs  map[uintptr]*tab
	keys  uintptr
	// requests counts the requests told, each one's id.
	requests int
	osBrowser
}

func newBrowser(w *wire) *browser {
	return &browser{wire: w, start: time.Now(), pages: map[string]*page{}, byKey: map[uintptr]*page{}, tabs: map[uintptr]*tab{}}
}

func (b *browser) newKey() uintptr {
	b.keys++
	return b.keys
}

// at is the clock every event reads, monotonic from lich-browser's start.
func (b *browser) at() int64 {
	return time.Since(b.start).Nanoseconds()
}

// handler answers one request, now or once WebKit has: done is called once.
type handler func(b *browser, req browserwire.Request, done func(any, error))

func (b *browser) handle(req browserwire.Request) {
	h, ok := handlers[req.Method]
	if !ok {
		b.wire.reply(req.ID, nil, fmt.Errorf("no method %q", req.Method))
		return
	}
	h(b, req, func(result any, err error) { b.wire.reply(req.ID, result, err) })
}

// now is a handler that answers as soon as do returns, its params P.
func now[P any](do func(b *browser, p P) (any, error)) handler {
	return func(b *browser, req browserwire.Request, done func(any, error)) {
		var p P
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &p); err != nil {
				done(nil, err)
				return
			}
		}
		done(do(b, p))
	}
}

// later is a handler whose do answers once WebKit has.
func later[P any](do func(b *browser, p P, done func(any, error))) handler {
	return func(b *browser, req browserwire.Request, done func(any, error)) {
		var p P
		if err := json.Unmarshal(req.Params, &p); err != nil {
			done(nil, err)
			return
		}
		do(b, p, done)
	}
}

// handlers are the requests every engine answers alike; each system's own
// add the rest (osHandlers).
var handlers = map[string]handler{
	browserwire.Hello: now(func(*browser, struct{}) (any, error) {
		return browserwire.HelloResult{Version: browserwire.Version}, nil
	}),
	browserwire.Window: now(func(_ *browser, p browserwire.WindowParams) (any, error) {
		C.lich_browser_set_app(C.ulong(p.XID))
		return nil, nil
	}),
	browserwire.Clock: now(func(b *browser, _ struct{}) (any, error) {
		return browserwire.ClockResult{NS: b.at()}, nil
	}),
	browserwire.Open:      now((*browser).open),
	browserwire.Close:     now((*browser).close),
	browserwire.NewTab:    now((*browser).newTabReq),
	browserwire.SelectTab: now((*browser).selectTab),
	browserwire.CloseTab:  now((*browser).closeTabReq),
	browserwire.Navigate:  now((*browser).navigate),
	browserwire.Reload:    now(onFront(func(t *tab) { C.lich_tab_reload(t.c) })),
	browserwire.Back:      now(onFront(func(t *tab) { C.lich_tab_back(t.c) })),
	browserwire.Forward:   now(onFront(func(t *tab) { C.lich_tab_forward(t.c) })),
	browserwire.Inspect:   now((*browser).inspect),
	browserwire.Zoom:      now((*browser).zoom),
	browserwire.Size:      now((*browser).size),
	browserwire.Dialog:    now((*browser).answerDialog),
	browserwire.PopOut:    now((*browser).popOut),
	browserwire.PopIn:     now((*browser).popIn),
	browserwire.Raise:     now((*browser).raise),
	browserwire.Draw:      now((*browser).draw),
}

func init() { maps.Copy(handlers, osHandlers) }

// onFront is a request that does act on the page's front tab.
func onFront(act func(*tab)) func(*browser, browserwire.PageParams) (any, error) {
	return func(b *browser, p browserwire.PageParams) (any, error) {
		pg, err := b.page(p.Page)
		if err != nil {
			return nil, err
		}
		act(pg.front)
		return nil, nil
	}
}
