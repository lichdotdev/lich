package main

import (
	"testing"

	"gioui.org/io/key"

	"github.com/lichdotdev/lich/native/gioterm"
)

func TestTerminalKeyBytesRewritesThePassedThroughChords(t *testing.T) {
	cases := []struct {
		name string
		goos string
		ev   key.Event
		want string
	}{
		{"image attach", "linux", key.Event{Name: "V", Modifiers: key.ModCtrl}, "\x16"},
		{"image attach on Windows", "windows", key.Event{Name: "V", Modifiers: key.ModCtrl}, "\x1bv"},
		{"newline", "linux", key.Event{Name: key.NameReturn, Modifiers: key.ModShift}, "\x1b\r"},
		{"erase word", "linux", key.Event{Name: key.NameDeleteBackward, Modifiers: key.ModCtrl}, "\x17"},
		{"enter still sends", "linux", key.Event{Name: key.NameReturn}, "\r"},
	}
	// The agent may turn on the kitty keyboard protocol; the chords are the
	// same bytes either way.
	for _, mode := range []struct{ name, setup string }{{"legacy", ""}, {"kitty", "\x1b[>1u"}} {
		for _, c := range cases {
			t.Run(mode.name+"/"+c.name, func(t *testing.T) {
				vt, err := gioterm.New(80, 24, 10, 20, func([]byte) {})
				if err != nil {
					t.Fatal(err)
				}
				vt.Write([]byte(mode.setup))
				if got := string(terminalKeyBytes(vt, c.ev, c.goos)); got != c.want {
					t.Fatalf("sent %q, want %q", got, c.want)
				}
			})
		}
	}
}

func TestPasteChords(t *testing.T) {
	cases := []struct {
		name string
		goos string
		ev   key.Event
		want bool
	}{
		{"ctrl+shift+v", "linux", key.Event{Name: "V", Modifiers: key.ModCtrl | key.ModShift}, true},
		{"ctrl+shift+v on Windows", "windows", key.Event{Name: "V", Modifiers: key.ModCtrl | key.ModShift}, true},
		{"cmd+v on macOS", "darwin", key.Event{Name: "V", Modifiers: key.ModCommand}, true},
		{"ctrl+v attaches instead", "linux", key.Event{Name: "V", Modifiers: key.ModCtrl}, false},
		{"ctrl+shift+c", "linux", key.Event{Name: "C", Modifiers: key.ModCtrl | key.ModShift}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isPasteChord(c.ev, c.goos); got != c.want {
				t.Fatalf("paste chord %v, want %v", got, c.want)
			}
		})
	}
}

func TestPastedClipboard(t *testing.T) {
	vt, err := gioterm.New(80, 24, 10, 20, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vt.Close)
	vt.Write([]byte("\x1b[?2004h"))
	cases := []struct {
		name, goos, text, want string
	}{
		{"text goes in bracketed", "linux", "ls\n", "\x1b[200~ls\n\x1b[201~"},
		// No text on the clipboard is an image (or a file) for the agent to
		// read itself.
		{"no text attaches the image", "darwin", "", "\x16"},
		{"no text on Windows", "windows", "", "\x1bv"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(pastedClipboard(vt, c.text, c.goos)); got != c.want {
				t.Fatalf("sent %q, want %q", got, c.want)
			}
		})
	}
}
