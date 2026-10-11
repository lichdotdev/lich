package main

import (
	"fmt"
	"image"
	"strings"
	"testing"
	"time"

	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/omartelo/lich/native/ui"
)

// benchCardLines is the measured case: one open 60-line diff card.
const benchCardLines = 60

func benchCard(b *testing.B) (*reviewPanel, *fileCard) {
	b.Helper()
	var patch strings.Builder
	patch.WriteString("diff --git a/main.go b/main.go\nindex 1111111..2222222 100644\n--- a/main.go\n+++ b/main.go\n")
	fmt.Fprintf(&patch, "@@ -1,%d +1,%d @@\n", benchCardLines, benchCardLines)
	for i := range benchCardLines {
		line := fmt.Sprintf("\tresult = append(result, compute(input[%d], options.Threshold*%d))\n", i, i)
		if i%7 == 0 {
			patch.WriteString("-" + line + "+" + strings.Replace(line, "compute", "computeFaster", 1))
			continue
		}
		patch.WriteString(" " + line)
	}
	files, err := parsePatchFiles(patch.String())
	if err != nil {
		b.Fatal(err)
	}
	p, err := newReviewPanel(nil, &ui.Root{}, nil, "monospace", 12, func() {}, func(ui.Toast) {})
	if err != nil {
		b.Fatal(err)
	}
	p.th = &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection())), Surface: ui.Dark.Sidebar}
	c := newFileCard(p, files[0], false, 0)
	return p, c
}

// benchmarkCardAt lays the card out once per iteration, at the width width
// returns for that frame: a drag of the dock's edge changes it every frame.
func benchmarkCardAt(b *testing.B, width func(i int) int) {
	p, c := benchCard(b)
	var ops op.Ops
	frame := func(i int) {
		ops.Reset()
		gtx := layout.Context{Ops: &ops, Now: time.Unix(1, 0), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Constraints{Max: image.Pt(width(i), 4000)}}
		c.layout(gtx, p)
	}
	frame(0)
	frame(1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		frame(i)
	}
}

func BenchmarkReviewCardFixedWidth(b *testing.B) {
	benchmarkCardAt(b, func(int) int { return 500 })
}

func BenchmarkReviewCardDraggedWidth(b *testing.B) {
	benchmarkCardAt(b, func(i int) int { return 500 + (i%200)*3 })
}
