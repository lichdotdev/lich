package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/ui"
)

const blockTestPatch = "diff --git a/a.go b/a.go\nindex 1..2 100644\n--- a/a.go\n+++ b/a.go\n@@ -1,3 +1,3 @@\n one\n-two\n+TWO\n three\n"

// cardHeight lays the card out until its height holds still: blocks reach the
// view once highlighting allows, and their room the frame after.
func cardHeight(t *testing.T, p *reviewPanel, c *fileCard) int {
	t.Helper()
	var ops op.Ops
	last, steady := -1, 0
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ops.Reset()
		gtx := layout.Context{Ops: &ops, Now: time.Now(), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Constraints{Max: image.Pt(600, 4000)}}
		h := c.layout(gtx, p).Size.Y
		if h == last {
			if steady++; steady > 5 {
				return h
			}
		} else {
			last, steady = h, 0
		}
		time.Sleep(10 * time.Millisecond)
	}
	return last
}

func TestFileCardMakesRoomForTheBlocksUnderItsLines(t *testing.T) {
	files, err := parsePatchFiles(blockTestPatch)
	if err != nil {
		t.Fatal(err)
	}
	p, err := newReviewPanel(nil, &ui.Root{}, nil, "monospace", 12, func() {}, func(ui.Toast) {})
	if err != nil {
		t.Fatal(err)
	}
	p.th = &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Sidebar}
	const blockSize = 77
	var shown bool
	p.pr = &prFiles{ticks: map[string]*widget.Bool{}, blocks: func(string) []cardBlock {
		if !shown {
			return nil
		}
		return []cardBlock{{key: "thread:T", at: godemirror.LineRef{Side: godemirror.NewSide, Line: 2},
			widget: func(gtx C) D { return D{Size: image.Pt(gtx.Constraints.Max.X, blockSize)} }}}
	}}
	c := newFileCard(p, files[0], false, 0)
	bare := cardHeight(t, p, c)
	shown = true
	if got := cardHeight(t, p, c); got != bare+blockSize {
		t.Fatalf("card with a thread = %dpx, want %d+%d", got, bare, blockSize)
	}
	shown = false
	if got := cardHeight(t, p, c); got != bare {
		t.Errorf("card once the thread left = %dpx, want %d", got, bare)
	}
}
