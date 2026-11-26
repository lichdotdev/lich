package main

import (
	"testing"

	"github.com/lichdotdev/lich/browserwire"
)

func TestMacKeyIsTheNSEventOfADOMKey(t *testing.T) {
	for _, tc := range []struct {
		name, key, code string
		mods            browserwire.Mods
		want            macKeyEvent
	}{
		{"a named key", "Enter", "Enter", 0, macKeyEvent{code: 0x24, chars: "\r", bare: "\r"}},
		{"an arrow, AppKit's function key", "ArrowUp", "", 0, macKeyEvent{code: 0x7e, chars: "", bare: ""}},
		{"a letter by its code", "a", "KeyA", 0, macKeyEvent{code: 0x00, chars: "a", bare: "a"}},
		{"a letter with no code", "z", "", 0, macKeyEvent{code: 0x06, chars: "z", bare: "z"}},
		{"a capital, shifted", "B", "KeyB", browserwire.ModShift, macKeyEvent{code: 0x0b, chars: "B", bare: "B"}},
		{"control and a letter", "c", "KeyC", browserwire.ModCtrl, macKeyEvent{code: 0x08, chars: "\x03", bare: "c"}},
		{"a character no key types", "ç", "", 0, macKeyEvent{code: noKeyCode, chars: "ç", bare: "ç"}},
		{"a modifier, as the flag it holds", "Shift", "ShiftLeft", 0, macKeyEvent{code: 0x38, modifier: browserwire.ModShift}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := macKey(tc.key, tc.code, tc.mods)
			if !ok || got != tc.want {
				t.Fatalf("macKey(%q, %q) = %+v, %v; want %+v", tc.key, tc.code, got, ok, tc.want)
			}
		})
	}
	if _, ok := macKey("Unknown", "", 0); ok {
		t.Fatal("a key name nobody knows made a key")
	}
}
