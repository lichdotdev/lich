package main

import (
	"image"

	"gioui.org/layout"

	"github.com/lichdotdev/godemirror"
	"github.com/lichdotdev/lich/native/fonts"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/codeview"
	"github.com/lichdotdev/lich/native/ui/markdown"
)

const richMarkdown = "## Markdown\n\nA paragraph with **bold**, *italics*, `inline code` and a [link](https://example.com).\n\n" +
	"- [x] a task done\n- [ ] a task to do\n  - and a nested item\n\n| Viewer | Input |\n|---|---|\n| Markdown | text |\n| Code | source and filename |\n"

// richState holds what the rich content viewers keep between frames.
type richState struct {
	md      markdown.Cache
	code    *codeview.View
	codeErr error
}

func (g *gallery) markdownSection(gtx C, th *ui.Theme) D {
	if g.rich.md == nil {
		g.rich.md = markdown.Cache{}
	}
	return g.rich.md.Layout(gtx, th, "gallery", richMarkdown, nil)
}

const richSource = `// Package demo is what the code view draws in the gallery.
package demo

import "fmt"

// Greet says hello to name.
func Greet(name string) string {
	return fmt.Sprintf("hello, %s", name)
}
`

const richCodeHeight = 240

func (g *gallery) codeSection(gtx C, th *ui.Theme) D {
	r := &g.rich
	if r.code == nil && r.codeErr == nil {
		r.code, r.codeErr = codeview.New(fonts.SystemMono, 13)
		if r.codeErr == nil {
			r.code.SetSource(codeview.Source{Filename: "demo.go", Text: richSource})
		}
	}
	if r.codeErr != nil {
		return th.Text(ui.TextSM, r.codeErr.Error()).Muted().Layout(gtx)
	}
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(richCodeHeight)))
	dims, err := r.code.Layout(gtx, th, godemirror.DarkTheme)
	if err != nil {
		r.codeErr = err
	}
	return dims
}
