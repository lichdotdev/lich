package gioterm

import (
	"strings"
	"testing"
)

// screenText is the viewport as text, a line per row, trailing blanks and
// blank rows at the bottom dropped.
func screenText(t *testing.T, v *Terminal) string {
	t.Helper()
	s, err := v.Update()
	if err != nil {
		t.Fatal(err)
	}
	cols, rows := v.Size()
	lines := make([]string, rows)
	for y := range rows {
		var b strings.Builder
		for x := range cols {
			cp := s.cells[y*cols+x].cp
			if cp == 0 {
				cp = ' '
			}
			b.WriteRune(rune(cp))
		}
		lines[y] = strings.TrimRight(b.String(), " ")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

// A prompt line as wide as the terminal wraps when the terminal narrows; the
// shell that asked for its prompt to be redrawn (OSC 133;A;redraw=1) gets all
// of it cleared, or the head of the wrapped line stays on screen above the
// prompt the shell draws again.
func TestNarrowingClearsAWrappedRedrawablePrompt(t *testing.T) {
	cases := []struct {
		name, mark, want string
	}{
		{"redrawable", "\x1b]133;A;redraw=1\x07", "out"},
		{"not redrawable", "\x1b]133;A\x07", "out\n" + strings.Repeat("x", 15) + "\nxxxxx\n>"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v, err := New(20, 6, func([]byte) {})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(v.Close)
			v.Write([]byte("out\r\n" + c.mark + strings.Repeat("x", 20) + "\r\n> \x1b]133;B\x07"))
			if err := v.Resize(15, 6, testCellW, testCellH); err != nil {
				t.Fatal(err)
			}
			if got := screenText(t, v); got != c.want {
				t.Fatalf("screen after narrowing:\n%s\nwant:\n%s", got, c.want)
			}
		})
	}
}
