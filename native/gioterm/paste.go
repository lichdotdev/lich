package gioterm

/*
#include <stdlib.h>
#include <ghostty/vt.h>

static bool mode_on(GhosttyTerminal t, GhosttyMode mode) {
	GhosttyTerminalModeConfig c = {0};
	c.mode = mode;
	if (ghostty_terminal_get(t, GHOSTTY_TERMINAL_DATA_MODE, &c) != GHOSTTY_SUCCESS) return false;
	return c.value;
}

static bool bracketed_paste(GhosttyTerminal t) { return mode_on(t, GHOSTTY_MODE_BRACKETED_PASTE); }

static bool alternate_scroll(GhosttyTerminal t) {
	GhosttyTerminalScreen screen = GHOSTTY_TERMINAL_SCREEN_PRIMARY;
	ghostty_terminal_get(t, GHOSTTY_TERMINAL_DATA_ACTIVE_SCREEN, &screen);
	return screen == GHOSTTY_TERMINAL_SCREEN_ALTERNATE && mode_on(t, GHOSTTY_MODE_ALT_SCROLL);
}
*/
import "C"

import "unsafe"

// pasteFrame is the most bracketed paste adds around the text: ESC[200~ and
// ESC[201~.
const pasteFrame = 12

// Paste is what pasting text sends the program: wrapped in bracketed paste
// when the program turned it on, newlines as returns when not, and the
// control bytes that could end the paste early or inject a command replaced
// with spaces either way.
func (v *Terminal) Paste(text string) []byte {
	v.mu.Lock()
	if v.closed {
		v.mu.Unlock()
		return nil
	}
	bracketed := C.bracketed_paste(v.t)
	v.mu.Unlock()
	data := C.CBytes([]byte(text))
	defer C.free(data)
	out := make([]byte, len(text)+pasteFrame)
	var n C.size_t
	if C.ghostty_paste_encode((*C.char)(data), C.size_t(len(text)), bracketed,
		(*C.char)(unsafe.Pointer(&out[0])), C.size_t(len(out)), &n) != C.GHOSTTY_SUCCESS {
		return nil
	}
	return out[:n]
}

// AlternateScroll reports whether the wheel should move the cursor: a
// full-screen program (less, an editor) on the alternate screen that keeps
// mode 1007, on by default, and does not track the mouse itself.
func (v *Terminal) AlternateScroll() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed {
		return false
	}
	return bool(C.alternate_scroll(v.t))
}
