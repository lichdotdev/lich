package main

import (
	"gioui.org/io/key"

	"github.com/lichdotdev/lich/native/gioterm"
)

// terminalKeyBytes is what a key press sends to a session: the chords of the
// "Passed through to the agent" group as the bytes the agent binds, nothing
// for Ctrl+Z, and every other key as the terminal's encoder spells it.
func terminalKeyBytes(vt *gioterm.Terminal, e key.Event, goos string) []byte {
	if isSuspendChord(e) {
		return nil
	}
	if b := passedThroughChord(e, goos); b != "" {
		return []byte(b)
	}
	return vt.HandleKey(e)
}

// isSuspendChord is Ctrl+Z, which stops the agent (SIGTSTP from the line
// discipline, or the agent's own suspend under the kitty protocol). The agent
// is the PTY's only process, with no shell to send the SIGCONT back, so one
// stray press left the session frozen for good.
func isSuspendChord(e key.Event) bool {
	return e.Modifiers == key.ModCtrl && e.Name == "Z"
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

// isPasteChord is the clipboard paste: Ctrl+Shift+V, and Cmd+V on macOS.
func isPasteChord(e key.Event, goos string) bool {
	if e.Name != "V" {
		return false
	}
	return e.Modifiers == key.ModCtrl|key.ModShift || goos == "darwin" && e.Modifiers == key.ModCommand
}

// isCopyChord is the clipboard copy: Ctrl+Shift+C, and Cmd+C on macOS.
func isCopyChord(e key.Event, goos string) bool {
	if e.Name != "C" {
		return false
	}
	return e.Modifiers == key.ModCtrl|key.ModShift || goos == "darwin" && e.Modifiers == key.ModCommand
}

// pastedClipboard is what a paste of the clipboard's text sends. A clipboard
// with no text holds an image or a file, which the agent reads itself on the
// image key.
func pastedClipboard(vt *gioterm.Terminal, text, goos string) []byte {
	if text == "" {
		return []byte(imageAttach(goos))
	}
	return vt.Paste(text)
}
