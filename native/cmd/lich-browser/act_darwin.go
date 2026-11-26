package main

/*
#include <stdlib.h>
#include "browser.h"
*/
import "C"

import (
	"fmt"
	"math/bits"
	"unsafe"

	"github.com/lichdotdev/lich/browserwire"
)

// osPage is what the WKWebView engine adds to a page.
type osPage struct {
	// chooser is the file chooser the page waits on an answer for.
	chooser *C.LichChooser
	// buttons are the DOM's mouse buttons held down, a bit each.
	buttons uint32
	// waiting are the wire's input events the page has yet to get.
	waiting []*waiter
}

type osTab struct{}

var mouseKinds = map[string]C.int{browserwire.MouseMove: 0, browserwire.MouseDown: 1, browserwire.MouseUp: 2, browserwire.MouseWheel: 3}

// held is the button a move drags with, the lowest held; -1 for none.
func (p *page) held() int {
	if p.buttons == 0 {
		return -1
	}
	return bits.TrailingZeros32(p.buttons)
}

func (b *browser) mouse(o browserwire.MouseParams) (*page, string, error) {
	p, err := b.page(o.Page)
	if err != nil {
		return nil, "", err
	}
	kind, ok := mouseKinds[o.Kind]
	if !ok {
		return nil, "", fmt.Errorf("no mouse event %q", o.Kind)
	}
	button, clicks := o.Button, max(o.Clicks, 1)
	switch o.Kind {
	case browserwire.MouseMove:
		button = p.held()
	case browserwire.MouseDown:
		C.lich_tab_focus(p.front.c)
		p.buttons |= 1 << o.Button
	case browserwire.MouseUp:
		p.buttons &^= 1 << o.Button
	}
	C.lich_tab_mouse(p.front.c, kind, C.double(o.X), C.double(o.Y), C.int(button), C.int(clicks), C.int(o.Mods), C.double(o.DX), C.double(o.DY))
	return p, mouseEvents[o.Kind], nil
}

func (b *browser) key(o browserwire.KeyParams) (*page, string, error) {
	p, err := b.page(o.Page)
	if err != nil {
		return nil, "", err
	}
	k, ok := macKey(o.Key, o.Code, o.Mods)
	if !ok {
		return nil, "", fmt.Errorf("no key %q", o.Key)
	}
	event, ok := keyEvents[o.Kind]
	if !ok {
		return nil, "", fmt.Errorf("no key event %q", o.Kind)
	}
	press, mods := C.int(0), o.Mods
	if o.Kind == browserwire.KeyDown {
		press = 1
	}
	// A modifier is a change of the flags, which it is held in.
	if k.modifier != 0 && press == 1 {
		mods |= k.modifier
	} else {
		mods &^= k.modifier
	}
	chars, bare := cOrNil(k.chars, k.modifier == 0), cOrNil(k.bare, k.modifier == 0)
	defer C.free(unsafe.Pointer(chars))
	defer C.free(unsafe.Pointer(bare))
	C.lich_tab_focus(p.front.c)
	C.lich_tab_key(p.front.c, press, C.int(k.code), chars, bare, C.int(mods))
	return p, event, nil
}
