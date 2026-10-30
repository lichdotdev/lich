package main

import (
	"image"
	"testing"

	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

func TestMarkdownInlineCodeAndFencedCodeAreDistinct(t *testing.T) {
	blocks := renderBlocks(markdownTheme(), "Before `go test ./...` after.\n\n```go\ngo test ./...\n```")
	inline := blocks[0].prose
	marked := 0
	for _, code := range inline.code {
		if code {
			marked++
		}
	}
	if marked == 0 {
		t.Fatal("inline code has no background spans")
	}
	for _, code := range blocks[1].prose.code {
		if code {
			t.Fatal("a fenced code block was treated as inline code")
		}
	}
}

func TestMarkdownInlineCodeBackgroundWrapsWithText(t *testing.T) {
	win, err := headless.NewWindow(180, 150)
	if err != nil {
		t.Skipf("no headless GPU: %v", err)
	}
	defer win.Release()
	th := markdownTheme()
	th.Surface = th.Background
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: image.Pt(180, 150)}}
	paint.Fill(ops, th.Background)
	(markdownCache{}).layout(gtx, th, "test", "`one two three four five six seven eight nine`", nil)
	if err := win.Frame(ops); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 180, 150))
	if err := win.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	background := ui.MixOKLab(th.Background, th.Foreground, 0.1)
	first, last := -1, -1
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if img.RGBAAt(x, y).R == background.R && img.RGBAAt(x, y).G == background.G && img.RGBAAt(x, y).B == background.B {
				if first < 0 {
					first = y
				}
				last = y
			}
		}
	}
	if first < 0 || last-first < gtx.Sp(markdownBodySize)*2 {
		t.Errorf("background extends from %d to %d, want backgrounds on multiple lines", first, last)
	}
}
