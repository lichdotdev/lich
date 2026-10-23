package main

import (
	"path"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// commentEditDialog rewords the batch comment a pencil was clicked on, in a
// textarea big enough for a long one. One per window: every surface's batch
// opens it through the shared commentStore. Ctrl+Enter saves, Enter is a new
// line.
type commentEditDialog struct {
	dialog       ui.DialogState
	editor       widget.Editor
	resize       ui.TextareaResize
	save, cancel widget.Clickable
	// shown is the comment the editor was filled from, and where is its
	// anchor, kept so the panel still reads right while it closes.
	shown int
	where string
}

// layout is laid out by the window before its content, as every dialog is.
func (d *commentEditDialog) layout(gtx C, th *ui.Theme, store *commentStore) {
	c, open := store.edited()
	if !open && !d.dialog.Animating(gtx) {
		return
	}
	if open && d.shown != c.id {
		d.fill(gtx, c)
	}
	text := strings.TrimSpace(d.editor.Text())
	if (d.save.Clicked(gtx) || ctrlEnter(gtx, &d.editor)) && open && text != "" {
		store.edit(store.editing.target, c.id, text)
		open = false
	}
	if d.cancel.Clicked(gtx) {
		open = false
	}
	on := th.On(th.Popover)
	cancel := on.Button(&d.cancel, i18n.T("common.action.cancel"))
	cancel.Variant = ui.VariantOutline
	save := on.Button(&d.save, i18n.T("diff.commentEdit.save"))
	save.Disabled = text == ""
	dlg := th.Dialog(&d.dialog, open, i18n.T("diff.commentEdit.title"), d.where)
	dlg.MaxWidth = ui.DialogWidthXL
	dlg.Body = func(gtx C) D {
		gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(ui.Space(64)))
		ta := on.Textarea(&d.editor, "")
		ta.Resize = &d.resize
		return ta.Layout(gtx)
	}
	dlg.Actions = []layout.Widget{cancel.Layout, save.Layout}
	if dlg.Layout(gtx) {
		open = false
	}
	if !open {
		store.endEdit()
		d.shown = 0
	}
}

func (d *commentEditDialog) fill(gtx C, c reviewComment) {
	d.shown = c.id
	d.where = ""
	if c.path != "" {
		d.where = path.Base(c.path) + ":" + c.lines
	}
	d.editor.SetText(c.txt)
	d.editor.SetCaret(d.editor.Len(), d.editor.Len())
	gtx.Execute(key.FocusCmd{Tag: &d.editor})
}
