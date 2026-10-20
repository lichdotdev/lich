package main

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// The provider's title keeps the documentation link beside the name while the
// line holds the name, the link and the switch, and moves it under the name
// when it does not.
func TestProviderTitleRowWrapsDocsLinkWhenNarrow(t *testing.T) {
	const nameW, docsW, toggleW, lineH, docsH = 200, 100, 80, 32, 16
	block := func(w, h int) recorded { return recorded{size: image.Pt(w, h)} }
	height := func(width int) int {
		gtx := layout.Context{
			Ops:         new(op.Ops),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Constraints{Min: image.Pt(width, 0), Max: image.Pt(width, 1000)},
		}
		docs := block(docsW, docsH)
		return providerTitleRow(gtx, block(nameW, lineH), &docs, block(toggleW, lineH)).Size.Y
	}
	if got := height(1000); got != lineH {
		t.Errorf("wide title height %d, want %d: the link beside the name", got, lineH)
	}
	if got := height(320); got != lineH+docsH {
		t.Errorf("narrow title height %d, want %d: the link under the name", got, lineH+docsH)
	}
}
