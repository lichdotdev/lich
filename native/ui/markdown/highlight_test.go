package markdown

import (
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/godemirror"

	"github.com/lichdotdev/lich/native/ui"
)

func highlightTheme() *ui.Theme {
	th := markdownTheme()
	th.Code = godemirror.DarkTheme
	return th
}

func spanColors(b markdownBlock) map[color.NRGBA]bool {
	colors := map[color.NRGBA]bool{}
	for _, s := range b.prose.spans {
		if strings.TrimSpace(s.Content) != "" {
			colors[s.Color] = true
		}
	}
	return colors
}

func TestAGoBlockIsColoredAsTheCodeViewsColorIt(t *testing.T) {
	th := highlightTheme()
	block := renderBlocks(th, "```go\nfunc main() {\n\treturn \"hi\"\n}\n```")[0]
	if n := len(spanColors(block)); n < 3 {
		t.Errorf("a Go block is drawn in %d colors, want its keywords, strings and the rest apart", n)
	}
	if got, want := proseText(block), "func main() {\n    return \"hi\"\n}"; got != want {
		t.Errorf("the block reads %q, want %q", got, want)
	}
	lines, err := godemirror.Highlight("code.go", "func main() {", nil, th.Code)
	if err != nil {
		t.Fatal(err)
	}
	if first := block.prose.spans[0]; first.Color != lines[0][0].Style.Color {
		t.Errorf("func is %v in the block and %v in the code view", first.Color, lines[0][0].Style.Color)
	}
}

func TestABlockInALanguageNobodyKnowsOrNoneStaysPlain(t *testing.T) {
	th := highlightTheme()
	for _, src := range []string{"```nolanguage\na b\nc\n```", "```\nfunc main() {}\n```"} {
		block := renderBlocks(th, src)[0]
		if colors := spanColors(block); len(colors) != 1 || !colors[markdownProseColor(th)] {
			t.Errorf("%q is drawn in %v, want the one color of prose's code", src, colors)
		}
	}
}

func TestAThemeWithNoHighlightLeavesCodePlain(t *testing.T) {
	th := markdownTheme()
	block := renderBlocks(th, "```go\nfunc main() {}\n```")[0]
	if colors := spanColors(block); len(colors) != 1 {
		t.Errorf("with no highlight style a Go block is drawn in %d colors, want 1", len(colors))
	}
}

func TestAThemeChangeHighlightsTheBodyAgain(t *testing.T) {
	m, th := Cache{}, markdownTheme()
	gtx, _ := markdownFrame(400)
	m.Layout(gtx, th, "k", "```go\nfunc main() {}\n```", nil)
	th.Code = godemirror.DarkTheme
	m.Layout(gtx, th, "k", "```go\nfunc main() {}\n```", nil)
	if n := len(spanColors(m["k"].blocks[0])); n < 2 {
		t.Error("the body kept its plain code after the theme gained a highlight style")
	}
}

// fiveHundredLines is a streaming answer's long Go block.
var fiveHundredLines = "```go\n" + strings.Repeat("func f(x int) string {\n\treturn fmt.Sprintf(\"%d\", x) // a comment\n}\n", 167) + "```\n"

func TestAFiveHundredLineBlockRendersWithinAFrame(t *testing.T) {
	th := highlightTheme()
	const renders = 10
	start := time.Now()
	for range renders {
		renderBlocks(th, fiveHundredLines)
	}
	// Average out scheduler noise while preserving the 60 Hz frame budget.
	if took := time.Since(start) / renders; took > time.Second/60 {
		t.Errorf("rendering 500 lines of Go took %v per render, more than a frame", took)
	}
}

func BenchmarkRenderAFiveHundredLineGoBlock(b *testing.B) {
	th := highlightTheme()
	for b.Loop() {
		renderBlocks(th, fiveHundredLines)
	}
}
