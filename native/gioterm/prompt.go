package gioterm

/*
#include <stdio.h>
#include <string.h>
#include <ghostty/vt.h>

static bool active_ref(GhosttyTerminal t, uint32_t y, uint16_t x, GhosttyGridRef* ref) {
	GhosttyPoint p = {0};
	p.tag = GHOSTTY_POINT_TAG_ACTIVE;
	p.value.coordinate.x = x;
	p.value.coordinate.y = y;
	*ref = (GhosttyGridRef)GHOSTTY_INIT_SIZED(GhosttyGridRef);
	return ghostty_terminal_grid_ref(t, p, ref) == GHOSTTY_SUCCESS;
}

static bool row_get(GhosttyTerminal t, uint32_t y, GhosttyRowData data, void* out) {
	GhosttyGridRef ref;
	GhosttyRow row;
	if (!active_ref(t, y, 0, &ref) || ghostty_grid_ref_row(&ref, &row) != GHOSTTY_SUCCESS) return false;
	return ghostty_row_get(row, data, out) == GHOSTTY_SUCCESS;
}

static GhosttyRowSemanticPrompt row_prompt(GhosttyTerminal t, uint32_t y) {
	GhosttyRowSemanticPrompt s = GHOSTTY_ROW_SEMANTIC_NONE;
	row_get(t, y, GHOSTTY_ROW_DATA_SEMANTIC_PROMPT, &s);
	return s;
}

static bool row_flag(GhosttyTerminal t, uint32_t y, GhosttyRowData data) {
	bool v = false;
	row_get(t, y, data, &v);
	return v;
}

static bool row_blank(GhosttyTerminal t, uint32_t y, uint16_t cols) {
	for (uint16_t x = 0; x < cols; x++) {
		GhosttyGridRef ref;
		GhosttyCell cell;
		bool text = false;
		if (!active_ref(t, y, x, &ref) || ghostty_grid_ref_cell(&ref, &cell) != GHOSTTY_SUCCESS) return false;
		ghostty_cell_get(cell, GHOSTTY_CELL_DATA_HAS_TEXT, &text);
		if (text) return false;
	}
	return true;
}

// clear_wrapped_prompt_heads finishes the prompt clearing a resize does for a
// shell that redraws its prompt (OSC 133;A;redraw=1). A prompt line that the
// resize wrapped leaves its wrapped rows all marked as a prompt start, so the
// terminal clears from the last of them down and keeps the rows above: those
// are erased here, once the terminal has shown it cleared the rest, which is
// also what tells a redrawing shell from one that keeps its prompt.
static void clear_wrapped_prompt_heads(GhosttyTerminal t, uint16_t cols) {
	bool at_prompt = false;
	uint16_t cx = 0, cy = 0;
	ghostty_terminal_get(t, GHOSTTY_TERMINAL_DATA_CURSOR_AT_PROMPT, &at_prompt);
	if (!at_prompt) return;
	ghostty_terminal_get(t, GHOSTTY_TERMINAL_DATA_CURSOR_X, &cx);
	ghostty_terminal_get(t, GHOSTTY_TERMINAL_DATA_CURSOR_Y, &cy);
	int start = cy;
	while (start >= 0 && row_prompt(t, start) == GHOSTTY_ROW_SEMANTIC_PROMPT_CONTINUATION) start--;
	if (start < 0 || row_prompt(t, start) != GHOSTTY_ROW_SEMANTIC_PROMPT) return;
	if (!row_flag(t, start, GHOSTTY_ROW_DATA_WRAP_CONTINUATION) || !row_blank(t, start, cols)) return;
	int top = start;
	while (top > 0 && row_flag(t, top - 1, GHOSTTY_ROW_DATA_WRAP)) top--;
	char seq[32];
	for (int y = top; y < start; y++) {
		int n = snprintf(seq, sizeof seq, "\x1b[%d;1H\x1b[2K", y + 1);
		ghostty_terminal_vt_write(t, (const uint8_t*)seq, (size_t)n);
	}
	int n = snprintf(seq, sizeof seq, "\x1b[%d;%dH", cy + 1, cx + 1);
	ghostty_terminal_vt_write(t, (const uint8_t*)seq, (size_t)n);
}
*/
import "C"

// clearWrappedPromptHeads works around a libghostty-vt resize that clears a
// redrawable prompt only from the last row its wrapped first line spans; see
// clear_wrapped_prompt_heads. The caller holds v.mu.
func (v *Terminal) clearWrappedPromptHeads() {
	C.clear_wrapped_prompt_heads(v.t, C.uint16_t(v.cols))
}
