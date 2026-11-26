package markdown

import (
	"image"
	"image/color"
	"slices"
	"strings"
	"testing"

	"gioui.org/gpu/headless"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

const diffSrc = "```diff\n--- a/x.go\n+++ b/x.go\n@@ -1,3 +1,3 @@\n keep\n-old\n+new\n```"

func TestADiffBlockTonesEachLine(t *testing.T) {
	th := markdownTheme()
	blocks := renderBlocks(th, diffSrc)
	want := []diffTone{diffHeader, diffHeader, diffHeader, diffContext, diffDeleted, diffAdded}
	if len(blocks) != 1 || !slices.Equal(blocks[0].diff, want) {
		t.Fatalf("tones = %v, want %v", blocks[0].diff, want)
	}
	if got := proseText(blocks[0]); got != strings.TrimSuffix(strings.TrimPrefix(diffSrc, "```diff\n"), "\n```") {
		t.Errorf("the diff reads %q, want it as written", got)
	}
	colors := []any{th.MutedForeground, th.MutedForeground, th.MutedForeground, markdownProseColor(th), th.Destructive, th.TonePass}
	for i, span := range blocks[0].prose.spans {
		if span.Color != colors[i] {
			t.Errorf("line %d %q color = %v, want %v", i, span.Content, span.Color, colors[i])
		}
	}
}

func TestOnlyADiffOrPatchFenceIsADiff(t *testing.T) {
	for src, diff := range map[string]bool{
		"```patch\n+a\n```": true,
		"```DIFF\n+a\n```":  true,
		"```go\n+a\n```":    false,
		"```\n+a\n```":      false,
		"    +a":            false,
	} {
		if got := renderBlocks(markdownTheme(), src)[0].diff != nil; got != diff {
			t.Errorf("%q is a diff: %v, want %v", src, got, diff)
		}
	}
}

func TestADiffTintsItsChangedRowsAcrossThePanel(t *testing.T) {
	size := image.Pt(300, 200)
	win, err := headless.NewWindow(size.X, size.Y)
	if err != nil {
		t.Skipf("no headless GPU: %v", err)
	}
	defer win.Release()
	th := markdownTheme()
	ops := new(op.Ops)
	gtx := layout.Context{Ops: ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Constraints{Max: size}}
	paint.Fill(ops, th.Background)
	(Cache{}).Layout(gtx, th, "test", "```diff\n keep\n-old\n+new\n```", nil)
	if err := win.Frame(ops); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rectangle{Max: size})
	if err := win.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	fill := codeFill(th)
	line := int(float32(gtx.Sp(BodySize)) * markdownCodeLineHeightScale)
	pad := gtx.Dp(codePanelPad)
	edge := size.X - 2
	for i, want := range []color.NRGBA{fill, ui.MixOKLab(fill, th.Destructive, diffRowMix), ui.MixOKLab(fill, th.TonePass, diffRowMix)} {
		got := img.RGBAAt(edge, pad+i*line+line/2)
		if got.R != want.R || got.G != want.G || got.B != want.B {
			t.Errorf("row %d at the panel's right edge is %v, want %v", i, got, want)
		}
	}
}
