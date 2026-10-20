package gioterm

/*
#include <ghostty/vt.h>

extern bool goColorSchemeDark(uintptr_t);

static bool on_color_scheme(GhosttyTerminal t, void* ud, GhosttyColorScheme* out) {
	*out = goColorSchemeDark((uintptr_t)ud) ? GHOSTTY_COLOR_SCHEME_DARK : GHOSTTY_COLOR_SCHEME_LIGHT;
	return true;
}

static GhosttyResult install_color_scheme(GhosttyTerminal t) {
	return ghostty_terminal_set(t, GHOSTTY_TERMINAL_OPT_COLOR_SCHEME, (const void*)on_color_scheme);
}

static bool scheme_reporting(GhosttyTerminal t) {
	GhosttyTerminalModeConfig c = {0};
	c.mode = GHOSTTY_MODE_COLOR_SCHEME_REPORT;
	if (ghostty_terminal_get(t, GHOSTTY_TERMINAL_DATA_MODE, &c) != GHOSTTY_SUCCESS) return false;
	return c.value;
}
*/
import "C"

import (
	"image/color"
	"runtime/cgo"
	"unsafe"
)

// maxSchemeReportLen fits ESC[?997;1n.
const maxSchemeReportLen = 16

//export goColorSchemeDark
func goColorSchemeDark(h C.uintptr_t) C.bool {
	return C.bool(cgo.Handle(h).Value().(*Terminal).dark.Load())
}

// installColorScheme has DSR 996 answered from dark.
func (v *Terminal) installColorScheme() C.GhosttyResult { return C.install_color_scheme(v.t) }

// isDark is whether a background reads as a dark scheme: its luminance below
// the middle of the range.
func isDark(bg color.NRGBA) bool {
	const mid = 128
	return 0.2126*float64(bg.R)+0.7152*float64(bg.G)+0.0722*float64(bg.B) < mid
}

// setScheme takes the scheme of background bg: what DSR 996 answers from
// now on, and, when it changed and the program turned mode 2031 on, the
// report that tells it so. Claude Code's "auto" theme reads it to follow
// lich's own light and dark. Called with v.mu held.
func (v *Terminal) setScheme(bg color.NRGBA) {
	dark := isDark(bg)
	changed := v.schemeSet && v.dark.Load() != dark
	v.dark.Store(dark)
	v.schemeSet = true
	if !changed || !bool(C.scheme_reporting(v.t)) {
		return
	}
	scheme := C.GhosttyColorScheme(C.GHOSTTY_COLOR_SCHEME_LIGHT)
	if dark {
		scheme = C.GHOSTTY_COLOR_SCHEME_DARK
	}
	var buf [maxSchemeReportLen]C.char
	var n C.size_t
	if C.ghostty_color_scheme_report_encode(scheme, &buf[0], C.size_t(len(buf)), &n) != C.GHOSTTY_SUCCESS {
		return
	}
	v.replies(C.GoBytes(unsafe.Pointer(&buf[0]), C.int(n)))
}
