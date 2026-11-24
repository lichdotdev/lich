package main

// The stage editor's Markdown preview: the open file rendered as the Review
// tab renders a Markdown card, in place of its source or beside it.

import (
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/markdown"
)

// markdownMode is what the editor shows of a Markdown file.
type markdownMode int

const (
	mdSource markdownMode = iota
	mdRendered
	mdSplit
)

// codeMarkdown is the editor's rendering state; each file opens in
// markdownModeFor's mode.
type codeMarkdown struct {
	mode           markdownMode
	toggle, splits ui.IconActionState
	rendered       markdown.Cache
	scroll         widget.List
}

// markdownModeFor is how rel opens: a Markdown file rendered, unless it
// opens on lines or with a note, which only its source shows.
func markdownModeFor(rel string, line int, note agentNote) markdownMode {
	if isMarkdown(rel) && line == 0 && note.text == "" {
		return mdRendered
	}
	return mdSource
}

// markdownToggles are the header's Preview and Split actions; pressing the
// one that is on goes back to the source.
func (p *fileEditor) markdownToggles(gtx C, th *ui.Theme, root *ui.Root) D {
	if p.md.toggle.Click.Clicked(gtx) {
		p.md.mode = p.md.pick(mdRendered)
	}
	if p.md.splits.Click.Clicked(gtx) {
		p.md.mode = p.md.pick(mdSplit)
	}
	label, splitLabel := i18n.T("common.action.previewMarkdown"), i18n.T("diff.dockReviewBody.split")
	switch p.md.mode {
	case mdRendered:
		label = i18n.T("dock.filesPanel.showSource")
	case mdSplit:
		splitLabel = i18n.T("dock.filesPanel.showSource")
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return th.IconAction(root, &p.md.splits, icSplit, splitLabel, p.md.mode == mdSplit).Layout(gtx)
		}),
		ui.Gap(1),
		layout.Rigid(func(gtx C) D {
			return th.IconAction(root, &p.md.toggle, icMarkdownPreview, label, p.md.mode == mdRendered).Layout(gtx)
		}),
	)
}

func (m *codeMarkdown) pick(mode markdownMode) markdownMode {
	if m.mode == mode {
		return mdSource
	}
	return mode
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
