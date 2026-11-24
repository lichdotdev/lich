package markdown

import (
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/ui"
)

// look is how a body is set. Inline is Markdown inside a screen, a message or
// a comment, kept dense; a document is a file opened to read.
type look struct {
	// headings is the size of an h1, an h2, and an h3 or deeper.
	headings [3]unit.Sp
	// above and below are the room, in ui.Space steps, before a heading and
	// between it and what it opens.
	above, below float32
	// column is the widest a line runs, 0 for the body's width.
	column      unit.Dp
	frontmatter bool
}

var (
	inlineLook   = look{headings: [3]unit.Sp{20, 18, ui.TextBase}, above: 3, below: 1}
	documentLook = look{headings: [3]unit.Sp{26, 20, ui.TextBase}, above: 7, below: 2, column: 680, frontmatter: true}
)

// listItemGap is the room, in ui.Space steps, between a list's items.
const listItemGap = 1.5

func (lk look) headingSize(level int) unit.Sp {
	return lk.headings[min(level, len(lk.headings))-1]
}

// gapBefore is the room, in ui.Space steps, above blocks[i].
func (lk look) gapBefore(blocks []markdownBlock, i int) float32 {
	switch {
	case blocks[i].heading:
		return lk.above
	case blocks[i-1].heading:
		return lk.below
	}
	return BlockGap
}

// resize sets every span of a heading's cell to size.
func (c *markdownCell) resize(size unit.Sp) {
	for i := range c.spans {
		c.spans[i].Size = size
	}
}

// property is one key of a document's frontmatter.
type property struct{ key, value string }

// splitFrontmatter is src's frontmatter, the block between a first line of
// --- and the next, as properties, and the Markdown after it. src with none,
// or with lines in it that are no "key: value", is all Markdown.
//
// shortcut: reads top-level keys and folds indented or "- " lines into the
// key above; nested maps flatten to text. A YAML parser if that misreads a
// real file.
func splitFrontmatter(src string) ([]property, string) {
	lines := strings.SplitAfter(src, "\n")
	if strings.TrimRight(lines[0], "\r\n") != "---" {
		return nil, src
	}
	for end := 1; end < len(lines); end++ {
		if fence := strings.TrimRight(lines[end], "\r\n"); fence == "---" || fence == "..." {
			props := parseProperties(lines[1:end])
			if props == nil {
				return nil, src
			}
			return props, strings.Join(lines[end+1:], "")
		}
	}
	return nil, src
}

func parseProperties(lines []string) []property {
	var props []property
	for _, line := range lines {
		line = strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if line[0] == ' ' || line[0] == '\t' || strings.HasPrefix(line, "- ") {
			if len(props) == 0 {
				return nil
			}
			last := &props[len(props)-1]
			last.value = strings.TrimSpace(last.value + " " + strings.TrimPrefix(strings.TrimSpace(line), "- "))
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.ContainsAny(key, " \t") {
			return nil
		}
		props = append(props, property{key: key, value: unquote(strings.TrimSpace(value))})
	}
	return props
}

// unquote is a value without the quotes around it, and without the > or |
// that only says how the lines under it fold.
func unquote(v string) string {
	if v == ">" || v == "|" || v == ">-" || v == "|-" {
		return ""
	}
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
		return v[1 : len(v)-1]
	}
	return v
}

// propsKeyWidth is the column a property's key takes, in ui.Space steps.
const propsKeyWidth = 28

// layoutProps draws a document's frontmatter as a table of keys and values,
// each value on at most two lines, over a separator.
func layoutProps(gtx C, th *ui.Theme, props []property) D {
	children := make([]layout.FlexChild, 0, 2*len(props)+2)
	for i, p := range props {
		if i > 0 {
			children = append(children, ui.VGap(1))
		}
		children = append(children, layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Start}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					width := gtx.Dp(ui.Space(propsKeyWidth))
					gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, min(width, gtx.Constraints.Max.X)
					return th.Text(ui.TextSM, p.key).Mono().Muted().Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: ui.Space(3)}.Layout),
				layout.Flexed(1, func(gtx C) D {
					label := th.Text(ui.TextSM, p.value).In(markdownProseColor(th))
					label.MaxLines = 2
					return label.Layout(gtx)
				}),
			)
		}))
	}
	children = append(children, ui.VGap(4), layout.Rigid(th.Separator))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}
