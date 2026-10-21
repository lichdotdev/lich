package main

// Markdown to richtext, one block per top-level node, so a long file lays
// out only the blocks on screen. gioui.org/x/markdown renders a whole source
// to one flat run of spans and drops soft line breaks, gluing the words on
// either side of one.

import (
	"bytes"
	"fmt"
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/x/richtext"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/lichdotdev/lich/native/ui"
)

// markdownURL is the span metadata a link's destination is kept under.
const markdownURL = "url"

var markdownParser = goldmark.New(goldmark.WithExtensions(extension.Table, extension.Linkify)).Parser()

// renderBlocks parses src once, so a reference link resolves wherever its
// definition sits, and renders each top-level node as its own block.
func renderBlocks(th *ui.Theme, src string) []markdownBlock {
	source := []byte(src)
	doc := markdownParser.Parse(text.NewReader(source))
	var out []markdownBlock
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		if t, ok := n.(*extast.Table); ok {
			out = append(out, markdownBlock{table: renderTable(th, source, t)})
			continue
		}
		if spans := renderNode(th, source, n, th.MutedForeground); len(spans) > 0 {
			out = append(out, markdownBlock{prose: &markdownCell{spans: spans}})
		}
	}
	return out
}

// spanWriter walks a node into spans: mdProse's text-sm body in fg with the
// links in the foreground, h1 text-lg, h2 text-base, the rest text-sm.
type spanWriter struct {
	th     *ui.Theme
	source []byte
	fg     color.NRGBA
	cur    richtext.SpanStyle
	spans  []richtext.SpanStyle
	lists  []*ast.List
	index  []int
	// itemStart holds a list item's marker as the last span, so the item's
	// first paragraph opens on the marker's line.
	itemStart bool
}

func renderNode(th *ui.Theme, source []byte, n ast.Node, fg color.NRGBA) []richtext.SpanStyle {
	w := &spanWriter{th: th, source: source, fg: fg}
	w.cur = richtext.SpanStyle{Font: font.Font{Typeface: th.Sans}, Size: ui.TextSM, Color: fg}
	_ = ast.Walk(n, w.visit)
	if last := len(w.spans) - 1; last >= 0 {
		w.spans[last].Content = strings.TrimRight(w.spans[last].Content, "\n")
	}
	return w.spans
}

func (w *spanWriter) emit(s string) {
	span := w.cur.DeepCopy()
	span.Content = s
	w.spans = append(w.spans, span)
	w.itemStart = false
}

func (w *spanWriter) endsWith(suffix string) bool {
	return len(w.spans) == 0 || w.itemStart || strings.HasSuffix(w.spans[len(w.spans)-1].Content, suffix)
}

// newline ends the line the last span is on.
func (w *spanWriter) newline() {
	if !w.endsWith("\n") {
		w.spans[len(w.spans)-1].Content += "\n"
	}
}

// separate leaves a blank line between the last span and the next block.
func (w *spanWriter) separate() {
	if w.endsWith("\n\n") {
		return
	}
	w.newline()
	w.spans[len(w.spans)-1].Content += "\n"
}

func (w *spanWriter) visit(n ast.Node, entering bool) (ast.WalkStatus, error) {
	switch n := n.(type) {
	case *ast.Heading:
		w.heading(n, entering)
	case *ast.Paragraph:
		if entering {
			w.separate()
		}
	case *ast.List:
		w.list(n, entering)
	case *ast.ListItem:
		if entering {
			w.listItem()
		}
	case *ast.CodeBlock, *ast.FencedCodeBlock:
		w.codeBlock(n)
		return ast.WalkSkipChildren, nil
	case *ast.HTMLBlock, *ast.RawHTML:
		return ast.WalkSkipChildren, nil
	case *ast.CodeSpan:
		w.monospace(entering)
	case *ast.Emphasis:
		w.emphasis(n, entering)
	case *ast.Link:
		w.link(string(n.Destination), entering)
	case *ast.AutoLink:
		if entering {
			url := string(n.URL(w.source))
			w.link(url, true)
			w.emit(url)
			w.link("", false)
		}
		return ast.WalkSkipChildren, nil
	case *ast.Text:
		if entering {
			w.text(n)
		}
	case *ast.String:
		if entering {
			w.emit(string(n.Value))
		}
	}
	return ast.WalkContinue, nil
}

func (w *spanWriter) heading(n *ast.Heading, entering bool) {
	if !entering {
		w.cur.Size = ui.TextSM
		return
	}
	w.separate()
	switch n.Level {
	case 1:
		w.cur.Size = markdownH1
	case 2:
		w.cur.Size = ui.TextBase
	}
}

func (w *spanWriter) list(n *ast.List, entering bool) {
	if !entering {
		w.lists, w.index = w.lists[:len(w.lists)-1], w.index[:len(w.index)-1]
		return
	}
	if len(w.lists) == 0 {
		w.separate()
	}
	w.lists, w.index = append(w.lists, n), append(w.index, n.Start)
}

func (w *spanWriter) listItem() {
	w.newline()
	depth := len(w.lists) - 1
	marker := " • "
	if w.lists[depth].IsOrdered() {
		marker = fmt.Sprintf(" %d. ", w.index[depth])
		w.index[depth]++
	}
	w.emit(strings.Repeat("   ", depth) + marker)
	w.itemStart = true
}

func (w *spanWriter) codeBlock(n ast.Node) {
	w.separate()
	w.monospace(true)
	lines := n.Lines()
	for i := range lines.Len() {
		seg := lines.At(i)
		w.emit(string(seg.Value(w.source)))
	}
	w.monospace(false)
}

func (w *spanWriter) monospace(on bool) {
	w.cur.Font.Typeface = w.th.Sans
	if on {
		w.cur.Font.Typeface = w.th.Mono
	}
}

func (w *spanWriter) emphasis(n *ast.Emphasis, entering bool) {
	switch {
	case n.Level == 2 && entering:
		w.cur.Font.Weight = font.Bold
	case n.Level == 2:
		w.cur.Font.Weight = font.Normal
	case entering:
		w.cur.Font.Style = font.Italic
	default:
		w.cur.Font.Style = font.Regular
	}
}

func (w *spanWriter) link(url string, entering bool) {
	w.cur.Interactive = entering
	w.cur.Set(markdownURL, url)
	w.cur.Color = w.fg
	if entering {
		w.cur.Color = w.th.Foreground
	}
}

func (w *spanWriter) text(n *ast.Text) {
	content := n.Segment.Value(w.source)
	if !n.IsRaw() {
		content = util.ResolveEntityNames(util.ResolveNumericReferences(util.UnescapePunctuations(content)))
	}
	switch {
	case n.HardLineBreak():
		w.emit(string(bytes.TrimRight(content, " ")) + "\n")
	case n.SoftLineBreak():
		w.emit(string(bytes.TrimRight(content, " ")) + " ")
	default:
		w.emit(string(content))
	}
}
