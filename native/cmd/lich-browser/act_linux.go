package main

/*
#include <stdlib.h>
#include "browser_linux.h"
*/
import "C"

import (
	"fmt"

	"github.com/lichdotdev/lich/browserwire"
)

// osPage is what the Linux engine adds to a page.
type osPage struct {
	// chooser is the file chooser the page waits on an answer for.
	chooser *C.LichChooser
	// buttons are the mouse buttons held down, GDK's state bits.
	buttons uint32
	// waiting are the wire's input events the page has yet to get.
	waiting []*waiter
	cast    casting
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
	// shortcut: Clicks is not passed on, WebKit counts a double click by the
	// time and distance between presses as it does a person's; a backend
	// that needs a count apart from timing needs a WebKit that takes one.
	button := gdkButton(o.Button)
	state := modState(o.Mods) | p.buttons
	if o.Kind == browserwire.MouseDown {
		C.lich_tab_focus(p.front.c)
	}
	C.lich_tab_mouse(p.front.c, kind, C.double(o.X), C.double(o.Y), C.guint(button), C.guint(state), C.double(o.DX), C.double(o.DY))
	switch o.Kind {
	case browserwire.MouseDown:
		p.buttons |= buttonMask(button)
	case browserwire.MouseUp:
		p.buttons &^= buttonMask(button)
	}
	return p, mouseEvents[o.Kind], nil
}

func unicodeKeyval(r rune) uint32 {
	return uint32(C.lich_keyval_for(C.gunichar(r)))
}

func (b *browser) key(o browserwire.KeyParams) (*page, string, error) {
	p, err := b.page(o.Page)
	if err != nil {
		return nil, "", err
	}
	v := keyval(o.Key, unicodeKeyval)
	if v == 0 {
		return nil, "", fmt.Errorf("no key %q", o.Key)
	}
	event, ok := keyEvents[o.Kind]
	if !ok {
		return nil, "", fmt.Errorf("no key event %q", o.Kind)
	}
	press := C.int(0)
	if o.Kind == browserwire.KeyDown {
		press = 1
	}
	C.lich_tab_focus(p.front.c)
	C.lich_tab_key(p.front.c, press, C.guint(v), C.guint(modState(o.Mods)))
	return p, event, nil
}
