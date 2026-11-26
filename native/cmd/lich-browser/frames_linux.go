package main

/*
#include "browser.h"
*/
import "C"

import (
	"encoding/base64"
	"errors"
	"log"
	"unsafe"

	"github.com/lichdotdev/lich/browserwire"
	"github.com/lichdotdev/lich/native/internal/turbojpeg"
)

// Off screen (no X display), the window draws a page from the frames its
// shown tab sends while a screencast runs (docs/lich-browser.md).

// frameQuality is the frames' JPEG quality, the one Chrome's screencast had
// (internal/browser screencastQuality).
const frameQuality = 85

var errNoFrames = errors.New("this lich-browser's pages are X windows the window embeds: it sends no frames")

// casting is what a page's screencast keeps.
type casting struct {
	on bool
	// encoding: a frame is on its way; dirty: the shown tab drew again
	// meanwhile, and is captured once it is out.
	encoding, dirty bool
	// unreadable is logged once: GTK gave a surface that is no image.
	unreadable bool
}

func init() {
	handlers[browserwire.Screencast] = now((*browser).screencast)
	handlers[browserwire.StopScreencast] = now((*browser).stopScreencast)
}

func drawsOffscreen() bool { return C.lich_browser_offscreen() != 0 }

func (b *browser) screencast(o browserwire.ScreencastParams) (any, error) {
	if !drawsOffscreen() {
		return nil, errNoFrames
	}
	p, err := b.page(o.Page)
	if err != nil {
		return nil, err
	}
	p.w, p.h = o.W, o.H
	if o.Scale > 0 {
		p.scale = o.Scale
	}
	b.relayout(p)
	p.cast.on = true
	p.sendFrame()
	return nil, nil
}

func (b *browser) stopScreencast(o browserwire.PageParams) (any, error) {
	p, err := b.page(o.Page)
	if err != nil {
		return nil, err
	}
	p.cast.on = false
	return nil, nil
}

// shown is the tab the pane shows: the front one, or its inspector.
func (p *page) shown() *tab {
	if p.inspecting && p.front.devtools != nil {
		return p.front.devtools
	}
	return p.front
}

// frameShown sends the shown tab as the tabs change: one just brought to
// the front may not draw again for a while.
func (p *page) frameShown() { p.sendFrame() }

//export lichDamaged
func lichDamaged(key C.uintptr_t) {
	t := the.tabs[uintptr(key)]
	if t != nil && t.page.shown() == t {
		t.page.sendFrame()
	}
}

// sendFrame captures the shown tab and sends it once encoded, off GTK's
// thread; a frame asked for meanwhile is captured once that one is out.
func (p *page) sendFrame() {
	if !p.cast.on || p.poppedOut() || the.pages[p.id] != p {
		return
	}
	if p.cast.encoding {
		p.cast.dirty = true
		return
	}
	t := p.shown()
	var w, h, stride C.int
	data := C.lich_tab_frame(t.c, &w, &h, &stride)
	if data == nil {
		if !p.cast.unreadable {
			p.cast.unreadable = true
			log.Printf("page %s: the tab's surface is no image, no frames", p.id)
		}
		return
	}
	pixels := C.GoBytes(unsafe.Pointer(data), stride*h)
	p.cast.encoding = true
	go func() {
		jpeg, err := turbojpeg.EncodeBGRX(pixels, int(w), int(h), int(stride), frameQuality)
		onMain(func() { p.framed(t, jpeg, err) })
	}()
}

// framed sends what sendFrame encoded, and captures again when the tab drew
// meanwhile.
func (p *page) framed(t *tab, jpeg []byte, err error) {
	p.cast.encoding = false
	if err != nil {
		log.Printf("page %s: encode a frame: %v", p.id, err)
	} else if p.cast.on && the.pages[p.id] == p {
		the.wire.event(browserwire.EventFrame, browserwire.FrameEvent{Page: p.id, Tab: t.id, Data: base64.StdEncoding.EncodeToString(jpeg)})
	}
	if p.cast.dirty {
		p.cast.dirty = false
		p.sendFrame()
	}
}
