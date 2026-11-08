package gioterm

import (
	"testing"
	"time"
)

var graphemeCases = []struct {
	name, text string
	// next is the column the character after the grapheme lands in.
	next int
}{
	{"ZWJ family", "👨‍👩‍👧", 2},
	{"flag", "🇧🇷", 2},
	{"combining accent", "é", 1},
}

func cellText(c cell) string { return string(rune(c.cp)) + c.rest }

// A grapheme is one cell, as wide as the host terminal draws it, and the
// cell holds every codepoint of it.
func TestAGraphemeClusterIsOneCell(t *testing.T) {
	for _, tc := range graphemeCases {
		t.Run(tc.name, func(t *testing.T) {
			v := newSizedTerminal(t)
			v.Write([]byte(tc.text + "X"))
			s, err := v.Update()
			if err != nil {
				t.Fatal(err)
			}
			if got := cellText(s.cells[0]); got != tc.text {
				t.Errorf("cell 0 holds %+q, want %+q", got, tc.text)
			}
			if got := cellText(s.cells[tc.next]); got != "X" {
				t.Errorf("cell %d holds %+q, want X", tc.next, got)
			}
			if s.s.cursorX != tc.next+1 {
				t.Errorf("cursor at column %d, want %d", s.s.cursorX, tc.next+1)
			}
		})
	}
}

// reset (RIS) puts every mode back to its default; clustering stays on.
func TestGraphemeClusteringSurvivesAFullReset(t *testing.T) {
	v := newSizedTerminal(t)
	v.Write([]byte("\x1bc🇧🇷X"))
	s, err := v.Update()
	if err != nil {
		t.Fatal(err)
	}
	if s.s.cursorX != 3 {
		t.Fatalf("after RIS the cursor is at column %d, want 3", s.s.cursorX)
	}
}

func TestSelectionCopiesTheWholeGrapheme(t *testing.T) {
	for _, tc := range graphemeCases {
		t.Run(tc.name, func(t *testing.T) {
			v := newSizedTerminal(t)
			v.Write([]byte(tc.text + "X"))
			if _, err := v.Update(); err != nil {
				t.Fatal(err)
			}
			y := float32(testCellH / 2)
			v.SelectPress(1, y, time.Second)
			end := float32(tc.next*testCellW + testCellW - 1)
			v.SelectDrag(end, y)
			v.SelectRelease(end, y)
			if got := v.SelectionText(); got != tc.text+"X" {
				t.Fatalf("selected %+q, want %+q", got, tc.text+"X")
			}
		})
	}
}

func TestLinkAtKeepsACombiningMark(t *testing.T) {
	v := newSizedTerminal(t)
	url := "https://x.dev/café/menu"
	v.Write([]byte("see " + url + " now"))
	if _, err := v.Update(); err != nil {
		t.Fatal(err)
	}
	x, y := cellCenter(6, 0)
	l, ok := v.LinkAt(x, y)
	if !ok || l.URL != url {
		t.Fatalf("LinkAt = %+q, %v; want %+q", l.URL, ok, url)
	}
	// The accent shares the e's cell: the link ends 23 cells after it starts.
	if l.From.X != 4 || l.To.X != 4+22 {
		t.Fatalf("link covers columns %d..%d, want 4..26", l.From.X, l.To.X)
	}
}
