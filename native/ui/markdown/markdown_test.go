package markdown

import (
	"image"
	"slices"
	"strings"
	"testing"
	"time"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

func markdownTheme() *ui.Theme {
	th := &ui.Theme{Palette: ui.Dark, Shaper: text.NewShaper(text.WithCollection(gofont.Collection()))}
	return th.On(th.Background)
}

func markdownFrame(height int) (C, *op.Ops) {
	ops := new(op.Ops)
	return layout.Context{Ops: ops, Now: time.Unix(1, 0), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(600, height)}}, ops
}

func proseText(b markdownBlock) string {
	var s strings.Builder
	for _, span := range b.prose.spans {
		s.WriteString(span.Content)
	}
	return s.String()
}

func TestALongMarkdownFileLaysOutOnlyTheBlocksInItsWindow(t *testing.T) {
	src := strings.Repeat("## Heading\n\nA paragraph of the changelog, long enough to wrap once or twice in the column.\n\n", 1000)
	m := Cache{}
	gtx, _ := markdownFrame(800)
	m.LayoutWindow(gtx, markdownTheme(), "k", src, nil, Window{Top: 0, Bottom: 800})
	laid := 0
	for _, h := range m["k"].heights {
		if h > 0 {
			laid++
		}
	}
	if laid == 0 || laid > 100 {
		t.Errorf("laid out %d of %d blocks for an 800px window, want only the few on screen", laid, len(m["k"].blocks))
	}
}

func TestASoftLineBreakKeepsTheWordsApart(t *testing.T) {
	blocks := renderBlocks(markdownTheme(), "por id de\nconversación, y\n  llega")
	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(blocks))
	}
	if got := proseText(blocks[0]); got != "por id de conversación, y llega" {
		t.Errorf("paragraph = %q, want its lines joined by a space", got)
	}
}

func TestAReferenceLinkResolvesAcrossBlocks(t *testing.T) {
	blocks := renderBlocks(markdownTheme(), "## [0.76.0] - 2026-10-21\n\nBody\n\n[0.76.0]: https://example.com/compare")
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want the heading and the paragraph", len(blocks))
	}
	for _, span := range blocks[0].prose.spans {
		if span.Content == "0.76.0" {
			if !span.Interactive {
				t.Error("the heading's 0.76.0 is not a link")
			}
			return
		}
	}
	t.Errorf("heading spans %+v hold no 0.76.0 link", blocks[0].prose.spans)
}

func TestGFMTablesBecomeTableBlocks(t *testing.T) {
	src := "Intro\n\n| Provider | Works |\n|:--|--:|\n| Claude \\| Code | `yes` |\n| Codex |\n\nAfter\n\n```\n| not | a table |\n|---|---|\n```"
	blocks := renderBlocks(markdownTheme(), src)
	if len(blocks) != 4 || blocks[1].table == nil {
		t.Fatalf("got %d blocks, want intro, table, after and the fenced code", len(blocks))
	}
	rows := blocks[1].table.rows
	if len(rows) != 3 || len(rows[2]) != 2 {
		t.Fatalf("table has %d rows, the short one %d cells; want 3 rows padded to 2 columns", len(rows), len(rows[2]))
	}
	if got := proseText(markdownBlock{prose: rows[1][0]}); got != "Claude | Code" {
		t.Errorf("escaped pipe cell = %q", got)
	}
	if blocks[3].table != nil || !strings.Contains(proseText(blocks[3]), "| not | a table |") {
		t.Errorf("fenced code became %+v, want its lines as prose", blocks[3])
	}
}

func TestMarkdownReadingColorsAndHierarchy(t *testing.T) {
	for _, palette := range []ui.Palette{ui.Dark, ui.Light} {
		th := markdownTheme()
		th.Palette = palette
		blocks := renderBlocks(th, "## Heading\n\nBody with [a link](https://example.com).\n\n| Header |\n|---|\n| Cell |")
		heading := blocks[0].prose.spans[0]
		if heading.Color != th.Foreground || heading.Font.Weight != font.Bold || heading.Size <= BodySize {
			t.Errorf("heading style = %+v, want prominent foreground heading", heading)
		}
		for _, span := range blocks[1].prose.spans {
			want := markdownProseColor(th)
			if span.Interactive {
				want = th.TonePass
			}
			if span.Color != want {
				t.Errorf("span %q color = %v, want %v", span.Content, span.Color, want)
			}
		}
		if got := blocks[2].table.rows[1][0].spans[0].Color; got != markdownProseColor(th) {
			t.Errorf("table cell color = %v, want %v", got, markdownProseColor(th))
		}
	}
}

func TestMarkdownUpdatesWhenInterfaceAndEditorFontsChange(t *testing.T) {
	th := markdownTheme()
	th.Sans, th.Mono = "Interface A", "Editor A"
	cache := Cache{}
	src := "## Heading\n\n- Text `code` after\n\n| Header |\n|---|\n| `cell` |"
	cache.text(th, "body", src).heights[0] = 42
	th.Sans, th.Mono = "Interface B", "Editor B"
	updated := cache.text(th, "body", src)
	if updated.heights[0] != 0 || updated.blocks[0].prose.spans[0].Font.Typeface != th.Sans {
		t.Fatal("heading retained the old interface font or its measurements")
	}
	item := updated.blocks[1].list.items[0].blocks[0].prose
	for i, span := range item.spans {
		want := th.Sans
		if item.code[i] {
			want = th.Mono
		}
		if span.Font.Typeface != want {
			t.Errorf("span %q uses %s, want %s", span.Content, span.Font.Typeface, want)
		}
	}
	cell := updated.blocks[2].table.rows[1][0]
	if cell.spans[0].Font.Typeface != th.Mono {
		t.Errorf("table code uses %s, want %s", cell.spans[0].Font.Typeface, th.Mono)
	}
	updated.heights[0] = 42
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	if cache.text(th, "body", src).heights[0] != 0 {
		t.Fatal("shaper change retained the old measurements")
	}
}

func TestMarkdownRulesUnderTopHeadingsOnly(t *testing.T) {
	blocks := renderBlocks(markdownTheme(), "# One\n\n## Two\n\n### Three\n\nText")
	var got []bool
	for _, b := range blocks {
		got = append(got, b.rule)
	}
	if want := []bool{true, true, false, false}; !slices.Equal(got, want) {
		t.Errorf("rules = %v, want %v", got, want)
	}
}

// The walker visits a fenced block on the way in and on the way out; drawing
// it on both turns showed every fenced block twice (Pulls descriptions too).
func TestAFencedCodeBlockIsDrawnOnce(t *testing.T) {
	blocks := renderBlocks(markdownTheme(), "a\n\n```\npanic: nil map\n```\n\nb")
	var all strings.Builder
	for _, b := range blocks {
		all.WriteString(proseText(b))
	}
	if got := strings.Count(all.String(), "panic: nil map"); got != 1 {
		t.Errorf("the fenced block shows %d times in %q, want once", got, all.String())
	}
}

func TestABlockquoteIsMutedBehindABar(t *testing.T) {
	th := markdownTheme()
	blocks := renderBlocks(th, "Body\n\n> Quoted **words**\n> go on")
	if len(blocks) != 2 || !blocks[1].quote || blocks[0].quote {
		t.Fatalf("blocks = %+v, want a paragraph and a quote", blocks)
	}
	for _, span := range blocks[1].prose.spans {
		if span.Color != th.MutedForeground {
			t.Errorf("quoted span %q color = %v, want muted %v", span.Content, span.Color, th.MutedForeground)
		}
	}
	gtx, _ := markdownFrame(400)
	plain := blocks[1]
	plain.quote = false
	if quoted, bare := blocks[1].layout(gtx, th, nil), plain.layout(gtx, th, nil); quoted.Size.X-bare.Size.X != gtx.Dp(quoteBar+quoteInset) {
		t.Errorf("a quote is %d wide and its text %d, want the bar and inset between them", quoted.Size.X, bare.Size.X)
	}
}

func TestAFencedCodeBlockSitsInAPanel(t *testing.T) {
	th := markdownTheme()
	blocks := renderBlocks(th, "Run `this`:\n\n```go\ngo test ./...\n```\n\n    indented")
	if len(blocks) != 3 || blocks[0].panel || !blocks[1].panel || !blocks[2].panel {
		t.Fatalf("panels = %+v, want the two code blocks alone", blocks)
	}
	gtx, _ := markdownFrame(400)
	plain := blocks[1]
	plain.panel = false
	panel, bare := blocks[1].layout(gtx, th, nil), plain.layout(gtx, th, nil)
	if panel.Size.X != gtx.Constraints.Max.X {
		t.Errorf("a code panel is %d wide, want the body's %d", panel.Size.X, gtx.Constraints.Max.X)
	}
	if panel.Size.Y-bare.Size.Y != 2*gtx.Dp(codePanelPad) {
		t.Errorf("a code panel is %d tall and its text %d, want the padding around it", panel.Size.Y, bare.Size.Y)
	}
}
