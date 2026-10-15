package gioterm

/*
#include <stdlib.h>
#include <ghostty/vt.h>

static GhosttyResult viewport_ref(GhosttyTerminal t, uint16_t x, uint32_t y, GhosttyGridRef* ref) {
	GhosttyPoint p = {0};
	p.tag = GHOSTTY_POINT_TAG_VIEWPORT;
	p.value.coordinate.x = x;
	p.value.coordinate.y = y;
	*ref = (GhosttyGridRef)GHOSTTY_INIT_SIZED(GhosttyGridRef);
	return ghostty_terminal_grid_ref(t, p, ref);
}

// cell_under_pointer has a single click's drag run from the cell clicked to
// the cell under the pointer, both taken whole, in place of the selection
// Ghostty's 60% threshold leaves short at either end. A drag still inside the
// clicked cell, a word or line drag and a rectangle keep Ghostty's.
static void cell_under_pointer(GhosttyTerminal t, GhosttySelectionGesture g, GhosttyGridRef ref, GhosttySelection* sel, GhosttyResult* r) {
	if (*r == GHOSTTY_SUCCESS && sel->rectangle) return;
	if (*r != GHOSTTY_SUCCESS && *r != GHOSTTY_NO_VALUE) return;
	uint8_t clicks = 0;
	if (ghostty_selection_gesture_get(g, t, GHOSTTY_SELECTION_GESTURE_DATA_CLICK_COUNT, &clicks) != GHOSTTY_SUCCESS || clicks != 1) return;
	GhosttyGridRef anchor = GHOSTTY_INIT_SIZED(GhosttyGridRef);
	if (ghostty_selection_gesture_get(g, t, GHOSTTY_SELECTION_GESTURE_DATA_ANCHOR, &anchor) != GHOSTTY_SUCCESS) return;
	sel->start = anchor;
	sel->end = ref;
	sel->rectangle = false;
	*r = GHOSTTY_SUCCESS;
}

static GhosttyResult apply_gesture(GhosttyTerminal t, GhosttySelectionGesture g, GhosttySelectionGestureEventType type,
		uint16_t col, uint32_t row, double px, double py, uint64_t ns, uint32_t cols, uint32_t cell_w, uint32_t height,
		uint64_t repeat_interval_ns, double repeat_distance, bool left_clicked_cell) {
	GhosttyGridRef ref;
	GhosttyResult r = viewport_ref(t, col, row, &ref);
	if (r != GHOSTTY_SUCCESS) return r;
	GhosttySelectionGestureEvent ev;
	r = ghostty_selection_gesture_event_new(NULL, &ev, type);
	if (r != GHOSTTY_SUCCESS) return r;
	ghostty_selection_gesture_event_set(ev, GHOSTTY_SELECTION_GESTURE_EVENT_OPT_REF, &ref);
	if (type != GHOSTTY_SELECTION_GESTURE_EVENT_TYPE_RELEASE) {
		GhosttySurfacePosition pos = {px, py};
		ghostty_selection_gesture_event_set(ev, GHOSTTY_SELECTION_GESTURE_EVENT_OPT_POSITION, &pos);
	}
	if (type == GHOSTTY_SELECTION_GESTURE_EVENT_TYPE_PRESS) {
		ghostty_selection_gesture_event_set(ev, GHOSTTY_SELECTION_GESTURE_EVENT_OPT_TIME_NS, &ns);
		uint64_t interval = repeat_interval_ns;
		ghostty_selection_gesture_event_set(ev, GHOSTTY_SELECTION_GESTURE_EVENT_OPT_REPEAT_INTERVAL_NS, &interval);
		double distance = repeat_distance;
		ghostty_selection_gesture_event_set(ev, GHOSTTY_SELECTION_GESTURE_EVENT_OPT_REPEAT_DISTANCE, &distance);
	}
	if (type == GHOSTTY_SELECTION_GESTURE_EVENT_TYPE_DRAG) {
		GhosttySelectionGestureGeometry geo = {cols, cell_w, 0, height};
		ghostty_selection_gesture_event_set(ev, GHOSTTY_SELECTION_GESTURE_EVENT_OPT_GEOMETRY, &geo);
	}
	GhosttySelection sel = GHOSTTY_INIT_SIZED(GhosttySelection);
	r = ghostty_selection_gesture_event(g, t, ev, &sel);
	ghostty_selection_gesture_event_free(ev);
	if (type == GHOSTTY_SELECTION_GESTURE_EVENT_TYPE_DRAG && left_clicked_cell) {
		cell_under_pointer(t, g, ref, &sel, &r);
	}
	if (r == GHOSTTY_SUCCESS) {
		return ghostty_terminal_set(t, GHOSTTY_TERMINAL_OPT_SELECTION, &sel);
	}
	return r;
}

static GhosttyResult selection_text(GhosttyTerminal t, uint8_t** out, size_t* len) {
	GhosttyTerminalSelectionFormatOptions o = GHOSTTY_INIT_SIZED(GhosttyTerminalSelectionFormatOptions);
	o.emit = GHOSTTY_FORMATTER_FORMAT_PLAIN;
	o.unwrap = true;
	o.trim = true;
	return ghostty_terminal_selection_format_alloc(t, NULL, o, out, len);
}

static GhosttyMouseTrackingMode mouse_tracking(GhosttyTerminal t) {
	GhosttyMouseTrackingMode m = GHOSTTY_MOUSE_TRACKING_NONE;
	ghostty_terminal_get(t, GHOSTTY_TERMINAL_DATA_MOUSE_TRACKING, &m);
	return m;
}

static GhosttyResult encode_mouse(GhosttyMouseEncoder enc, GhosttyMouseEvent ev, GhosttyTerminal t,
		GhosttyMouseAction action, GhosttyMouseButton button, GhosttyMods mods, float x, float y,
		uint32_t cols, uint32_t rows, uint32_t cell_w, uint32_t cell_h, bool any_pressed,
		char* buf, size_t cap, size_t* n) {
	ghostty_mouse_encoder_setopt_from_terminal(enc, t);
	GhosttyMouseEncoderSize size = GHOSTTY_INIT_SIZED(GhosttyMouseEncoderSize);
	size.screen_width = cols * cell_w;
	size.screen_height = rows * cell_h;
	size.cell_width = cell_w;
	size.cell_height = cell_h;
	ghostty_mouse_encoder_setopt(enc, GHOSTTY_MOUSE_ENCODER_OPT_SIZE, &size);
	ghostty_mouse_encoder_setopt(enc, GHOSTTY_MOUSE_ENCODER_OPT_ANY_BUTTON_PRESSED, &any_pressed);
	bool track_cell = true;
	ghostty_mouse_encoder_setopt(enc, GHOSTTY_MOUSE_ENCODER_OPT_TRACK_LAST_CELL, &track_cell);
	ghostty_mouse_event_set_action(ev, action);
	if (button == GHOSTTY_MOUSE_BUTTON_UNKNOWN) {
		ghostty_mouse_event_clear_button(ev);
	} else {
		ghostty_mouse_event_set_button(ev, button);
	}
	ghostty_mouse_event_set_mods(ev, mods);
	GhosttyMousePosition pos = {x, y};
	ghostty_mouse_event_set_position(ev, pos);
	return ghostty_mouse_encoder_encode(enc, ev, buf, cap, n);
}
*/
import "C"

import (
	"time"
	"unsafe"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
)

// MouseAction is what a pointer event reported to the program did.
type MouseAction int

const (
	MousePress   MouseAction = C.GHOSTTY_MOUSE_ACTION_PRESS
	MouseRelease MouseAction = C.GHOSTTY_MOUSE_ACTION_RELEASE
	MouseMotion  MouseAction = C.GHOSTTY_MOUSE_ACTION_MOTION
)

// MouseButton is the button of a reported pointer event. Wheel steps are
// presses of MouseWheelUp and MouseWheelDown, as the protocol spells them.
type MouseButton int

const (
	MouseNoButton  MouseButton = C.GHOSTTY_MOUSE_BUTTON_UNKNOWN
	MouseLeft      MouseButton = C.GHOSTTY_MOUSE_BUTTON_LEFT
	MouseRight     MouseButton = C.GHOSTTY_MOUSE_BUTTON_RIGHT
	MouseMiddle    MouseButton = C.GHOSTTY_MOUSE_BUTTON_MIDDLE
	MouseWheelUp   MouseButton = C.GHOSTTY_MOUSE_BUTTON_FOUR
	MouseWheelDown MouseButton = C.GHOSTTY_MOUSE_BUTTON_FIVE
)

// maxMouseReplyLen fits the longest report, an SGR-pixels one with five-digit
// coordinates.
const maxMouseReplyLen = 64

// MouseButtonOf is the reported button a Gio pointer button stands for.
func MouseButtonOf(b pointer.Buttons) MouseButton {
	switch {
	case b.Contain(pointer.ButtonPrimary):
		return MouseLeft
	case b.Contain(pointer.ButtonSecondary):
		return MouseRight
	case b.Contain(pointer.ButtonTertiary):
		return MouseMiddle
	}
	return MouseNoButton
}

// MouseTracking reports whether the program asked for pointer events (any of
// the 9, 1000, 1002, 1003 modes).
func (v *Terminal) MouseTracking() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return C.mouse_tracking(v.t) != C.GHOSTTY_MOUSE_TRACKING_NONE
}

// EncodeMouse is what the program expects for a pointer event at pixel
// (x, y) of the grid, in the tracking mode and format it asked for; nil when
// its mode does not report this event. anyPressed says a button is held,
// which decides whether a motion is a drag.
func (v *Terminal) EncodeMouse(action MouseAction, button MouseButton, mods key.Modifiers, x, y float32, anyPressed bool) []byte {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.mouseEnc == nil {
		if C.ghostty_mouse_encoder_new(nil, &v.mouseEnc) != C.GHOSTTY_SUCCESS {
			return nil
		}
		if C.ghostty_mouse_event_new(nil, &v.mouseEv) != C.GHOSTTY_SUCCESS {
			return nil
		}
	}
	var buf [maxMouseReplyLen]C.char
	var n C.size_t
	r := C.encode_mouse(v.mouseEnc, v.mouseEv, v.t, C.GhosttyMouseAction(action), C.GhosttyMouseButton(button),
		C.GhosttyMods(ghosttyMods(mods)), C.float(x), C.float(y), C.uint32_t(v.cols), C.uint32_t(v.rows),
		C.uint32_t(max(v.cellW, 1)), C.uint32_t(max(v.cellH, 1)), C.bool(anyPressed), &buf[0], C.size_t(len(buf)), &n)
	if r != C.GHOSTTY_SUCCESS || n == 0 {
		return nil
	}
	return C.GoBytes(unsafe.Pointer(&buf[0]), C.int(n))
}

// A press this soon and this close (in pixels) to the one before counts as
// its double or triple click. GTK's defaults, gtk-double-click-time and
// gtk-double-click-distance: Gio reads neither from the desktop.
const (
	repeatClickInterval = 400 * time.Millisecond
	repeatClickDistance = 5
)

// SelectPress starts a selection at pixel (x, y): a click selects from the
// cell, a second and third click within the repeat interval the word and the
// line. at is the event's monotonic time.
func (v *Terminal) SelectPress(x, y float32, at time.Duration) {
	v.gesture(C.GHOSTTY_SELECTION_GESTURE_EVENT_TYPE_PRESS, x, y, at)
}

// SelectDrag extends the selection a press started to pixel (x, y).
func (v *Terminal) SelectDrag(x, y float32) {
	v.gesture(C.GHOSTTY_SELECTION_GESTURE_EVENT_TYPE_DRAG, x, y, 0)
}

// SelectRelease ends the drag at pixel (x, y).
func (v *Terminal) SelectRelease(x, y float32) {
	v.gesture(C.GHOSTTY_SELECTION_GESTURE_EVENT_TYPE_RELEASE, x, y, 0)
}

func (v *Terminal) gesture(kind C.GhosttySelectionGestureEventType, x, y float32, at time.Duration) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.gest == nil && C.ghostty_selection_gesture_new(nil, &v.gest) != C.GHOSTTY_SUCCESS {
		return
	}
	col, row := v.cellAt(x, y)
	if kind == C.GHOSTTY_SELECTION_GESTURE_EVENT_TYPE_PRESS {
		v.pressCell = [2]int{col, row}
	}
	left := v.pressCell != [2]int{col, row}
	C.apply_gesture(v.t, v.gest, kind, C.uint16_t(col), C.uint32_t(row), C.double(x), C.double(y),
		C.uint64_t(at.Nanoseconds()), C.uint32_t(v.cols), C.uint32_t(max(v.cellW, 1)), C.uint32_t(v.rows*max(v.cellH, 1)),
		C.uint64_t(repeatClickInterval.Nanoseconds()), C.double(repeatClickDistance), C.bool(left))
}

// cellAt is the viewport cell under pixel (x, y), clamped to the grid so a
// drag past an edge selects up to it.
func (v *Terminal) cellAt(x, y float32) (col, row int) {
	col = min(max(int(x)/max(v.cellW, 1), 0), v.cols-1)
	row = min(max(int(y)/max(v.cellH, 1), 0), v.rows-1)
	return col, row
}

// ClearSelection drops the selection.
func (v *Terminal) ClearSelection() {
	v.mu.Lock()
	defer v.mu.Unlock()
	C.ghostty_terminal_set(v.t, C.GHOSTTY_TERMINAL_OPT_SELECTION, nil)
}

// SelectionText is the selected text, soft-wrapped lines joined and trailing
// blanks trimmed; "" without a selection.
func (v *Terminal) SelectionText() string {
	v.mu.Lock()
	defer v.mu.Unlock()
	var out *C.uint8_t
	var n C.size_t
	if C.selection_text(v.t, &out, &n) != C.GHOSTTY_SUCCESS {
		return ""
	}
	defer C.ghostty_free(nil, out, n)
	return C.GoStringN((*C.char)(unsafe.Pointer(out)), C.int(n))
}
