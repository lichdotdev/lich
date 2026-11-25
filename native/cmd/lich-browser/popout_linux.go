package main

/*
#include "browser_linux.h"
*/
import "C"

import (
	"errors"

	"github.com/lichdotdev/lich/browserwire"
)

var (
	errPoppedOut = errors.New("the page is in a window of its own already")
	errNotPopped = errors.New("the page is not in a window of its own")
)

func (p *page) poppedOut() bool { return p.c.popped != nil }

func (b *browser) popOut(o browserwire.PageParams) (any, error) {
	p, err := b.page(o.Page)
	if err != nil {
		return nil, err
	}
	if p.poppedOut() {
		return nil, errPoppedOut
	}
	w, h := p.device()
	C.lich_page_pop_out(p.c, p.front.c, w, h)
	b.sendTabs(p)
	return nil, nil
}

func (b *browser) popIn(o browserwire.PageParams) (any, error) {
	p, err := b.page(o.Page)
	if err != nil {
		return nil, err
	}
	if !p.poppedOut() {
		return nil, errNotPopped
	}
	C.lich_page_pop_in(p.c)
	b.sendTabs(p)
	return nil, nil
}

func (b *browser) raise(o browserwire.PageParams) (any, error) {
	p, err := b.page(o.Page)
	if err != nil {
		return nil, err
	}
	if !p.poppedOut() {
		return nil, errNotPopped
	}
	C.lich_page_raise(p.c)
	return nil, nil
}

// showFront puts the front tab in the page's own window when it has one.
func (p *page) showFront() {
	if p.poppedOut() {
		C.lich_page_show(p.c, p.front.c)
	}
}

//export lichClosed
func lichClosed(key C.uintptr_t) {
	p := the.byKey[uintptr(key)]
	if p == nil {
		return
	}
	the.wire.event(browserwire.EventClosed, browserwire.PageParams{Page: p.id})
	// Out of the window's own delete-event, which closing destroys.
	onGTK(func() { _, _ = the.close(browserwire.PageParams{Page: p.id}) })
}
