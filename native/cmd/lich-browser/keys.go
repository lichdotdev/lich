package main

import (
	"unicode/utf8"

	"github.com/lichdotdev/lich/browserwire"
)

// namedKeys are GDK's keyvals (gdk/gdkkeysyms.h) for the DOM's key names
// that are no character.
var namedKeys = map[string]uint32{
	"Enter": 0xff0d, "Tab": 0xff09, "Backspace": 0xff08, "Escape": 0xff1b, "Delete": 0xffff, "Insert": 0xff63,
	"ArrowLeft": 0xff51, "ArrowUp": 0xff52, "ArrowRight": 0xff53, "ArrowDown": 0xff54,
	"Home": 0xff50, "End": 0xff57, "PageUp": 0xff55, "PageDown": 0xff56,
	"Shift": 0xffe1, "Control": 0xffe3, "Alt": 0xffe9, "Meta": 0xffeb, "CapsLock": 0xffe5, "ContextMenu": 0xff67,
	"F1": 0xffbe, "F2": 0xffbf, "F3": 0xffc0, "F4": 0xffc1, "F5": 0xffc2, "F6": 0xffc3,
	"F7": 0xffc4, "F8": 0xffc5, "F9": 0xffc6, "F10": 0xffc7, "F11": 0xffc8, "F12": 0xffc9,
}

// keyval is GDK's keyval for a DOM KeyboardEvent.key, unicode turning a
// character into one; zero for a key it does not know.
func keyval(key string, unicode func(rune) uint32) uint32 {
	if v, ok := namedKeys[key]; ok {
		return v
	}
	if r, size := utf8.DecodeRuneInString(key); r != utf8.RuneError && size == len(key) {
		return unicode(r)
	}
	return 0
}

// GDK's modifier and button masks (GdkModifierType).
const (
	gdkShift   = 1 << 0
	gdkControl = 1 << 2
	gdkAlt     = 1 << 3
	gdkButton1 = 1 << 8
	gdkMeta    = 1 << 28
)

func modState(m browserwire.Mods) uint32 {
	var s uint32
	for _, b := range []struct {
		mod  browserwire.Mods
		mask uint32
	}{{browserwire.ModShift, gdkShift}, {browserwire.ModCtrl, gdkControl}, {browserwire.ModAlt, gdkAlt}, {browserwire.ModMeta, gdkMeta}} {
		if m&b.mod != 0 {
			s |= b.mask
		}
	}
	return s
}

// gdkButton is X's button for a DOM MouseEvent.button: 0 main, 1 middle, 2
// secondary, then back and forward, which X numbers past its wheel's 4 to 7.
func gdkButton(dom int) uint32 {
	switch dom {
	case 0, 1, 2:
		return uint32(dom) + 1
	case 3, 4:
		return uint32(dom) + 5
	}
	return 1
}

// buttonMask is the state bit X sets while button is held, none for the
// buttons past the third, which X has no bit for.
func buttonMask(button uint32) uint32 {
	if button < 1 || button > 3 {
		return 0
	}
	return gdkButton1 << (button - 1)
}
