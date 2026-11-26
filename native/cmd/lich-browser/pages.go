//go:build linux || windows

package main

/*
#include <stdlib.h>
#include "browser.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"unsafe"

	"github.com/lichdotdev/lich/browserwire"
)

const blankPage = "about:blank"

var errNoDownloads = errors.New("open needs the directory downloads land in")

var errLastTab = errors.New("a page keeps its last tab: close the page instead")

// page is one of the backend's browsers: its tabs, the front one first in
// what it acts on.
type page struct {
	id    string
	key   uintptr
	c     *C.LichPage
	tabs  []*tab
	front *tab
	// scale is device pixels per CSS pixel; w, h the CSS size a tab lays out
	// at while no window holds it.
	scale float64
	w, h  int
	// downloads is the directory its downloads land in.
	downloads string
	// dialog is the JavaScript dialog the page waits on an answer for.
	dialog *C.LichDialog
	// drawing is the window's layer, one script in the page at a time.
	drawing latest
	// inspecting: the pane shows the front tab's inspector in its place.
	inspecting bool
	osPage
}

type tab struct {
	id   string
	key  uintptr
	page *page
	// c is nil until WebKit made the tab's view.
	c    *C.LichTab
	icon []byte
	// failed is whether the load under way failed, so its finish is not
	// told as well.
	failed bool
	// devtools is the tab this tab's inspector is in, nil while closed;
	// inspects is the tab an inspector's tab inspects.
	devtools, inspects *tab
	osTab
}

func (b *browser) page(id string) (*page, error) {
	p := b.pages[id]
	if p == nil {
		return nil, fmt.Errorf("page %s is not open", id)
	}
	return p, nil
}

// device is the tab's size in device pixels while no window holds it.
func (p *page) device() (C.int, C.int) {
	return C.int(max(1, math.Round(float64(p.w)*p.scale))), C.int(max(1, math.Round(float64(p.h)*p.scale)))
}

func (b *browser) open(o browserwire.OpenParams) (any, error) {
	if b.pages[o.Page] != nil {
		return nil, fmt.Errorf("page %s is open already", o.Page)
	}
	if o.Downloads == "" {
		return nil, errNoDownloads
	}
	p := &page{id: o.Page, key: b.newKey(), scale: o.Scale, w: o.W, h: o.H, downloads: o.Downloads}
	if p.scale <= 0 {
		p.scale = 1
	}
	script := C.CString(pageScript)
	p.c = C.lich_page_new(C.uintptr_t(p.key), script)
	C.free(unsafe.Pointer(script))
	b.pages[p.id], b.byKey[p.key] = p, p
	url := o.URL
	if url == "" {
		url = blankPage
	}
	b.addTab(p, url)
	return nil, nil
}

// addTab opens a tab in front of p on url.
func (b *browser) addTab(p *page, url string) {
	t := b.newTab(p)
	w, h := p.device()
	b.placeTab(t, C.lich_tab_new(p.c, nil, C.uintptr_t(t.key), w, h))
	t.navigate(url)
}

// newTab is a tab of p's that WebKit has not made yet.
func (b *browser) newTab(p *page) *tab {
	t := &tab{key: b.newKey(), page: p}
	t.id = "t" + strconv.FormatUint(uint64(t.key), 10)
	b.tabs[t.key] = t
	return t
}

// placeTab puts t, once WebKit made it, in front of its page.
func (b *browser) placeTab(t *tab, c *C.LichTab) {
	t.c = c
	p := t.page
	C.lich_tab_zoom(c, C.double(p.scale))
	p.tabs = append(p.tabs, t)
	p.front = t
	p.showFront()
	b.sendTabs(p)
}

func (t *tab) navigate(url string) {
	u := C.CString(url)
	defer C.free(unsafe.Pointer(u))
	C.lich_tab_navigate(t.c, u)
}

func (b *browser) close(o browserwire.PageParams) (any, error) {
	p, err := b.page(o.Page)
	if err != nil {
		return nil, err
	}
	b.settle(p)
	if p.poppedOut() {
		C.lich_page_pop_in(p.c)
	}
	for _, t := range p.tabs {
		b.closeDevtools(t)
		b.dropTab(t)
	}
	C.lich_page_free(p.c)
	delete(b.pages, p.id)
	delete(b.byKey, p.key)
	return nil, nil
}

// settle answers what p waits on, as dismissed: a dialog or chooser left
// open holds its web process.
func (b *browser) settle(p *page) {
	if p.dialog != nil {
		C.lich_dialog_answer(p.dialog, 0, nil)
		p.dialog = nil
	}
	p.cancelChooser()
}

func (b *browser) dropTab(t *tab) {
	delete(b.tabs, t.key)
	C.lich_tab_close(t.c)
}

func (b *browser) newTabReq(o browserwire.PageParams) (any, error) {
	p, err := b.page(o.Page)
	if err != nil {
		return nil, err
	}
	b.addTab(p, blankPage)
	return nil, nil
}

func (b *browser) tabOf(o browserwire.TabParams) (*page, *tab, error) {
	p, err := b.page(o.Page)
	if err != nil {
		return nil, nil, err
	}
	for _, t := range p.tabs {
		if t.id == o.Tab {
			return p, t, nil
		}
		if t.devtools != nil && t.devtools.id == o.Tab {
			return p, t.devtools, nil
		}
	}
	return nil, nil, fmt.Errorf("page %s has no tab %s", o.Page, o.Tab)
}

func (b *browser) selectTab(o browserwire.TabParams) (any, error) {
	p, t, err := b.tabOf(o)
	if err != nil {
		return nil, err
	}
	p.front, p.inspecting = t, false
	if t.inspects != nil {
		p.front, p.inspecting = t.inspects, true
	}
	p.showFront()
	b.sendTabs(p)
	return nil, nil
}

func (b *browser) closeTabReq(o browserwire.TabParams) (any, error) {
	_, t, err := b.tabOf(o)
	if err != nil {
		return nil, err
	}
	return nil, b.closeTab(t)
}

// closeTab closes t, putting the tab before it in front when t was.
func (b *browser) closeTab(t *tab) error {
	p := t.page
	if t.inspects != nil {
		b.closeDevtools(t.inspects)
		b.sendTabs(p)
		return nil
	}
	if len(p.tabs) == 1 {
		return errLastTab
	}
	b.closeDevtools(t)
	i := slices.Index(p.tabs, t)
	p.tabs = slices.Delete(p.tabs, i, i+1)
	if p.front == t {
		p.front = p.tabs[max(0, i-1)]
		p.showFront()
	}
	b.dropTab(t)
	b.sendTabs(p)
	return nil
}

func (b *browser) navigate(o browserwire.NavigateParams) (any, error) {
	p, err := b.page(o.Page)
	if err != nil {
		return nil, err
	}
	p.front.navigate(o.URL)
	return nil, nil
}

func (b *browser) zoom(o browserwire.ZoomParams) (any, error) {
	p, err := b.page(o.Page)
	if err != nil {
		return nil, err
	}
	if o.Scale <= 0 {
		return nil, fmt.Errorf("zoom %v is no scale", o.Scale)
	}
	p.scale = o.Scale
	b.relayout(p)
	return nil, nil
}

func (b *browser) size(o browserwire.SizeParams) (any, error) {
	p, err := b.page(o.Page)
	if err != nil {
		return nil, err
	}
	p.w, p.h = o.W, o.H
	b.relayout(p)
	return nil, nil
}

func (b *browser) relayout(p *page) {
	w, h := p.device()
	for _, t := range p.tabs {
		C.lich_tab_zoom(t.c, C.double(p.scale))
		C.lich_tab_resize(t.c, w, h)
		if t.devtools != nil {
			C.lich_tab_resize(t.devtools.c, w, h)
		}
	}
}

func (b *browser) sendTabs(p *page) {
	ev := browserwire.TabsEvent{Page: p.id, Tabs: make([]browserwire.Tab, 0, len(p.tabs))}
	for _, t := range p.tabs {
		xid := uint64(C.lich_tab_xid(t.c))
		if p.poppedOut() {
			xid = 0
		}
		ev.Tabs = append(ev.Tabs, browserwire.Tab{
			ID: t.id, URL: C.GoString(C.lich_tab_uri(t.c)), Title: C.GoString(C.lich_tab_title(t.c)),
			Loading: C.lich_tab_loading(t.c) != 0, Front: t == p.front && !p.inspecting, XID: xid, Icon: t.icon,
		})
		if t.devtools != nil {
			dx := uint64(C.lich_tab_xid(t.devtools.c))
			if p.poppedOut() {
				dx = 0
			}
			ev.Tabs = append(ev.Tabs, p.devtoolsTab(t, dx))
		}
	}
	b.wire.event(browserwire.EventTabs, ev)
}
