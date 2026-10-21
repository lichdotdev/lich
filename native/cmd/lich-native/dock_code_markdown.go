package main

// The Code tab's Markdown preview: the open file rendered as the Review tab
// renders a Markdown card, in place of its source.

import (
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
	"github.com/lichdotdev/lich/native/ui"
)

// codeMarkdown is the preview's rendering state; rendering goes back to the
// source whenever another file opens.
type codeMarkdown struct {
	rendering bool
	toggle    iconActionState
	rendered  markdownCache
	scroll    widget.List
}

func (p *preview) markdownToggle(gtx C, th *ui.Theme, root *ui.Root) D {
	label := i18n.T("diff.fileDiff.previewMarkdown")
	if p.md.rendering {
		label = i18n.T("dock.filesPanel.showSource")
	}
	return iconAction(gtx, th, root, &p.md.toggle, icMarkdownPreview, label, p.md.rendering)
}

func (p *preview) markdownBody(gtx C, th *ui.Theme) D {
	text, loading, err := p.file.get()
	switch {
	case err != nil:
		return binaryState(gtx, th, lichclient.ErrorText(err), "", &p.exits)
	case loading:
		return notice(gtx, th, i18n.T("dock.filesPanel.loading"))
	}
	inset, viewport := ui.Space(3), gtx.Constraints.Max.Y
	return th.ScrollArea(&p.md.scroll).Layout(gtx, 1, func(gtx C, _ int) D {
		window := viewportWindow(p.md.scroll.Position.Offset-gtx.Dp(inset), viewport)
		return layout.UniformInset(inset).Layout(gtx, func(gtx C) D {
			return p.md.rendered.layoutWindow(gtx, th, p.rel, text, func(url string) { openLink(p.client, p.env.notify, url) }, window)
		})
	})
}
