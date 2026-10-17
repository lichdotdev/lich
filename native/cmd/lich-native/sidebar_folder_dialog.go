package main

import (
	"slices"
	"strings"

	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// folderNaming is NewFolderDialog's state: the sessions a new folder will
// hold once it is named, one card or a whole checkout's block. A folder is
// only a name, so filing the first session under it is what creates it.
type folderNaming struct {
	ids    []string
	open   bool
	focus  bool // the field asks for focus the frame after opening (autoFocus)
	dialog ui.DialogState
	name   widget.Editor
	cancel widget.Clickable
	create widget.Clickable
}

// askFolderName opens the dialog for ids, the field emptied: the dialog
// outlives one naming, and a second card sent here must not arrive holding
// the abandoned text.
func (s *sidebar) askFolderName(ids []string) {
	n := &s.naming
	n.ids, n.open, n.focus = ids, true, true
	n.name.SetText("")
}

// folderCommit is what the dialog's button does with the typed name: nothing
// for a blank one, else file into it, and whether that folder exists already.
func folderCommit(typed string, existing []string) (name string, ok, known bool) {
	name = strings.TrimSpace(typed)
	return name, name != "", slices.Contains(existing, name)
}

// layoutFolderDialog is NewFolderDialog. Lay it out before the content, so it
// claims Escape ahead of anything under it; it reports the filing it commits.
func (s *sidebar) layoutFolderDialog(gtx C, th *ui.Theme, existing []string) fileRequest {
	n := &s.naming
	name, ok, known := folderCommit(n.name.Text(), existing)
	submitted := false
	for {
		ev, more := n.name.Update(gtx)
		if !more {
			break
		}
		if _, is := ev.(widget.SubmitEvent); is {
			submitted = true
		}
	}
	var filed fileRequest
	if n.create.Clicked(gtx) || submitted {
		if ok && n.open {
			filed = fileRequest{ids: n.ids, folder: name}
			n.open = false
		}
	}
	if n.cancel.Clicked(gtx) {
		n.open = false
	}
	if n.focus && n.open {
		gtx.Execute(key.FocusCmd{Tag: &n.name})
		n.focus = false
	}
	on := th.On(th.Popover)
	cancel := on.Button(&n.cancel, i18n.T("common.action.cancel"))
	cancel.Variant = ui.VariantGhost
	label := i18n.T("common.action.create")
	if known {
		label = i18n.T("sidebar.newFolderDialog.moveHere")
	}
	create := on.Button(&n.create, label)
	create.Disabled = !ok
	d := th.Dialog(&n.dialog, n.open, i18n.T("sidebar.newFolderDialog.title"), folderDescription(len(n.ids)))
	d.Body = func(gtx C) D { return folderField(gtx, on, &n.name, name, known) }
	d.Actions = []layout.Widget{cancel.Layout, create.Layout}
	if d.Layout(gtx) {
		n.open = false
	}
	return filed
}

func folderDescription(count int) string {
	return i18n.T("sidebar.newFolderDialog.description", i18n.Args{"count": count})
}

// folderField is `flex flex-col gap-1.5`: the label, the field, and the note
// a name the project already holds earns.
func folderField(gtx C, th *ui.Theme, editor *widget.Editor, name string, known bool) D {
	editor.Submit = true
	children := []layout.FlexChild{
		layout.Rigid(th.Label(i18n.T("sidebar.newFolderDialog.name")).Layout),
		ui.VGap(1.5),
		layout.Rigid(th.Input(editor, i18n.T("sidebar.newFolderDialog.placeholder")).Layout),
	}
	if known {
		before, after := aroundValue("sidebar.newFolderDialog.exists", "name")
		rest := th.Text(ui.TextXS, after).Muted()
		rest.MaxLines = 0
		children = append(children, ui.VGap(1.5), layout.Rigid(func(gtx C) D {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(th.Text(ui.TextXS, before).Muted().Layout),
				layout.Rigid(th.Text(ui.TextXS, name).Weight(font.Medium).Layout),
				layout.Flexed(1, rest.Layout),
			)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}
