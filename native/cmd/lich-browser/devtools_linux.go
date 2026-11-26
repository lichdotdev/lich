package main

/*
#include "browser_linux.h"
*/
import "C"

import (
	"strconv"

	"github.com/lichdotdev/lich/browserwire"
)

// A tab's WebKit inspector is a tab of the page's, right after the tab it
// inspects, as Chrome's DevTools are in the pane (browserwire.Tab.DevTools).
// While the pane shows it the page's front stays the inspected tab, which
// every request acts on.

func (b *browser) inspect(o browserwire.PageParams) (any, error) {
	p, err := b.page(o.Page)
	if err != nil {
		return nil, err
	}
	if p.front.devtools != nil {
		p.inspecting = true
		b.sendTabs(p)
		return nil, nil
	}
	C.lich_tab_inspect(p.front.c)
	return nil, nil
}

// lichDevtoolsKey is the key of the tab the inspector of inspected opens
// in, 0 to leave it to WebKit.
//
//export lichDevtoolsKey
func lichDevtoolsKey(inspected C.uintptr_t) C.uintptr_t {
	t := the.tabs[uintptr(inspected)]
	if t == nil || t.devtools != nil {
		return 0
	}
	d := &tab{key: the.newKey(), page: t.page, inspects: t}
	d.id = "t" + strconv.FormatUint(uint64(d.key), 10)
	the.tabs[d.key] = d
	return C.uintptr_t(d.key)
}

//export lichDevtoolsOpened
func lichDevtoolsOpened(key C.uintptr_t, c *C.LichTab) {
	d := the.tabs[uintptr(key)]
	if d == nil {
		return
	}
	d.c = c
	t := d.inspects
	t.devtools = d
	p := t.page
	p.front, p.inspecting = t, true
	p.showFront()
	the.sendTabs(p)
}

// lichDevtoolsClosed is inspected's inspector closing, from the pane's
// close or the inspector's own.
//
//export lichDevtoolsClosed
func lichDevtoolsClosed(inspected C.uintptr_t) {
	if t := the.tabs[uintptr(inspected)]; t != nil && t.devtools != nil {
		the.dropDevtools(t)
		the.sendTabs(t.page)
	}
}

func (b *browser) dropDevtools(t *tab) {
	d := t.devtools
	t.devtools = nil
	if t.page.front == t {
		t.page.inspecting = false
	}
	b.dropTab(d)
}

// closeDevtools closes t's inspector, the tab it is in with it.
func (b *browser) closeDevtools(t *tab) {
	if t.devtools == nil {
		return
	}
	C.lich_tab_close_inspector(t.c)
	if t.devtools != nil {
		b.dropDevtools(t)
	}
}

// devtoolsTab is the wire's tab for t's inspector, front while the pane
// shows it.
func (p *page) devtoolsTab(t *tab, xid uint64) browserwire.Tab {
	return browserwire.Tab{ID: t.devtools.id, Title: "Web Inspector", DevTools: true, Front: t == p.front && p.inspecting, XID: xid}
}
