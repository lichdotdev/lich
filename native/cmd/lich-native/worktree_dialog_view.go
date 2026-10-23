package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icWtBranch = icons.Lucide("git-branch")
	// wtListMax is how tall the branch list grows before it scrolls.
	wtListMax = ui.Space(64)
)

// wtRow is one line of the branch list: a group heading, or a base to pick.
type wtRow struct {
	heading string
	value   string
	label   string
	note    string
}

// branchRows is the list the picker shows for query: worktrees to reopen,
// then local and remote branches, each group headed and counted.
func branchRows(b *backend.Branches, query string) []wtRow {
	if b == nil {
		return nil
	}
	needle := strings.ToLower(strings.TrimSpace(query))
	match := func(name string) bool { return strings.Contains(strings.ToLower(name), needle) }
	var rows []wtRow
	group := func(title string, opts []wtRow) {
		if len(opts) > 0 {
			rows = append(rows, wtRow{heading: fmt.Sprintf("%s (%d)", strings.ToUpper(title), len(opts))})
			rows = append(rows, opts...)
		}
	}
	var trees, local, remote []wtRow
	for _, w := range b.Worktrees {
		if match(w.Name) {
			trees = append(trees, wtRow{value: baseValue(baseWorktree, w.Path), label: w.Name, note: i18n.T("sidebar.worktreeDialogView.opensSession")})
		}
	}
	for _, name := range b.Local {
		if match(name) {
			local = append(local, wtRow{value: baseValue(baseLocal, name), label: name})
		}
	}
	for _, name := range b.Remote {
		if match(name) {
			remote = append(remote, wtRow{value: baseValue(baseRemote, name), label: name})
		}
	}
	group(i18n.T("sidebar.worktreeDialog.worktrees"), trees)
	group(i18n.T("sidebar.worktreeDialogView.local"), local)
	group(i18n.T("sidebar.worktreeDialogView.remote"), remote)
	return rows
}

// stepBase is the base the arrows land on from current, delta rows away
// among the pickable ones.
func stepBase(rows []wtRow, current string, delta int) string {
	var values []string
	for _, r := range rows {
		if r.heading == "" {
			values = append(values, r.value)
		}
	}
	if len(values) == 0 {
		return current
	}
	at := 0
	for i, v := range values {
		if v == current {
			at = i + delta
		}
	}
	return values[max(0, min(at, len(values)-1))]
}

// baseLabel is what the trigger names: the branch, or the worktree's name.
func baseLabel(base string) string {
	group, id := splitBase(base)
	if group == baseWorktree {
		return filepath.Base(id)
	}
	return id
}

// layoutWorktreeDialog draws the dialog and carries out its keys and clicks.
// Lay it out before the content, so it claims Escape ahead of anything under
// it.
func (u *window) layoutWorktreeDialog(gtx C, th *ui.Theme) {
	d := u.worktree
	if !d.open && !d.dialog.Animating(gtx) {
		return
	}
	if d.view().done {
		d.open = false
	}
	if checkout, ok := u.model.gitStatus(d.source); ok {
		d.checkout = checkout
	}
	d.applyRung()
	d.input(gtx, u)
	v := d.view()
	on := th.On(th.Popover)
	cancel := on.Button(&d.cancel, i18n.T("common.action.cancel"))
	cancel.Variant = ui.VariantGhost
	title, description := i18n.T("sidebar.worktreeDialogView.title"), i18n.T("sidebar.worktreeDialogView.description")
	label := i18n.T("sidebar.worktreeDialogView.create")
	if d.fork != nil {
		title = i18n.T("sidebar.worktreeDialogView.forkTitle", i18n.Args{"label": d.fork.label})
		description = i18n.T("sidebar.worktreeDialogView.forkDescription")
		label = i18n.T("sidebar.worktreeDialogView.fork")
	}
	switch group, _ := splitBase(v.base); {
	case v.submitting:
		label = i18n.T("sidebar.worktreeDialog.creating")
	case group == baseWorktree:
		label = i18n.T("sidebar.worktreeDialogView.open")
	}
	create := on.Button(&d.create, label)
	create.Disabled = v.base == "" || v.submitting
	dlg := th.Dialog(&d.dialog, d.open, title, description)
	dlg.Root = d.root
	dlg.Body = func(gtx C) D { return d.body(gtx, on, u.ctx, v) }
	dlg.Actions = []layout.Widget{cancel.Layout, create.Layout}
	// An Escape that closes the branch list leaves the dialog open.
	listOpen := d.pickerOpen
	if dlg.Layout(gtx) && !v.submitting && !listOpen {
		d.open = false
	}
}

// input reads the frame's typing, keys and clicks.
func (d *worktreeDialog) input(gtx C, u *window) {
	d.name.SingleLine, d.name.Submit, d.search.SingleLine = true, true, true
	submitted := editorSubmitted(gtx, &d.name)
	d.lookUpIssue(u.ctx, d.name.Text())
	// Asked until it lands: the dialog takes the focus for its panel on the
	// frame it opens, after this.
	if d.focus && d.open {
		if gtx.Focused(&d.name) {
			d.focus = false
		} else {
			gtx.Execute(key.FocusCmd{Tag: &d.name})
		}
	}
	if d.picker.Opened() != d.pickerOpen {
		d.pickerOpen = d.picker.Opened()
		if d.pickerOpen {
			d.search.SetText("")
		}
	}
	// The open panel claims the focus for itself; the search is the one
	// thing in it that takes keys.
	if d.pickerOpen && !gtx.Focused(&d.search) {
		gtx.Execute(key.FocusCmd{Tag: &d.search})
	}
	d.searchKeys(gtx, u.ctx)
	// The base's line reads the box, and is laid out above it.
	d.carry.Update(gtx)
	if d.cancel.Clicked(gtx) {
		d.open = false
	}
	v := d.view()
	if (d.create.Clicked(gtx) || submitted) && v.base != "" && !v.submitting && d.open {
		r := d.request(v)
		d.setSubmitting("")
		go u.openWorktree(r)
	}
}

// searchKeys walks the list with the arrows while the picker is open; Enter
// keeps the row under the cursor and Escape the base it had. The search holds
// the focus, so the panel's own Escape never fires.
func (d *worktreeDialog) searchKeys(gtx C, ctx context.Context) {
	names := []key.Name{key.NameUpArrow, key.NameDownArrow, key.NameReturn, key.NameEnter, key.NameEscape}
	filters := make([]event.Filter, len(names))
	for i, name := range names {
		filters[i] = key.Filter{Focus: &d.search, Name: name}
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		e, isKey := ev.(key.Event)
		if !isKey || e.State != key.Press {
			continue
		}
		v := d.view()
		rows := branchRows(v.branches, d.search.Text())
		switch e.Name {
		case key.NameDownArrow:
			d.pick(ctx, stepBase(rows, v.base, 1))
		case key.NameUpArrow:
			d.pick(ctx, stepBase(rows, v.base, -1))
		default:
			d.picker.Close()
		}
	}
}

func (d *worktreeDialog) body(gtx C, th *ui.Theme, ctx context.Context, v worktreeView) D {
	group, branch := splitBase(v.base)
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return d.nameField(gtx, th, v, group == baseWorktree) }),
		ui.VGap(4),
		layout.Rigid(th.Label(i18n.T("sidebar.worktreeDialogView.from")).Layout),
		ui.VGap(1.5),
		layout.Rigid(func(gtx C) D { return d.basePicker(gtx, th, ctx, v) }),
	}
	if text, warn := freshnessLine(branch, v.check, d.carry.Value && d.canCarry(v.base)); text != "" && group == baseLocal {
		line := th.Text(ui.TextXS, text).Muted()
		if warn {
			line = th.Text(ui.TextXS, text).In(th.ToneWait)
		}
		children = append(children, ui.VGap(1.5), layout.Rigid(line.Layout))
	}
	if d.canCarry(v.base) {
		children = append(children, ui.VGap(3), layout.Rigid(func(gtx C) D { return d.carryRow(gtx, th) }))
	}
	// A fork opens like a session another one asked for, on its provider's
	// rung, so it has no box to tick.
	if v.canConfine && group != baseWorktree && d.fork == nil {
		children = append(children, ui.VGap(3), layout.Rigid(func(gtx C) D {
			return checkRow(gtx, th, &d.confined, i18n.T("sidebar.worktreeSandboxRow.runConfined"), "")
		}))
	}
	if failure := v.loadErr + v.submitErr; failure != "" {
		children = append(children, ui.VGap(3), layout.Rigid(func(gtx C) D { return wrapped(gtx, th.Text(ui.TextXS, failure).In(th.Destructive)) }))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// nameField is the name, and under it what it creates: the issue it names
// and the branch, or that a worktree is being reopened instead.
func (d *worktreeDialog) nameField(gtx C, th *ui.Theme, v worktreeView, reopening bool) D {
	field := th.Input(&d.name, i18n.T("sidebar.worktreeDialogView.namePlaceholder"))
	field.Disabled = reopening
	children := []layout.FlexChild{
		layout.Rigid(th.Label(i18n.T("sidebar.worktreeDialogView.name")).Layout),
		ui.VGap(1.5),
		layout.Rigid(field.Layout),
		ui.VGap(1.5),
	}
	hint := func(text string) layout.FlexChild {
		return layout.Rigid(th.Text(ui.TextXS, text).Muted().Mono().Layout)
	}
	switch {
	case reopening:
		children = append(children, hint(i18n.T("sidebar.worktreeDialogView.opensSelected")))
	case v.issueRef != 0 && v.issue == nil && v.issueErr == "":
		children = append(children, hint(i18n.T("sidebar.worktreeDialog.lookingUpIssue", i18n.Args{"number": v.issueRef})))
	default:
		if v.issue != nil {
			children = append(children, layout.Rigid(th.Text(ui.TextXS, fmt.Sprintf("#%d %s", v.issue.Number, v.issue.Title)).Layout))
		}
		if v.issueErr != "" {
			children = append(children, layout.Rigid(func(gtx C) D { return wrapped(gtx, th.Text(ui.TextXS, v.issueErr).In(th.Destructive)) }))
		}
		branch := v.newBranch(d.name.Text())
		if branch == "" {
			branch = i18n.T("sidebar.worktreeDialogView.randomBranch")
		}
		children = append(children, hint(i18n.T("sidebar.worktreeDialogView.branchLine", i18n.Args{"branch": branch})))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// basePicker is the From select: the trigger naming the base, and the list
// with its search.
func (d *worktreeDialog) basePicker(gtx C, th *ui.Theme, ctx context.Context, v worktreeView) D {
	width := unit.Dp(float32(gtx.Constraints.Max.X) / gtx.Metric.PxPerDp)
	label := baseLabel(v.base)
	if v.branches == nil && v.loadErr == "" {
		label = i18n.T("sidebar.worktreeDialog.loadingBranches")
	}
	pop := th.DropdownPanel(d.root, &d.picker, width, func(gtx C, th *ui.Theme) D {
		return d.branchList(gtx, th, ctx, v)
	})
	lead := func(gtx C) D { return icWtBranch.Layout(gtx, ui.Space(4), th.MutedForeground) }
	return pop.Layout(gtx, func(gtx C) D { return selectTriggerWith(gtx, th, lead, label, width) })
}

func (d *worktreeDialog) branchList(gtx C, th *ui.Theme, ctx context.Context, v worktreeView) D {
	search := th.Input(&d.search, i18n.T("sidebar.worktreeDialogView.searchBranches"))
	search.Leading, search.Height, search.TextSize, search.Mono = icSearch, ui.Space(8), ui.TextXS, true
	rows := branchRows(v.branches, d.search.Text())
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return layout.UniformInset(ui.Space(1)).Layout(gtx, search.Layout) }),
	}
	if len(rows) == 0 {
		children = append(children, layout.Rigid(func(gtx C) D {
			return layout.UniformInset(ui.Space(3)).Layout(gtx, th.Text(ui.TextXS, i18n.T("sidebar.worktreeDialogView.noMatch")).Muted().Layout)
		}))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	d.list.Axis = layout.Vertical
	children = append(children, layout.Rigid(func(gtx C) D {
		gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(wtListMax))
		return th.ScrollArea(&d.list).Layout(gtx, len(rows), func(gtx C, i int) D {
			return d.branchRow(gtx, th, ctx, rows[i], rows[i].value == v.base)
		})
	}))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (d *worktreeDialog) branchRow(gtx C, th *ui.Theme, ctx context.Context, r wtRow, chosen bool) D {
	if r.heading != "" {
		return layout.Inset{Top: ui.Space(2), Bottom: ui.Space(1), Left: ui.Space(2)}.Layout(gtx,
			th.Text(ui.TextXS, r.heading).Muted().Weight(font.SemiBold).Layout)
	}
	if d.rows == nil {
		d.rows = map[string]*widget.Clickable{}
	}
	click := clickableOf(d.rows, r.value)
	if click.Clicked(gtx) {
		d.pick(ctx, r.value)
		d.picker.Close()
	}
	return click.Layout(gtx, func(gtx C) D {
		bg := th.Surface
		if chosen || click.Hovered() {
			bg = th.Accent
		}
		on := th.On(bg)
		macro := layout.Inset{Top: ui.Space(1.5), Bottom: ui.Space(1.5), Left: ui.Space(2), Right: ui.Space(2)}
		return layout.Background{}.Layout(gtx,
			func(gtx C) D {
				if bg == th.Surface {
					return D{Size: gtx.Constraints.Min}
				}
				return ui.Fill(gtx, bg, ui.RadiusSM)
			},
			func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return macro.Layout(gtx, func(gtx C) D {
					lines := []layout.FlexChild{layout.Rigid(on.Text(ui.TextXS, r.label).Mono().Layout)}
					if r.note != "" {
						lines = append(lines, layout.Rigid(on.Text(ui.TextXS, r.note).Muted().Layout))
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, lines...)
				})
			})
	})
}

func (d *worktreeDialog) carryRow(gtx C, th *ui.Theme) D {
	note := i18n.T("sidebar.worktreeDialogView.carryFiles", i18n.Args{"count": d.checkout.Files})
	return checkRow(gtx, th, &d.carry, i18n.T("sidebar.worktreeDialogView.carry"), note)
}

// checkRow is a checkbox with its label, and a muted line under the label.
func checkRow(gtx C, th *ui.Theme, box *widget.Bool, label, note string) D {
	text := []layout.FlexChild{layout.Rigid(th.Text(ui.TextSM, label).Layout)}
	if note != "" {
		text = append(text, layout.Rigid(th.Text(ui.TextXS, note).Muted().Layout))
	}
	return layout.Flex{Alignment: layout.Start}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Top: ui.Space(0.5)}.Layout(gtx, th.Checkbox(box).Layout)
		}),
		ui.Gap(2.5),
		layout.Flexed(1, func(gtx C) D { return layout.Flex{Axis: layout.Vertical}.Layout(gtx, text...) }),
	)
}
