package main

import (
	"gioui.org/io/key"

	"github.com/omartelo/lich/native/gioterm"
)

// terminalKeyBytes is what a key press sends to a session: the chords of the
// "Passed through to the agent" group as the bytes the agent binds, the same
// the web window sends (frontend/src/lib/terminal/term-keys.ts), and every
// other key as the terminal's encoder spells it.
func terminalKeyBytes(vt *gioterm.Terminal, e key.Event, goos string) []byte {
	if b := passedThroughChord(e, goos); b != "" {
		return []byte(b)
	}
	return vt.HandleKey(e)
}

func passedThroughChord(e key.Event, goos string) string {
	switch {
	case e.Modifiers == key.ModCtrl && e.Name == key.NameDeleteBackward:
		return "\x17"
	case e.Modifiers == key.ModCtrl && e.Name == "V":
		return imageAttach(goos)
	case e.Modifiers == key.ModShift && (e.Name == key.NameReturn || e.Name == key.NameEnter):
		return "\x1b\r"
	}
	return ""
}

// imageAttach is the key the agent reads the clipboard's image on. Claude
// Code binds it to Alt+V on Windows, where Ctrl+V is its own paste.
func imageAttach(goos string) string {
	if goos == "windows" {
		return "\x1bv"
	}
	return "\x16"
}

// isPasteChord is the clipboard paste: Ctrl+Shift+V, as xterm.js has it in the
// web window, and Cmd+V on macOS.
func isPasteChord(e key.Event, goos string) bool {
	if e.Name != "V" {
		return false
	}
	return e.Modifiers == key.ModCtrl|key.ModShift || goos == "darwin" && e.Modifiers == key.ModCommand
}

// pastedClipboard is what a paste of the clipboard's text sends. A clipboard
// with no text holds an image or a file, which the agent reads itself on the
// image key, as the web window's paste of files does (pastedImageSequence).
func pastedClipboard(vt *gioterm.Terminal, text, goos string) []byte {
	if text == "" {
		return []byte(imageAttach(goos))
	}
	return vt.Paste(text)
}
