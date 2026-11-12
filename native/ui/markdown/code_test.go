package markdown

import (
	"image"
	"image/color"
	"strings"
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
	(Cache{}).Layout(gtx, th, "test", "`one two three four five six seven eight nine`", nil)
	if err := win.Frame(ops); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 180, 150))
	if err := win.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	background := ui.MixOKLab(th.Background, th.Foreground, markdownCodeBackgroundMix)
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
	if first < 0 || last-first < gtx.Sp(BodySize)*2 {
		t.Errorf("background extends from %d to %d, want backgrounds on multiple lines", first, last)
	}
}

func TestAQuoteBarAndACodePanelAreDrawn(t *testing.T) {
	size := image.Pt(240, 200)
	win, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		t.Skipf("no headless GPU: %v", err)
	}
	defer win.Release()
	th := markdownTheme()
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: size}}
	paint.Fill(ops, th.Background)
	(Cache{}).Layout(gtx, th, "test", "> quoted\n\n```\ncode\n```", nil)
	if err := win.Frame(ops); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rectangle{Max: size})
	if err := win.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	ground := img.RGBAAt(size.X-1, size.Y-1)
	if bar := img.RGBAAt(1, 6); bar == ground {
		t.Error("no bar at the left of the blockquote")
	}
	if beside := img.RGBAAt(size.X-2, 6); beside != ground {
		t.Errorf("the blockquote is filled at its right edge (%v), want only a bar", beside)
	}
	quote := gtx.Sp(BodySize)*3/2 + gtx.Dp(ui.Space(BlockGap))
	if panel := img.RGBAAt(size.X-gtx.Dp(codePanelPad), quote+gtx.Dp(codePanelPad)); panel == ground {
		t.Error("no panel behind the fenced code block")
	}
}

func TestATabInCodeExpandsToTheCodeViewsTabStops(t *testing.T) {
	blocks := renderBlocks(markdownTheme(), "```go\n\tindented\nab\tc\n\t\tdeep\n```\n\nInline `a\tb` here.")
	if got, want := proseText(blocks[0]), "    indented\nab  c\n        deep"; got != want {
		t.Errorf("code block reads %q, want its tabs expanded to stops of 4: %q", got, want)
	}
	if got := proseText(blocks[1]); strings.Contains(got, "\t") {
		t.Errorf("inline code keeps a tab, which the mono font has no glyph for: %q", got)
	}
}

func TestACodeBlockUsesTheCodeLineHeight(t *testing.T) {
	th := markdownTheme()
	blocks := renderBlocks(th, "one\\\ntwo\\\nthree\n\n```\none\ntwo\nthree\n```")
	gtx, _ := markdownFrame(400)
	prose := blocks[0].layout(gtx, th, nil).Size.Y
	code := blocks[1].layout(gtx, th, nil).Size.Y - 2*gtx.Dp(codePanelPad)
	if code >= prose {
		t.Errorf("three lines of code are %dpx tall and three of prose %dpx, want code set tighter than prose", code, prose)
	}
}

// nestedPixels draws src and returns the frame with the ground's color.
func nestedPixels(t *testing.T, src string) (*image.RGBA, layout.Context) {
	t.Helper()
	size := image.Pt(320, 320)
	win, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		t.Skipf("no headless GPU: %v", err)
	}
	defer win.Release()
	th := markdownTheme()
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: size}}
	paint.Fill(ops, th.Background)
	(Cache{}).Layout(gtx, th, "test", src, nil)
	if err := win.Frame(ops); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rectangle{Max: size})
	if err := win.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	return img, gtx
}

// hasColor reports whether any pixel of img is c.
func hasColor(img *image.RGBA, c color.NRGBA) bool {
	for y := range img.Bounds().Dy() {
		for x := range img.Bounds().Dx() {
			if p := img.RGBAAt(x, y); p.R == c.R && p.G == c.G && p.B == c.B {
				return true
			}
		}
	}
	return false
}

func TestACodeBlockAndAQuoteInsideAListItemAreDrawn(t *testing.T) {
	th := markdownTheme()
	img, _ := nestedPixels(t, "- item\n\n  ```\n  code\n  ```\n")
	if !hasColor(img, codeFill(th)) {
		t.Error("no panel behind a code block inside a list item")
	}
	top, _ := nestedPixels(t, "> quoted")
	bar := top.RGBAAt(1, 6)
	img, _ = nestedPixels(t, "- item\n\n  > quoted\n")
	if !hasColor(img, color.NRGBA(bar)) {
		t.Error("no bar beside a quote inside a list item")
	}
}

func TestBlocksInsideAQuoteKeepTheirOwnDrawing(t *testing.T) {
	th := markdownTheme()
	blocks := renderBlocks(th, "> said\n>\n> ```\n> code\n> ```\n>\n> - item\n>\n> > deeper")
	if len(blocks) != 1 || !blocks[0].quote {
		t.Fatalf("blocks = %+v, want one quote", blocks)
	}
	inner := blocks[0].blocks
	if len(inner) != 4 || !inner[1].panel || inner[2].list == nil || !inner[3].quote {
		t.Fatalf("a quote holds %+v, want a paragraph, a code panel, a list and a quote", inner)
	}
	for _, span := range inner[0].prose.spans {
		if span.Color != th.MutedForeground {
			t.Errorf("quoted span %q color = %v, want muted %v", span.Content, span.Color, th.MutedForeground)
		}
	}
	img, gtx := nestedPixels(t, "> said\n>\n> ```\n> code\n> ```")
	if !hasColor(img, codeFill(th)) {
		t.Error("no panel behind a code block inside a quote")
	}
	ground := img.RGBAAt(img.Bounds().Dx()-1, img.Bounds().Dy()-1)
	if bar := img.RGBAAt(1, gtx.Sp(BodySize)*3); bar == ground {
		t.Error("the quote's bar stops above the code block it holds")
	}
}
