package main

import (
	"image"
	"slices"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"

	"github.com/lichdotdev/lich/native/fonts"
	"github.com/lichdotdev/lich/native/ui"
)

func TestAMarkdownFileOpensRendered(t *testing.T) {
	f := newFileEditor(nil, func() {}, fonts.SystemMono, 13, "/c")
	f.open("README.md", 0, 0, agentNote{})
	f.md.mode = mdSource
	f.open("docs/guide.md", 0, 0, agentNote{})
	if f.md.mode != mdRendered {
		t.Fatalf("a newly opened Markdown file came up in mode %d, want rendered", f.md.mode)
	}
}

func TestAMarkdownFileOpenedOnLinesOrWithANoteShowsItsSource(t *testing.T) {
	f := newFileEditor(nil, func() {}, fonts.SystemMono, 13, "/c")
	f.open("README.md", 3, 5, agentNote{})
	if f.md.mode != mdSource {
		t.Fatalf("opened on lines in mode %d, want the source", f.md.mode)
	}
	f.open("docs/guide.md", 0, 0, agentNote{text: "why"})
	if f.md.mode != mdSource {
		t.Fatalf("opened with a note in mode %d, want the source", f.md.mode)
	}
}

func TestTheMarkdownTogglesSwitchBetweenSourceRenderedAndSplit(t *testing.T) {
	f := newFileEditor(nil, func() {}, fonts.SystemMono, 13, "/c")
	f.open("README.md", 0, 0, agentNote{})
	r := newFrameRig(image.Pt(600, 60), func(gtx C, th *ui.Theme) { f.header(gtx, th) })
	r.settle(t)
	split, preview := markdownActions(t, r)
	steps := []struct {
		at   f32.Point
		want markdownMode
	}{{preview, mdSource}, {preview, mdRendered}, {split, mdSplit}, {preview, mdRendered}, {split, mdSplit}, {split, mdSource}}
	for i, s := range steps {
		r.click(t, s.at)
		if f.md.mode != s.want {
			t.Fatalf("click %d left mode %d, want %d", i, f.md.mode, s.want)
		}
	}
}

// markdownActions are the centres of the header's two icon actions, left to
// right.
func markdownActions(t *testing.T, r *frameRig) (left, right f32.Point) {
	t.Helper()
	var at []image.Rectangle
	for _, n := range r.nodes() {
		if b := n.Desc.Bounds; n.Desc.Gestures&input.ClickGesture != 0 && b.Dx() < 100 && !slices.Contains(at, b) {
			at = append(at, b)
		}
	}
	if len(at) != 2 {
		t.Fatalf("header drew %d icon actions, want 2:\n%s", len(at), r.drawn())
	}
	slices.SortFunc(at, func(a, b image.Rectangle) int { return a.Min.X - b.Min.X })
	return centre(at[0]), centre(at[1])
}
