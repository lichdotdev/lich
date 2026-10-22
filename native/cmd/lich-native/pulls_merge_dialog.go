package main

import (
	"fmt"
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
	"github.com/lichdotdev/lich/native/ui"
)

// The commit message gh would write itself, which goes into git as is.
const (
	mergeEditSquashSubj = "%s (#%d)"
	mergeEditMergeSubj  = "Merge pull request #%d from %s"
)

// The body field's room: MergeMessageDialog's min-h-[30vh] and max-h-[50vh],
// in dp of a typical window rather than of this one's height.
var (
	mergeBodyMin = unit.Dp(240)
	mergeBodyMax = unit.Dp(420)
)

// mergeEdit is what the dialog opens with for a method: its title, and the
// message gh would otherwise write (mergeEditFor). Rebase replays the
// branch's own commits, so it takes no message and has no edit.
type mergeEdit struct {
	method, title, subject, body string
}

func mergeEditFor(method string, d lichclient.PRDetail) mergeEdit {
	if method == "squash" {
		return mergeEdit{method: method, title: i18n.T("pulls.mergeMessageDialog.squashAndMerge"), subject: fmt.Sprintf(mergeEditSquashSubj, d.Title, d.Number), body: d.Body}
	}
	return mergeEdit{method: method, title: i18n.T("pulls.mergeMessageDialog.createMergeCommit"), subject: fmt.Sprintf(mergeEditMergeSubj, d.Number, d.HeadRefName)}
}

// mergeDialog is MergeMessageDialog: the subject, one line that wraps so its
// PR number stays in sight, and the body, then Merge.
type mergeDialog struct {
	open    bool
	edit    mergeEdit
	admin   bool
	dialog  ui.DialogState
	subject widget.Editor
	body    widget.Editor
	bodyFit ui.TextareaResize
	merge   widget.Clickable
	cancel  widget.Clickable
	focus   bool
}

// show opens the dialog on method's message for d.
func (m *mergeDialog) show(method string, admin bool, d lichclient.PRDetail) {
	m.open, m.admin, m.focus = true, admin, true
	m.edit = mergeEditFor(method, d)
	m.subject.SingleLine, m.subject.Submit = false, true
	m.subject.SetText(m.edit.subject)
	m.body.SetText(m.edit.body)
}

// editEntries are the Merge menu's "edit message…" items, below a separator,
// for the allowed methods that take a message.
func (v *pullView) editEntries(methods []string, bypass bool, d lichclient.PRDetail) []ui.MenuEntry {
	var out []ui.MenuEntry
	for _, m := range methods {
		if m == "rebase" {
			continue
		}
		label := i18n.T("pulls.pullRequestView.editMessage", i18n.Args{"method": mergeMethodLabel(m)})
		if bypass {
			label = i18n.T("pulls.pullRequestView.editMessageBypass", i18n.Args{"method": mergeMethodLabel(m)})
		}
		out = append(out, ui.MenuEntry{Label: label, OnClick: func() { v.mergeEdit.show(m, bypass, d) }})
	}
	if len(out) == 0 {
		return nil
	}
	return append([]ui.MenuEntry{{Kind: ui.MenuSeparator}}, out...)
}

// layoutMergeDialog draws the dialog at the window's level and merges on its
// Merge, with what was typed.
func (u *window) layoutMergeDialog(gtx C, th *ui.Theme) {
	v := &u.pulls.view
	m := &v.mergeEdit
	p := u.pulls
	if m.cancel.Clicked(gtx) {
		m.open = false
	}
	var d *lichclient.PRDetail
	if m.open && p.open && p.selected != 0 {
		d = p.store.detail(p.project.Path, p.selected).val
	}
	open := m.open && d != nil
	subject := strings.TrimSpace(strings.ReplaceAll(m.subject.Text(), "\n", " "))
	busy := v.busyNow() == "merge"
	submit := m.merge.Clicked(gtx) || editorSubmitted(gtx, &m.subject)
	if submit && open && !busy && subject != "" {
		project, detail, method, admin, body := p.project, *d, m.edit.method, m.admin, m.body.Text()
		go func() {
			if u.mergePR(project, detail, method, admin, subject, body) {
				v.onFrame(func() { m.open = false })
			}
		}()
	}
	on := th.On(th.Popover)
	cancel := on.Button(&m.cancel, i18n.T("common.action.cancel"))
	cancel.Variant = ui.VariantGhost
	label := m.edit.title
	if busy {
		label = i18n.T("pulls.mergeMessageDialog.merging")
	}
	merge := on.Button(&m.merge, label)
	merge.Icon, merge.Disabled = icPullMerged, busy || subject == ""
	dlg := th.Dialog(&m.dialog, open, m.edit.title, i18n.T("pulls.mergeMessageDialog.description"))
	dlg.MaxWidth = ui.DialogWidthXL
	dlg.Body = func(gtx C) D { return m.fields(gtx, on) }
	dlg.Actions = []layout.Widget{cancel.Layout, merge.Layout}
	dlg.Root = &u.root
	if dlg.Layout(gtx) {
		m.open = false
	}
}

func (m *mergeDialog) fields(gtx C, th *ui.Theme) D {
	if m.focus {
		gtx.Execute(key.FocusCmd{Tag: &m.subject})
		m.focus = false
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(th.Label(i18n.T("pulls.mergeMessageDialog.subject")).Layout),
		ui.VGap(1.5),
		layout.Rigid(th.Textarea(&m.subject, "").Layout),
		ui.VGap(3),
		layout.Rigid(th.Label(i18n.T("pulls.mergeMessageDialog.body")).Layout),
		ui.VGap(1.5),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Min.Y = gtx.Dp(mergeBodyMin)
			gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(mergeBodyMax))
			ta := th.Textarea(&m.body, i18n.T("pulls.mergeMessageDialog.optional"))
			ta.Resize = &m.bodyFit
			return ta.Layout(gtx)
		}),
	)
}
