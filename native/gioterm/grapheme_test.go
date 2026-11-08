package gioterm

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/lichdotdev/lich/native/glyphs"
)

var graphemeCases = []struct {
	name, text string
	// next is the column the character after the grapheme lands in.
	next int
}{
	{"ZWJ family", "👨\u200d👩\u200d👧", 2},
	{"flag", "🇧🇷", 2},
	{"combining accent", "e\u0301", 1},
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
	url := "https://x.dev/cafe\u0301/menu"
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

const familyOf4 = "👩\u200d👩\u200d👧\u200d👦"

// A screen of long sequences must not use up what the graphemes of a frame
// can hold: the last cell keeps all of its codepoints.
func TestAScreenOfLongSequencesKeepsEveryGrapheme(t *testing.T) {
	v := newSizedTerminal(t)
	cols, rows := v.Size()
	for range (cols / 2) * rows {
		v.Write([]byte(familyOf4))
	}
	s, err := v.Update()
	if err != nil {
		t.Fatal(err)
	}
	if got := cellText(s.cells[cols*rows-2]); got != familyOf4 {
		t.Fatalf("the last cell holds %+q, want %+q", got, familyOf4)
	}
}

// One cell of thousands of combining marks takes a bounded share and leaves
// the cells after it whole.
func TestAHeavyCellIsCappedAndLeavesTheOthersWhole(t *testing.T) {
	v := newSizedTerminal(t)
	v.Write([]byte("e" + strings.Repeat("\u0301", 5000) + " " + familyOf4))
	s, err := v.Update()
	if err != nil {
		t.Fatal(err)
	}
	if n := utf8.RuneCountInString(cellText(s.cells[0])); n != 32 {
		t.Errorf("the heavy cell holds %d codepoints, want 32", n)
	}
	if got := cellText(s.cells[2]); got != familyOf4 {
		t.Errorf("the cell after it holds %+q, want %+q", got, familyOf4)
	}
}

// A combining mark on a blank or a block element is ink of its own, which a
// renderer that looks only at the first codepoint draws as nothing or as the
// block.
func TestAMarkOnABlankOrBlockIsDrawn(t *testing.T) {
	for _, kind := range []string{RenderRows, RenderAtlas, RenderPaths} {
		for _, base := range []string{" ", "█"} {
			t.Run(kind+" "+base, func(t *testing.T) {
				frame := func(text string) ([32]byte, [][]byte) {
					r, err := NewRenderer(kind, "monospace", 14)
					if err != nil {
						t.Fatal(err)
					}
					vt, err := New(10, 2, 8, 16, func([]byte) {})
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(vt.Close)
					vt.Write([]byte(text))
					digest, _ := drawFrame(t, r, vt)
					return digest, rowPixels(r)
				}
				plain, plainRows := frame(base)
				marked, markedRows := frame(base + "\u0301")
				if plain == marked && reflect.DeepEqual(plainRows, markedRows) {
					t.Fatal("the mark changed nothing on screen")
				}
			})
		}
	}
}

// With more glyph pages than the bound, a frame starts over from an empty
// atlas: colors and graphemes are program-chosen, and pages are never freed
// one at a time. Every row is redrawn, as the recorded ones hold old pages.
func TestAtlasStartsOverPastItsPageBound(t *testing.T) {
	r, err := NewRenderer(RenderAtlas, "monospace", 14)
	if err != nil {
		t.Fatal(err)
	}
	vt, err := New(10, 4, 8, 16, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vt.Close)
	vt.Write([]byte("hello"))
	drawFrame(t, r, vt)
	a := r.(*atlasRenderer)
	for len(a.pages) < 8 {
		a.pages = append(a.pages, a.pages[0])
	}
	if _, redrawn := drawFrame(t, r, vt); redrawn != 4 {
		t.Errorf("past the bound redrew %d rows, want 4", redrawn)
	}
	if len(a.pages) != 1 {
		t.Errorf("%d pages after starting over, want 1", len(a.pages))
	}
}

// A screen that needs more pages than the bound keeps them: starting over
// every frame would re-tint and re-upload the screen for a cursor blink.
func TestAtlasSettlesOnAScreenPastItsPageBound(t *testing.T) {
	r, err := NewRenderer(RenderAtlas, "monospace", 80)
	if err != nil {
		t.Fatal(err)
	}
	vt, err := New(200, 20, 8, 16, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vt.Close)
	var b strings.Builder
	for i := range 200*20 - 1 {
		fmt.Fprintf(&b, "\x1b[38;2;%d;%d;0mM", i%256, i/256)
	}
	vt.Write([]byte(b.String()))
	drawFrame(t, r, vt)
	a := r.(*atlasRenderer)
	pages := len(a.pages)
	if pages < 8 {
		t.Fatalf("the screen took %d pages, the case needs 8 or more", pages)
	}
	first := a.pages[0]
	for frame := range 3 {
		if _, redrawn := drawFrame(t, r, vt); redrawn != 0 {
			t.Fatalf("frame %d redrew %d rows with nothing changed, want none", frame, redrawn)
		}
	}
	if len(a.pages) != pages || a.pages[0] != first {
		t.Fatalf("the pages did not settle: %d now, %d before", len(a.pages), pages)
	}
}

func TestPathsCacheIsBounded(t *testing.T) {
	r, err := NewRenderer(RenderPaths, "monospace", 14)
	if err != nil {
		t.Fatal(err)
	}
	vt, err := New(10, 2, 8, 16, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vt.Close)
	drawFrame(t, r, vt)
	g := r.(*grid)
	for i := range 1 << 14 {
		g.cache[glyphs.Key{R: rune(0x10000 + i)}] = cachedGlyph{}
	}
	g.glyph(glyphs.Key{R: 'x'})
	if len(g.cache) != 1 {
		t.Fatalf("%d glyphs cached past the bound, want the new one alone", len(g.cache))
	}
}

// A sequence with no ligature draws its first glyph, as the two other
// renderers do, not every glyph across the columns after the cell.
func TestPathsDrawTheFirstGlyphOfASequenceWithoutALigature(t *testing.T) {
	r, err := NewRenderer(RenderPaths, "monospace", 14)
	if err != nil {
		t.Fatal(err)
	}
	vt, err := New(10, 2, 8, 16, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vt.Close)
	drawFrame(t, r, vt)
	if n := len(r.(*grid).shape(glyphs.Key{R: '🚀', Rest: "\u200d🚀"})); n != 1 {
		t.Fatalf("shaped to %d glyphs, want 1", n)
	}
}
