package main

import (
	"strings"
	"unicode/utf8"

	"github.com/lichdotdev/lich/browserwire"
)

// macKeyEvent is a DOM key as an NSEvent of macOS carries it: code is its
// virtual key code (Carbon's kVK_*, HIToolbox/Events.h), chars what it types
// and bare what it types with no modifier but Shift. A modifier key carries
// none and names the flag it holds instead.
type macKeyEvent struct {
	code        int
	chars, bare string
	modifier    browserwire.Mods
}

// noKeyCode is no key of the keyboard's, which WebKit tells as code
// "Unidentified".
const noKeyCode = 0xffff

// macNamedKeys are the DOM's key names that are no character: the key code,
// and what AppKit hands as its characters, an NSEvent.h function key for most.
var macNamedKeys = map[string]macKeyEvent{
	"Enter": {code: 0x24, chars: "\r"}, "Tab": {code: 0x30, chars: "\t"}, "Backspace": {code: 0x33, chars: "\x7f"},
	"Escape": {code: 0x35, chars: "\x1b"}, "Delete": {code: 0x75, chars: ""}, "Insert": {code: 0x72, chars: ""},
	"ArrowLeft": {code: 0x7b, chars: ""}, "ArrowUp": {code: 0x7e, chars: ""},
	"ArrowRight": {code: 0x7c, chars: ""}, "ArrowDown": {code: 0x7d, chars: ""},
	"Home": {code: 0x73, chars: ""}, "End": {code: 0x77, chars: ""},
	"PageUp": {code: 0x74, chars: ""}, "PageDown": {code: 0x79, chars: ""},
	"ContextMenu": {code: 0x6e, chars: ""},
	"F1":          {code: 0x7a, chars: ""}, "F2": {code: 0x78, chars: ""}, "F3": {code: 0x63, chars: ""},
	"F4": {code: 0x76, chars: ""}, "F5": {code: 0x60, chars: ""}, "F6": {code: 0x61, chars: ""},
	"F7": {code: 0x62, chars: ""}, "F8": {code: 0x64, chars: ""}, "F9": {code: 0x65, chars: ""},
	"F10": {code: 0x6d, chars: ""}, "F11": {code: 0x67, chars: ""}, "F12": {code: 0x6f, chars: ""},
	"Shift": {code: 0x38, modifier: browserwire.ModShift}, "Control": {code: 0x3b, modifier: browserwire.ModCtrl},
	"Alt": {code: 0x3a, modifier: browserwire.ModAlt}, "Meta": {code: 0x37, modifier: browserwire.ModMeta},
	"CapsLock": {code: 0x39},
}

// macKeyCodes are the key codes of the DOM's KeyboardEvent.code, on a US
// keyboard.
var macKeyCodes = map[string]int{
	"KeyA": 0x00, "KeyS": 0x01, "KeyD": 0x02, "KeyF": 0x03, "KeyH": 0x04, "KeyG": 0x05, "KeyZ": 0x06, "KeyX": 0x07,
	"KeyC": 0x08, "KeyV": 0x09, "KeyB": 0x0b, "KeyQ": 0x0c, "KeyW": 0x0d, "KeyE": 0x0e, "KeyR": 0x0f, "KeyY": 0x10,
	"KeyT": 0x11, "Digit1": 0x12, "Digit2": 0x13, "Digit3": 0x14, "Digit4": 0x15, "Digit6": 0x16, "Digit5": 0x17,
	"Equal": 0x18, "Digit9": 0x19, "Digit7": 0x1a, "Minus": 0x1b, "Digit8": 0x1c, "Digit0": 0x1d,
	"BracketRight": 0x1e, "KeyO": 0x1f, "KeyU": 0x20, "BracketLeft": 0x21, "KeyI": 0x22, "KeyP": 0x23, "KeyL": 0x25,
	"KeyJ": 0x26, "Quote": 0x27, "KeyK": 0x28, "Semicolon": 0x29, "Backslash": 0x2a, "Comma": 0x2b, "Slash": 0x2c,
	"KeyN": 0x2d, "KeyM": 0x2e, "Period": 0x2f, "Space": 0x31, "Backquote": 0x32,
}

// macKey is the NSEvent for a DOM key, its code from code or, without one,
// from the character; false for a key name it does not know.
func macKey(key, code string, mods browserwire.Mods) (macKeyEvent, bool) {
	if k, ok := macNamedKeys[key]; ok {
		k.bare = k.chars
		return k, true
	}
	r, size := utf8.DecodeRuneInString(key)
	if r == utf8.RuneError || size != len(key) {
		return macKeyEvent{}, false
	}
	k := macKeyEvent{code: noKeyCode, chars: key, bare: key}
	if c, ok := macKeyCodes[code]; ok {
		k.code = c
	} else if c, ok := macKeyCodes[codeOf(r)]; ok {
		k.code = c
	}
	// Control and a letter type that letter's control character, as the
	// keyboard does.
	if lower := strings.ToLower(key); mods&browserwire.ModCtrl != 0 && len(lower) == 1 && lower[0] >= 'a' && lower[0] <= 'z' {
		k.chars, k.bare = string(rune(lower[0]-'a'+1)), lower
	}
	return k, true
}

// codeOf is the KeyboardEvent.code a US keyboard types r with, for a letter,
// a digit or a space.
func codeOf(r rune) string {
	switch {
	case r >= 'a' && r <= 'z':
		return "Key" + strings.ToUpper(string(r))
	case r >= 'A' && r <= 'Z':
		return "Key" + string(r)
	case r >= '0' && r <= '9':
		return "Digit" + string(r)
	case r == ' ':
		return "Space"
	}
	return ""
}
