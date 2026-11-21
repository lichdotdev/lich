package main

import (
	"os"
	"path/filepath"

	"gioui.org/layout"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
)

// attachPreview is the dialog a card opens: the file's name over its path,
// and an image whole, or the file's size.
type attachPreview struct {
	dialog ui.DialogState
	open   bool
	path   string
	size   int64
	sizeOK bool
}

// show opens the dialog on path, its image read as the card's was.
func (a *attachPreview) show(u *window, path string) {
	a.open, a.path, a.sizeOK = true, path, false
	if info, err := os.Stat(path); err == nil {
		a.size, a.sizeOK = info.Size(), true
	}
	u.thumb(path)
}

func (u *window) layoutAttachPreview(gtx C, th *ui.Theme) {
	a := &u.attachPreview
	d := th.Dialog(&a.dialog, a.open, filepath.Base(a.path), a.path)
	d.MaxWidth, d.Root = ui.DialogWidthXL, &u.root
	on := th.On(th.Popover)
	d.Body = func(gtx C) D {
		if r, ok := u.chats.thumbs[a.path]; ok {
			img, loading, err := r.get()
			switch {
			case img != nil:
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return img.draw(gtx, on) }),
					ui.VGap(1),
					layout.Rigid(on.Text(ui.Text2XS, img.caption()).Mono().Muted().Leading(text2XSLeading).Layout),
				)
			case err != nil:
				l := on.Text(ui.TextXS, backend.ErrorText(err)).Muted()
				l.MaxLines = 0
				return l.Layout(gtx)
			case loading:
				return D{}
			}
		}
		if !a.sizeOK {
			return D{}
		}
		return on.Text(ui.TextXS, formatBlobSize(int(a.size))).Mono().Muted().Layout(gtx)
	}
	if d.Layout(gtx) {
		a.open = false
	}
}
