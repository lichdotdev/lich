package main

// The stage editor's Markdown preview: the open file rendered as the Review
// tab renders a Markdown card, in place of its source.

import (
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/markdown"
)

// codeMarkdown is the editor's rendering state; rendering goes back to the
// source whenever another file opens.
type codeMarkdown struct {
	rendering bool
	toggle    ui.IconActionState
	rendered  markdown.Cache
	scroll    widget.List
}

func (p *fileEditor) markdownToggle(gtx C, th *ui.Theme, root *ui.Root) D {
	label := i18n.T("common.action.previewMarkdown")
	if p.md.rendering {
		label = i18n.T("dock.filesPanel.showSource")
	}
	return th.IconAction(root, &p.md.toggle, icMarkdownPreview, label, p.md.rendering).Layout(gtx)
}

// markdownBody renders what the editor holds, unsaved changes included.
func (p *fileEditor) markdownBody(gtx C, th *ui.Theme) D {
	text := p.ed.State().Doc()
	inset, viewport := ui.Space(3), gtx.Constraints.Max.Y
	return th.ScrollArea(&p.md.scroll).Layout(gtx, 1, func(gtx C, _ int) D {
		window := markdown.ViewportWindow(p.md.scroll.Position.Offset-gtx.Dp(inset), viewport)
		return layout.UniformInset(inset).Layout(gtx, func(gtx C) D {
			return p.md.rendered.LayoutWindow(gtx, th, p.rel, text, func(url string) { openLink(p.client, p.env.notify, url) }, window)
		})
	})
}
