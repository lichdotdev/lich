package markdown

import (
	"slices"
	"testing"

	"gioui.org/unit"
)

func TestSplitFrontmatter(t *testing.T) {
	src := "---\nname: lich-ux-declutter\ndescription: >\n  Cut a screen\n  down.\ntags:\n  - ui\n  - native\nquoted: \"a: b\"\n---\n\n# Declutter\n"
	props, body := splitFrontmatter(src)
	want := []property{
		{"name", "lich-ux-declutter"},
		{"description", "Cut a screen down."},
		{"tags", "ui native"},
		{"quoted", "a: b"},
	}
	if !slices.Equal(props, want) {
		t.Errorf("props = %q, want %q", props, want)
	}
	if body != "\n# Declutter\n" {
		t.Errorf("body = %q", body)
	}
}

func TestSplitFrontmatterLeavesMarkdownAlone(t *testing.T) {
	for _, src := range []string{
		"# Title\n\n---\nname: x\n---\n",
		"---\nname: x\n",
		"---\nJust a thematic break and a setext heading\n---\n",
		"",
	} {
		if props, body := splitFrontmatter(src); props != nil || body != src {
			t.Errorf("splitFrontmatter(%q) = %q, %q; want it untouched", src, props, body)
		}
	}
}

func TestSplitFrontmatterReadsCRLF(t *testing.T) {
	props, body := splitFrontmatter("---\r\nname: x\r\n---\r\nText")
	if !slices.Equal(props, []property{{"name", "x"}}) || body != "Text" {
		t.Errorf("got %q, %q", props, body)
	}
}

// A skill file's frontmatter, an h2 by the setext rule, is its properties in
// a document and stays text inside a message.
func TestFrontmatterOnlyInADocument(t *testing.T) {
	src := "---\nname: x\n---\n\n# Title"
	doc := renderLook(markdownTheme(), src, documentLook)
	if len(doc) != 2 || doc[0].props == nil || !doc[1].heading {
		t.Fatalf("document blocks = %+v, want properties then the title", doc)
	}
	inline := renderLook(markdownTheme(), src, inlineLook)
	if inline[0].props != nil {
		t.Error("a message read its opening --- as frontmatter")
	}
}

func TestHeadingSizesFollowTheLook(t *testing.T) {
	src := "# One\n\n## Two\n\n#### Four"
	for _, lk := range []look{inlineLook, documentLook} {
		blocks := renderLook(markdownTheme(), src, lk)
		for i, want := range []unit.Sp{lk.headings[0], lk.headings[1], lk.headings[2]} {
			for _, span := range blocks[i].prose.spans {
				if span.Size != want {
					t.Errorf("heading %d at %v, want %v", i, span.Size, want)
				}
			}
		}
	}
}

func TestAHeadingSitsCloserToWhatItOpens(t *testing.T) {
	blocks := renderBlocks(markdownTheme(), "Intro\n\n## Two\n\nBody\n\nMore")
	var got []float32
	for i := 1; i < len(blocks); i++ {
		got = append(got, inlineLook.gapBefore(blocks, i))
	}
	if want := []float32{inlineLook.above, inlineLook.below, BlockGap}; !slices.Equal(got, want) {
		t.Errorf("gaps = %v, want %v", got, want)
	}
	if documentLook.above <= BlockGap || documentLook.below >= BlockGap {
		t.Error("a document's heading is no closer to its body than to the text above")
	}
}
