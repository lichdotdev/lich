package main

// The Artifacts tab's reader: a Markdown artifact rendered in the tab, as the
// Code tab previews one, since the browser would only show its source.

import (
	"context"
	"image"
	"path/filepath"
	"strconv"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/markdown"
)

type artifactReader struct {
	client *backend.Client
	notify func(ui.Toast)

	// active is the session the tab showed when the file opened; the reader
	// closes once the tab shows another.
	active, owner, name string
	path                string // "" while closed

	file     remote[string]
	rendered markdown.Cache
	scroll   widget.List
	back     widget.Clickable
	backTip  ui.Tooltip
}

func newArtifactReader(client *backend.Client, notify func(ui.Toast), invalidate func()) *artifactReader {
	r := &artifactReader{client: client, notify: notify, rendered: markdown.Cache{}}
	r.file.invalidate = invalidate
	r.scroll.Axis = layout.Vertical
	return r
}

func (r *artifactReader) openAt(active, owner string, f backend.Artifact) {
	r.active, r.owner, r.name, r.path = active, owner, f.Name, f.Path
	r.scroll.Position = layout.Position{}
}

func (r *artifactReader) close() {
	r.path = ""
	r.file.stop()
}

// isOpenFor is whether the reader shows a file on the tab of session active.
func (r *artifactReader) isOpenFor(active string) bool {
	if r.path != "" && r.active != active {
		r.close()
	}
	return r.path != ""
}

// layout reads the file again each time the agent rewrites it, and goes back
// to the list once it is gone from the owner's folder.
func (r *artifactReader) layout(gtx C, th *ui.Theme, root *ui.Root, store *artifactStore) D {
	f, ok := findArtifact(store.of(r.owner), r.name)
	if !ok || r.back.Clicked(gtx) {
		r.close()
		return D{Size: gtx.Constraints.Max}
	}
	store.markRead(r.owner, r.name)
	key, client := f.Path+"\x00"+strconv.FormatInt(f.Modified, 10), r.client
	r.file.want(key, f.Path, func(ctx context.Context) (string, error) {
		return client.ReadFile(ctx, filepath.Dir(f.Path), filepath.Base(f.Path))
	})
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return r.header(gtx, th, root) }),
		layout.Rigid(th.Separator),
		layout.Flexed(1, func(gtx C) D {
			gtx.Constraints.Min = gtx.Constraints.Max
			return r.body(gtx, th)
		}),
	)
}

func findArtifact(files []backend.Artifact, name string) (backend.Artifact, bool) {
	for _, f := range files {
		if f.Name == name {
			return f, true
		}
	}
	return backend.Artifact{}, false
}

// header is the Code preview's h-8 strip: back and the file's name in mono.
func (r *artifactReader) header(gtx C, th *ui.Theme, root *ui.Root) D {
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(ui.Space(8))))
	return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
		return layout.W.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return r.backButton(gtx, th, root) }),
				ui.Gap(1.5),
				layout.Flexed(1, func(gtx C) D {
					l := th.Text(ui.TextXS, r.name).Mono()
					l.MaxLines = 1
					return l.Layout(gtx)
				}),
			)
		})
	})
}

func (r *artifactReader) backButton(gtx C, th *ui.Theme, root *ui.Root) D {
	return th.Tooltip(root, &r.backTip, i18n.T("dock.artifacts.backToList")).Layout(gtx, func(gtx C) D {
		return r.back.Layout(gtx, func(gtx C) D {
			side := gtx.Dp(ui.Space(5))
			gtx.Constraints = layout.Exact(image.Pt(side, side))
			fg := th.MutedForeground
			if r.back.Hovered() {
				ui.Fill(gtx, th.Accent, ui.RadiusSM)
				fg = th.AccentForeground
			}
			return layout.Center.Layout(gtx, func(gtx C) D { return icPreviewBack.Layout(gtx, ui.Space(4), fg) })
		})
	})
}

func (r *artifactReader) body(gtx C, th *ui.Theme) D {
	text, loading, err := r.file.get()
	switch {
	case err != nil:
		return th.Notice(backend.ErrorText(err)).Layout(gtx)
	case loading:
		return th.Notice(i18n.T("dock.filesPanel.loading")).Layout(gtx)
	}
	inset, viewport := ui.Space(3), gtx.Constraints.Max.Y
	return th.ScrollArea(&r.scroll).Layout(gtx, 1, func(gtx C, _ int) D {
		window := markdown.ViewportWindow(r.scroll.Position.Offset-gtx.Dp(inset), viewport)
		return layout.UniformInset(inset).Layout(gtx, func(gtx C) D {
			return r.rendered.LayoutWindow(gtx, th, r.path, text, func(url string) { openLink(r.client, r.notify, url) }, window)
		})
	})
}
