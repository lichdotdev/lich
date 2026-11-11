package main

import (
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/markdown"
)

const richMarkdown = "## Markdown\n\nA paragraph with **bold**, *italics*, `inline code` and a [link](https://example.com).\n\n" +
	"- [x] a task done\n- [ ] a task to do\n  - and a nested item\n\n| Viewer | Input |\n|---|---|\n| Markdown | text |\n| Code | source and filename |\n"

// richState holds what the rich content viewers keep between frames.
type richState struct {
	md markdown.Cache
}

func (g *gallery) markdownSection(gtx C, th *ui.Theme) D {
	if g.rich.md == nil {
		g.rich.md = markdown.Cache{}
	}
	return g.rich.md.Layout(gtx, th, "gallery", richMarkdown, nil)
}
