package main

// The branch tree's actions: the menu of each ref row, the in-row fields that
// rename a branch or start one, and the two confirmations, a checkout under a
// session's agent and a delete that leaves commits without a branch.

import (
	"context"
	"image"
	"sync"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

type gitBranchAct uint8

const (
	gitActCheckout gitBranchAct = iota
	gitActCheckoutRemote
	gitActOpenSession
	gitActNewBranch
	gitActRename
	gitActDelete
)

// gitBlocked is why a menu entry is there but cannot be chosen.
type gitBlocked uint8

const (
	gitFree gitBlocked = iota
	gitOnBranch
	gitInWorktree
)

type gitMenuItem struct {
	act     gitBranchAct
	blocked gitBlocked
}

// gitBranchMenu is what a ref row offers. A branch checked out in another
// worktree cannot be checked out here, so its first entry opens the session on
// it when there is one; a remote branch is checked out as the local one that
// tracks it.
func gitBranchMenu(row gitTreeRow, hasSession bool) []gitMenuItem {
	switch {
	case row.remote:
		return []gitMenuItem{{act: gitActCheckoutRemote}, {act: gitActNewBranch}}
	case row.tag:
		return []gitMenuItem{{act: gitActNewBranch}}
	}
	held := row.worktree != "" && !row.head
	var items []gitMenuItem
	switch {
	case held && hasSession:
		items = append(items, gitMenuItem{act: gitActOpenSession})
	case !held && !row.head:
		items = append(items, gitMenuItem{act: gitActCheckout})
	}
	items = append(items, gitMenuItem{act: gitActNewBranch}, gitMenuItem{act: gitActRename})
	del := gitMenuItem{act: gitActDelete}
	switch {
	case row.head:
		del.blocked = gitOnBranch
	case held:
		del.blocked = gitInWorktree
	}
	return append(items, del)
}

// gitEdit is the in-row field renaming from, or starting a branch at it.
type gitEdit struct {
	from   gitTreeRow
	create bool
	field  ui.InlineEdit
}

// gitConfirm is a checkout or a delete waiting on the dialog.
type gitConfirm struct {
	remove  bool
	row     gitTreeRow
	session string
	only    int
}

type branchOps struct {
	notify      func(ui.Toast)
	invalidate  func()
	openSession func(projectID, id string)
	editing     *gitEdit
	// menuOpen is whether a row's menu was open in the last frame, which
	// keeps the terminal off its keys.
	menuOpen bool

	mu sync.Mutex
	// asked is a delete whose count came back from the backend, for the next
	// frame to put up.
	asked *gitConfirm

	confirm      *gitConfirm
	confirmShown gitConfirm
	dialog       ui.DialogState
	ok, cancel   widget.Clickable
}

func gitActLabel(act gitBranchAct, session string) string {
	switch act {
	case gitActCheckout:
		return i18n.T("git.branch.checkout")
	case gitActCheckoutRemote:
		return i18n.T("git.branch.checkoutRemote")
	case gitActOpenSession:
		return i18n.T("git.branch.open", i18n.Args{"name": session})
	case gitActNewBranch:
		return i18n.T("git.branch.newBranch")
	case gitActRename:
		return i18n.T("git.branch.rename")
	}
	return i18n.T("git.branch.delete")
}

func gitBlockedText(b gitBlocked) string {
	if b == gitOnBranch {
		return i18n.T("git.branch.onBranch")
	}
	return i18n.T("git.branch.inWorktree")
}

func (g *gitPanel) branchEntries(row gitTreeRow, in gitPanelInput) []ui.MenuEntry {
	sv, hasSession := in.sessions[row.branch]
	hasSession = hasSession && row.branch != ""
	var out []ui.MenuEntry
	for _, it := range gitBranchMenu(row, hasSession) {
		if it.act == gitActDelete && len(out) > 0 {
			out = append(out, ui.MenuEntry{Kind: ui.MenuSeparator})
		}
		e := ui.MenuEntry{Label: gitActLabel(it.act, sv.Label), OnClick: func() { g.doBranch(it.act, row, sv, in) }}
		if it.act == gitActDelete {
			e.Variant = ui.VariantDestructive
		}
		if it.blocked != gitFree {
			e.Disabled, e.Shortcut = true, gitBlockedText(it.blocked)
		}
		out = append(out, e)
	}
	return out
}

func (g *gitPanel) doBranch(act gitBranchAct, row gitTreeRow, sv sessionView, in gitPanelInput) {
	switch act {
	case gitActCheckout, gitActCheckoutRemote:
		if in.here != nil {
			g.confirm = &gitConfirm{row: row, session: in.here.Label}
			return
		}
		g.checkout(row)
	case gitActOpenSession:
		g.openSession(sv.projectID, sv.ID)
	case gitActNewBranch, gitActRename:
		e := &gitEdit{from: row, create: act == gitActNewBranch}
		if !e.create {
			e.field.Start(row.name)
		} else {
			e.field.Start("")
		}
		g.editing = e
	case gitActDelete:
		g.askDelete(row)
	}
}

// runBranchOp runs a write off the UI goroutine, toasts git's answer when it
// fails and reads the refs again when it does not.
func (g *gitPanel) runBranchOp(fail string, op func(context.Context, *backend.Client, string) error, then func()) {
	path := g.path
	go func() {
		if err := op(g.store.ctx, g.store.client, path); err != nil {
			g.notify(ui.Toast{Variant: ui.ToastError, Title: fail, Description: backend.ErrorText(err)})
			return
		}
		g.store.refresh()
		if then != nil {
			then()
		}
	}()
}

func (g *gitPanel) checkout(row gitTreeRow) {
	g.runBranchOp(i18n.T("git.branch.checkoutFailed", i18n.Args{"name": row.name}), func(ctx context.Context, c *backend.Client, path string) error {
		return c.GitCheckout(ctx, path, row.name, row.remote)
	}, nil)
}

// askDelete counts what only the branch holds: nothing, and it goes at once
// with an Undo; something, and the dialog asks first.
func (g *gitPanel) askDelete(row gitTreeRow) {
	path, fail := g.path, i18n.T("git.branch.deleteFailed", i18n.Args{"name": row.name})
	go func() {
		only, err := g.store.client.GitBranchOnly(g.store.ctx, path, row.name)
		if err != nil {
			g.notify(ui.Toast{Variant: ui.ToastError, Title: fail, Description: backend.ErrorText(err)})
			return
		}
		if only == 0 {
			g.deleteBranch(path, row.name, 0)
			return
		}
		g.mu.Lock()
		g.asked = &gitConfirm{remove: true, row: row, only: only}
		g.mu.Unlock()
		g.invalidate()
	}()
}

// deleteBranch deletes, off the UI goroutine, and offers to make the branch
// again at the commit it pointed at.
func (g *gitPanel) deleteBranch(path, name string, only int) {
	hash, err := g.store.client.GitDeleteBranch(g.store.ctx, path, name)
	if err != nil {
		g.notify(ui.Toast{Variant: ui.ToastError, Title: i18n.T("git.branch.deleteFailed", i18n.Args{"name": name}), Description: backend.ErrorText(err)})
		return
	}
	g.store.refresh()
	toast := ui.Toast{Title: i18n.T("git.branch.deleted", i18n.Args{"name": name}), Duration: revertUndoFor,
		Action: &ui.ToastButton{Label: i18n.T("git.branch.undo"), OnClick: func() {
			g.runBranchOp(i18n.T("git.branch.undoFailed", i18n.Args{"name": name}), func(ctx context.Context, c *backend.Client, _ string) error {
				return c.GitCreateBranch(ctx, path, name, hash)
			}, nil)
		}}}
	if only > 0 {
		toast.Description = i18n.T("git.branch.onlyCommits", i18n.Args{"count": only})
	}
	g.notify(toast)
}

// renameField takes the label's place while the branch is renamed.
func (g *gitPanel) renameField(gtx C, th *ui.Theme) D {
	e := g.editing
	dims, res := th.InlineEdit(&e.field, ui.TextXS).Layout(gtx)
	switch res.Outcome {
	case ui.InlineEditCommitted:
		g.editing = nil
		name, newName := e.from.name, res.Value
		g.runBranchOp(i18n.T("git.branch.renameFailed", i18n.Args{"name": name}), func(ctx context.Context, c *backend.Client, path string) error {
			return c.GitRenameBranch(ctx, path, name, newName)
		}, nil)
	case ui.InlineEditCancelled:
		g.editing = nil
	}
	return dims
}

// newBranchRow is the field under a ref naming a branch to start at it.
func (g *gitPanel) newBranchRow(gtx C, th *ui.Theme) D {
	e := g.editing
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(gitRowHeight))
	gtx.Constraints = layout.Exact(size)
	return layout.Inset{Left: ui.Space(5.5), Right: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return icBranch.Layout(gtx, ui.Space(3), th.MutedForeground) }),
			ui.Gap(1.5),
			layout.Flexed(1, func(gtx C) D {
				dims, res := th.InlineEdit(&e.field, ui.TextXS).Layout(gtx)
				switch res.Outcome {
				case ui.InlineEditCommitted:
					g.editing = nil
					name, from := res.Value, e.from.ref
					g.runBranchOp(i18n.T("git.branch.createFailed", i18n.Args{"name": name}), func(ctx context.Context, c *backend.Client, path string) error {
						return c.GitCreateBranch(ctx, path, name, from)
					}, nil)
				case ui.InlineEditCancelled:
					g.editing = nil
				}
				return dims
			}),
			ui.Gap(1.5),
			layout.Rigid(th.Text(ui.TextXS, i18n.T("git.branch.from", i18n.Args{"name": e.from.name})).Muted().Layout),
		)
	})
}

// layoutDialogs is the checkout or delete confirmation, laid out by the window
// before its content so it takes Escape first.
func (g *gitPanel) layoutDialogs(gtx C, th *ui.Theme) {
	g.mu.Lock()
	if g.asked != nil {
		g.confirm, g.asked = g.asked, nil
	}
	g.mu.Unlock()
	if g.confirm == nil && !g.dialog.Animating(gtx) {
		return
	}
	if g.confirm != nil {
		g.confirmShown = *g.confirm
	}
	c := g.confirmShown
	switch {
	case g.ok.Clicked(gtx):
		g.confirm = nil
		g.carryOut(c)
	case g.cancel.Clicked(gtx):
		g.confirm = nil
	}
	title := i18n.T("git.branch.checkoutTitle", i18n.Args{"session": c.session, "branch": c.row.name})
	body := i18n.T("git.branch.checkoutBody", i18n.Args{"session": c.session})
	label, variant := i18n.T("git.branch.checkout"), ui.VariantDefault
	if c.remove {
		title = i18n.T("git.branch.deleteTitle", i18n.Args{"name": c.row.name})
		body = i18n.T("git.branch.deleteBody", i18n.Args{"count": c.only})
		label, variant = i18n.T("git.branch.delete"), ui.VariantDestructive
	}
	pop := th.On(th.Popover)
	d := th.Dialog(&g.dialog, g.confirm != nil, title, body)
	ok := pop.Button(&g.ok, label)
	ok.Variant = variant
	back := pop.Button(&g.cancel, i18n.T("common.action.cancel"))
	back.Variant = ui.VariantOutline
	d.Actions = []layout.Widget{back.Layout, ok.Layout}
	if d.Layout(gtx) {
		g.confirm = nil
	}
}

func (g *gitPanel) carryOut(c gitConfirm) {
	if !c.remove {
		g.checkout(c.row)
		return
	}
	path := g.path
	go g.deleteBranch(path, c.row.name, c.only)
}
