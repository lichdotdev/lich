package main

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/omartelo/lich/native/gioterm"
	"github.com/omartelo/lich/native/ui"
)

// pointer handles a pointer event on t's grid: a program that tracks the
// mouse gets it reported, otherwise a drag or a double or triple click
// selects. Shift selects even under tracking, as xterm.js lets it in the web
// window. send is what goes to the program; copied is the text a selection
// ended with, for the clipboard.
func (t *termView) pointer(e pointer.Event, cellH int) (send []byte, copied string) {
	if e.Kind == pointer.Scroll {
		return t.wheel(e, cellH), ""
	}
	reported := t.vt.MouseTracking() && !e.Modifiers.Contain(key.ModShift)
	if reported && !t.selecting {
		return t.report(e), ""
	}
	x, y := e.Position.X, e.Position.Y
	switch e.Kind {
	case pointer.Press:
		if !e.Buttons.Contain(pointer.ButtonPrimary) {
			return nil, ""
		}
		t.selecting = true
		t.vt.ClearSelection()
		t.vt.SelectPress(x, y, e.Time)
	case pointer.Drag:
		if t.selecting {
			t.vt.SelectDrag(x, y)
		}
	case pointer.Release:
		if !t.selecting {
			return nil, ""
		}
		t.selecting = false
		t.vt.SelectRelease(x, y)
		return nil, t.vt.SelectionText()
	}
	return nil, ""
}

// linkClick is the link a Ctrl+click (Cmd+click on macOS) opens, as
// linkClickIsOurs rules in the web window, whether the program tracks the
// mouse or not. took is e kept from the program and the selection: the press
// that found a link and its release.
func (t *termView) linkClick(e pointer.Event) (url string, took bool) {
	switch e.Kind {
	case pointer.Press:
		if !e.Buttons.Contain(pointer.ButtonPrimary) || !e.Modifiers.Contain(key.ModShortcut) {
			return "", false
		}
		l, ok := t.vt.LinkAt(e.Position.X, e.Position.Y)
		t.linkHeld = ok
		return l.URL, ok
	case pointer.Release:
		if t.linkHeld {
			t.linkHeld = false
			return "", true
		}
	}
	return "", false
}

// report is e as the tracking program reads it. A release carries no
// button in Gio, so the one pressed is remembered.
func (t *termView) report(e pointer.Event) []byte {
	x, y := e.Position.X, e.Position.Y
	switch e.Kind {
	case pointer.Press:
		t.held = gioterm.MouseButtonOf(e.Buttons)
		return t.vt.EncodeMouse(gioterm.MousePress, t.held, e.Modifiers, x, y, true)
	case pointer.Release:
		b := t.held
		t.held = gioterm.MouseNoButton
		return t.vt.EncodeMouse(gioterm.MouseRelease, b, e.Modifiers, x, y, false)
	case pointer.Drag:
		return t.vt.EncodeMouse(gioterm.MouseMotion, t.held, e.Modifiers, x, y, true)
	case pointer.Move:
		return t.vt.EncodeMouse(gioterm.MouseMotion, gioterm.MouseNoButton, e.Modifiers, x, y, false)
	}
	return nil
}

// wheel scrolls a row per cell height of e: as wheel steps for a program
// tracking the mouse, as arrow keys for a full-screen one that does not,
// through scrollback otherwise.
func (t *termView) wheel(e pointer.Event, cellH int) []byte {
	rows := int(e.Scroll.Y) / max(cellH, 1)
	tracking := t.vt.MouseTracking()
	if !tracking && !t.vt.AlternateScroll() {
		t.vt.Scroll(rows)
		return nil
	}
	button, arrow := gioterm.MouseWheelDown, key.NameDownArrow
	if rows < 0 {
		button, arrow, rows = gioterm.MouseWheelUp, key.NameUpArrow, -rows
	}
	var out []byte
	for range rows {
		if tracking {
			out = append(out, t.vt.EncodeMouse(gioterm.MousePress, button, e.Modifiers, e.Position.X, e.Position.Y, false)...)
		} else {
			out = append(out, t.vt.HandleKey(key.Event{Name: arrow})...)
		}
	}
	return out
}

// ponytail: English until the native window has i18n.
const (
	terminalCopiedOne   = "copied %d char to clipboard"
	terminalCopiedOther = "copied %d chars to clipboard"
	terminalCopyToastID = "terminal-copy"
	// copy-toast.ts COPY_TOAST_DURATION_MS.
	terminalCopyToastDuration = 1500 * time.Millisecond
)

// copySelection puts a selection on the clipboard and says so, as the web
// window's copy-on-select does.
func (ts *terminals) copySelection(gtx layout.Context, text string) {
	gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(text))})
	n := utf8.RuneCountInString(text)
	title := fmt.Sprintf(pluralFormat(n, terminalCopiedOne, terminalCopiedOther), n)
	ts.toaster.Show(ui.Toast{ID: terminalCopyToastID, Title: title, Duration: terminalCopyToastDuration})
}

// readTransfer is a clipboard read's text.
func readTransfer(e transfer.DataEvent) (string, error) {
	r := e.Open()
	defer r.Close()
	b, err := io.ReadAll(r)
	return string(b), err
}

// track follows the pointer over the grid for the hovered link.
func (t *termView) track(e pointer.Event) {
	switch e.Kind {
	case pointer.Leave:
		t.pointerIn = false
	case pointer.Move, pointer.Drag, pointer.Press, pointer.Release:
		t.pointerAt, t.pointerIn = e.Position, true
	}
}

// hoveredLink is the link under the pointer, read again each frame so
// output that scrolls under a still pointer moves the underline with it.
func (t *termView) hoveredLink() (gioterm.Link, bool) {
	if !t.pointerIn {
		return gioterm.Link{}, false
	}
	return t.vt.LinkAt(t.pointerAt.X, t.pointerAt.Y)
}

// drawHoveredLink underlines the hovered link in fg and gives the grid a
// hand cursor, as xterm.js marks a link under the pointer in the web window.
func (t *termView) drawHoveredLink(gtx layout.Context, fg color.NRGBA) {
	l, ok := t.hoveredLink()
	if !ok {
		return
	}
	cellW, cellH := t.vt.CellSize()
	cols, _ := t.vt.Size()
	thick := max(gtx.Dp(linkUnderline), 1)
	for row := l.From.Y; row <= l.To.Y; row++ {
		from, to := 0, cols-1
		if row == l.From.Y {
			from = l.From.X
		}
		if row == l.To.Y {
			to = l.To.X
		}
		y := (row+1)*cellH - thick
		paint.FillShape(gtx.Ops, fg, clip.Rect(image.Rect(from*cellW, y, (to+1)*cellW, y+thick)).Op())
	}
	pointer.CursorPointer.Add(gtx.Ops)
}

// linkUnderline is the hovered link's underline.
const linkUnderline = unit.Dp(1)
