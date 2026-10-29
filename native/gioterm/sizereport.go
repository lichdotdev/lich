package gioterm

/*
#include <ghostty/vt.h>

extern bool goSizeReport(uintptr_t, GhosttySizeReportSize*);

static bool on_size(GhosttyTerminal t, void* ud, GhosttySizeReportSize* out) {
	return goSizeReport((uintptr_t)ud, out);
}

static GhosttyResult install_size_report(GhosttyTerminal t) {
	return ghostty_terminal_set(t, GHOSTTY_TERMINAL_OPT_SIZE, (const void*)on_size);
}
*/
import "C"

import "runtime/cgo"

// goSizeReport answers XTWINOPS (CSI 14/16/18 t) and mode 2048. Programs that
// draw Kitty images size them from the cell's pixels it reports; without it
// some send no image at all. It runs inside Write with v.mu held.
//
//export goSizeReport
func goSizeReport(h C.uintptr_t, out *C.GhosttySizeReportSize) C.bool {
	v := cgo.Handle(h).Value().(*Terminal)
	if v.cellW <= 0 || v.cellH <= 0 {
		return false
	}
	out.rows, out.columns = C.uint16_t(v.rows), C.uint16_t(v.cols)
	out.cell_width, out.cell_height = C.uint32_t(v.cellW), C.uint32_t(v.cellH)
	return true
}

func (v *Terminal) installSizeReport() C.GhosttyResult { return C.install_size_report(v.t) }
