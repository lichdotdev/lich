package main

// A Markdown card's preview: the file's new version rendered as the Pulls
// screen renders a pull request's body, in place of its diff.

import (
	"context"
	"strings"

	"gioui.org/layout"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var icMarkdownPreview = icons.Lucide("book-open")

// markdownPreview is a card's preview state: the working tree's read of the
// file and its rendered blocks.
type markdownPreview struct {
	read     remote[string]
	rendered markdownCache
}

// previewsMarkdown reports whether the card offers its file rendered: a
// Markdown file whose new version this window can read. A pull request's
// clone holds none of its blobs (docs/ceilings.md), so there only an added
// file, whose patch is the whole file, has one.
func (c *fileCard) previewsMarkdown() bool {
	if c.git.binary || c.git.status == statusDeleted || !isMarkdown(c.git.path) {
		return false
	}
	return c.panel.pr == nil || c.git.status == statusAdded
}

func isMarkdown(path string) bool {
	switch extname(path) {
	case "md", "markdown":
		return true
	}
	return false
}

// addedText is an added file's content: its patch's lines without their "+",
// git's "\ No newline at end of file" left out.
func addedText(f patchFile) string {
	var b strings.Builder
	for _, h := range f.hunks {
		for _, l := range h.lines {
			if strings.HasPrefix(l, "+") {
				b.WriteString(l[1:] + "\n")
			}
		}
	}
	return b.String()
}

func (c *fileCard) previewToggle(gtx C, th *ui.Theme, p *reviewPanel) D {
	label := i18n.T("diff.fileDiff.previewMarkdown")
	if c.previewing {
		label = i18n.T("diff.fileDiff.showDiff")
	}
	return iconAction(gtx, th, p.root, &c.preview, icMarkdownPreview, label, c.previewing)
}

// markdownBody is the preview, px-9 like the lines a body without a diff
// shows.
func (c *fileCard) markdownBody(gtx C, p *reviewPanel) D {
	if c.md == nil {
		c.md = &markdownPreview{rendered: markdownCache{}}
		c.md.read.invalidate = p.invalidate
	}
	src, loading, err := c.markdownSource(p)
	switch {
	case err != nil:
		return bodyLine(gtx, p.th, lichclient.ErrorText(err))
	case loading:
		return bodyLine(gtx, p.th, i18n.T("diff.binaryPreview.loading"))
	}
	inset := layout.Inset{Left: ui.Space(9), Right: ui.Space(9), Top: ui.Space(3), Bottom: ui.Space(4)}
	window := viewportWindow(-(p.itemTop(p.laying) + c.headerHeight + gtx.Dp(inset.Top)), p.viewport)
	return inset.Layout(gtx, func(gtx C) D {
		return c.md.rendered.layoutWindow(gtx, p.th, c.git.path, src, func(url string) { openLink(p.client, p.notify, url) }, window)
	})
}

// markdownSource is the file's new version: an added file's patch, or the
// working tree's file, read again when the index line names a new blob.
func (c *fileCard) markdownSource(p *reviewPanel) (src string, loading bool, err error) {
	if c.git.status == statusAdded {
		return addedText(c.git), false, nil
	}
	client, checkout, rel := p.client, p.target, c.git.path
	key := strings.Join([]string{checkout, rel, c.git.blobs}, "\x00")
	c.md.read.want(key, checkout, func(ctx context.Context) (string, error) {
		var text string
		return text, client.Call(ctx, "project.ReadFile", &text, checkout, rel)
	})
	return c.md.read.get()
}

// openLink sends a link clicked in a Markdown preview to the browser.
func openLink(client *lichclient.Client, notify func(ui.Toast), url string) {
	go func() {
		if err := client.OpenExternal(context.Background(), url); err != nil {
			notify(ui.Toast{Variant: ui.ToastError, Title: i18n.T("diff.binaryState.openFailed", i18n.Args{"name": url}), Description: lichclient.ErrorText(err)})
		}
	}()
}
