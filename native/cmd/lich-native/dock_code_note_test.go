package main

import (
	"image"
	"strings"
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

func TestALongAgentNoteWrapsInsteadOfBeingCut(t *testing.T) {
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection()))}
	p := &preview{blockTh: th}
	height := func(comment string) int {
		p.note = agentNote{text: comment, author: "Claude Code", kind: "claude"}
		gtx := layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: image.Pt(300, 2000)}}
		return p.noteBlock(gtx).Size.Y
	}
	if short, long := height("why"), height(strings.Repeat("the importer skips billing ", 12)); long <= short {
		t.Fatalf("short note %dpx, long note %dpx: the long one was cut to a line", short, long)
	}
}
