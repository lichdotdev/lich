package gioterm

import (
	"errors"
	"image/color"
	"testing"
)

func TestEncodeCtrlLetter(t *testing.T) {
	v, err := New(80, 24, testCellW, testCellH, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		setup string
		want  string
	}{
		{"legacy", "", "\x04"},
		// nvim turns on kitty "disambiguate" (CSI > 1 u) once the terminal answers its query.
		{"kitty disambiguate", "\x1b[>1u", "\x1b[100;5u"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.setup != "" {
				v.Write([]byte(c.setup))
			}
			got := string(v.encodeKey(keyA+3, modCtrl, 'd', "d"))
			if got != c.want {
				t.Fatalf("Ctrl+D encoded as %q, want %q", got, c.want)
			}
		})
	}
}

func TestEncodeSpecialKeysUnderKitty(t *testing.T) {
	v, err := New(80, 24, testCellW, testCellH, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	v.Write([]byte("\x1b[>1u"))
	cases := []struct {
		name string
		key  int
		want string
	}{
		{"escape", keyEscape, "\x1b[27u"},
		{"enter", keyEnter, "\r"},
		{"backspace", keyBackspace, "\x7f"},
		{"up", keyUp, "\x1b[A"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(v.encodeKey(c.key, 0, 0, "")); got != c.want {
				t.Fatalf("encoded as %q, want %q", got, c.want)
			}
		})
	}
}

// A window frees a terminal on one goroutine while another may still hold it:
// a closed session's terminal is freed as a theme switch paints it.
func TestClosedTerminalIgnoresCalls(t *testing.T) {
	v, err := New(80, 24, testCellW, testCellH, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	v.Close()
	if err := v.SetColors(Colors{Background: color.NRGBA{A: 0xff}}); !errors.Is(err, ErrClosed) {
		t.Fatalf("SetColors after Close: %v, want ErrClosed", err)
	}
	if err := v.Resize(100, 30, testCellW, testCellH); !errors.Is(err, ErrClosed) {
		t.Fatalf("Resize after Close: %v, want ErrClosed", err)
	}
	if _, err := v.Update(); !errors.Is(err, ErrClosed) {
		t.Fatalf("Update after Close: %v, want ErrClosed", err)
	}
	v.Write([]byte("text"))
	v.Scroll(-1)
	v.ClearSelection()
	v.SelectPress(1, 1, 0)
	if v.Paste("x") != nil || v.EncodeMouse(0, 0, 0, 1, 1, false) != nil || v.encodeKey(keyEnter, 0, 0, "") != nil {
		t.Fatal("a closed terminal encoded input")
	}
	if v.MouseTracking() || v.AlternateScroll() || v.SelectionText() != "" {
		t.Fatal("a closed terminal reported a mode")
	}
	if _, ok := v.LinkAt(1, 1); ok {
		t.Fatal("a closed terminal found a link")
	}
	v.Close()
}
