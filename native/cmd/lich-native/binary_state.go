package main

// What a binary file with nothing to draw offers instead: its own words for
// why, and the way out to the desktop's application for its type, or to the
// folder holding it.

import (
	"context"
	"path/filepath"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// fileExits are the two buttons out of lich for one work-tree file.
type fileExits struct {
	open, reveal widget.Clickable
}

// update runs this frame's clicks against rel in checkout, off the UI
// goroutine; a failure comes back as a toast.
func (e *fileExits) update(gtx C, client *backend.Client, notify func(ui.Toast), checkout, rel string) {
	_, name := splitPath(rel)
	if e.open.Clicked(gtx) {
		go func() {
			if err := client.OpenFile(context.Background(), checkout, rel); err != nil {
				notify(ui.Toast{Variant: ui.ToastError, Title: i18n.T("diff.binaryState.openFailed", i18n.Args{"name": name}), Description: backend.ErrorText(err)})
			}
		}()
	}
	if e.reveal.Clicked(gtx) {
		dir := filepath.Join(checkout, filepath.Dir(filepath.FromSlash(rel)))
		go func() {
			if err := client.OpenFolder(context.Background(), dir); err != nil {
				notify(ui.Toast{Variant: ui.ToastError, Title: i18n.T("diff.binaryState.revealFailed", i18n.Args{"name": name}), Description: backend.ErrorText(err)})
			}
		}()
	}
}

func (e *fileExits) layout(gtx C, th *ui.Theme) D {
	open := th.Button(&e.open, i18n.T("diff.binaryState.open"))
	open.Size = ui.SizeXS
	reveal := th.Button(&e.reveal, i18n.T("diff.binaryState.reveal"))
	reveal.Size, reveal.Variant = ui.SizeXS, ui.VariantOutline
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(open.Layout),
		ui.Gap(1.5),
		layout.Rigid(reveal.Layout),
	)
}

// binaryState is a file the preview cannot draw: title, an optional hint
// under it, and the exits.
func binaryState(gtx C, th *ui.Theme, title, hint string, exits *fileExits) D {
	return th.InlineEmptyState(title, hint, func(gtx C) D { return exits.layout(gtx, th) }).Layout(gtx)
}
