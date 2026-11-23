package markdown

import (
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/x/richtext"
	"github.com/lichdotdev/godemirror"
	"github.com/yuin/goldmark/ast"

	"github.com/lichdotdev/lich/native/ui"
)

// fenceExtensions are the fence words that name a language by something
// other than its file extension, which godemirror picks a language by.
var fenceExtensions = map[string]string{
	"golang": "go", "typescript": "ts", "javascript": "js", "python": "py",
	"rust": "rs", "kotlin": "kt", "shell": "sh", "bash": "sh", "zsh": "sh", "console": "sh",
	"yml": "yaml", "markdown": "md", "c++": "cpp", "csharp": "cs",
}

// fenceFilename is the filename godemirror is handed for a fence's info
// string: its first word as the extension, "" for a fence with none.
func fenceFilename(info string) string {
	word, _, _ := strings.Cut(strings.TrimSpace(info), " ")
	word = strings.ToLower(word)
	if word == "" {
		return ""
	}
	if ext, ok := fenceExtensions[word]; ok {
		word = ext
	}
	return "code." + word
}

// highlightCode is a fenced block colored as the code views color it, nil
// for a block godemirror gives no colors to (no language, or one it does not
// know), which stays plain as prose's code.
func highlightCode(th *ui.Theme, source []byte, n ast.Node) *markdownCell {
	fenced, ok := n.(*ast.FencedCodeBlock)
	if !ok || th.Code.Highlight == nil {
		return nil
	}
	filename := fenceFilename(string(fenced.Language(source)))
	if filename == "" {
		return nil
	}
	lines, err := godemirror.Highlight(filename, rawCode(source, n), nil, th.Code)
	if err != nil || plainRuns(lines, th.Code.Foreground) {
		return nil
	}
	cell := &markdownCell{}
	base := richtext.SpanStyle{Font: font.Font{Typeface: th.Mono}, Size: BodySize}
	for i, runs := range lines {
		if i > 0 {
			cell.add(base, "\n")
		}
		for _, r := range runs {
			span := base
			span.Color = r.Style.Color
			if r.Style.Bold {
				span.Font.Weight = font.Bold
			}
			if r.Style.Italic {
				span.Font.Style = font.Italic
			}
			cell.add(span, r.Text)
		}
	}
	return cell
}

func (c *markdownCell) add(span richtext.SpanStyle, content string) {
	span.Content = content
	c.spans = append(c.spans, span)
	c.code = append(c.code, false)
	c.decor = append(c.decor, decoration{})
}

// plainRuns is whether every run is fg, as godemirror draws a language it
// does not know.
func plainRuns(lines [][]godemirror.Run, fg color.NRGBA) bool {
	for _, runs := range lines {
		for _, r := range runs {
			if r.Style.Color != fg || r.Style.Bold || r.Style.Italic {
				return false
			}
		}
	}
	return true
}
