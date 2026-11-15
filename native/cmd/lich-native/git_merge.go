package main

// Merging a branch from its row's menu: the entry, with what the merge would
// bring read ahead of the click, and the merge itself with its Undo.

import (
	"context"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// mergePreview is one row's GitMergePreview as far as it has come.
type mergePreview struct {
	done   bool
	failed bool
	api.GitMergePreview
}

// gitMergeItem is the Merge entry a row offers into the checkout cur, false
// for a row that has none: a tag, and the branch HEAD is on.
func gitMergeItem(row gitTreeRow, cur backend.DiffStats, p mergePreview) (gitMenuItem, bool) {
	if row.tag || row.head {
		return gitMenuItem{}, false
	}
	it := gitMenuItem{act: gitActMerge}
	switch {
	case cur.Merging != "":
		it.blocked = gitMergeRunning
	case cur.Files > 0:
		it.blocked = gitDirtyHere
	case p.done && !p.failed && p.Commits == 0:
		it.blocked = gitMerged
	}
	return it, true
}

// mergeEntry dresses the item: the branch it merges into, why it cannot run,
// or what the preview found.
func (g *gitPanel) mergeEntry(row gitTreeRow, in gitPanelInput) (ui.MenuEntry, bool) {
	p := g.preview(row, in.cur)
	it, ok := gitMergeItem(row, in.cur, p)
	if !ok {
		return ui.MenuEntry{}, false
	}
	e := ui.MenuEntry{Label: i18n.T("git.merge.into", i18n.Args{"branch": branchOrHead(in.cur.Branch)}),
		OnClick: func() { g.merge(row, in) }}
	switch {
	case it.blocked != gitFree:
		e.Disabled, e.Shortcut = true, gitBlockedText(it.blocked)
	case !p.done || p.failed:
	case len(p.Conflicts) > 0:
		e.Shortcut, e.ShortcutDestructive = i18n.T("git.merge.previewConflicts", i18n.Args{"count": len(p.Conflicts)}), true
	default:
		e.Shortcut = i18n.T("git.merge.previewClean")
	}
	return e, true
}

func branchOrHead(branch string) string {
	if branch == "" {
		return "HEAD"
	}
	return branch
}

// preview answers what merging row would bring, asking the backend the first
// time a menu shows it for this HEAD.
func (g *gitPanel) preview(row gitTreeRow, cur backend.DiffStats) mergePreview {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.previewHead != cur.Head || g.previews == nil {
		g.previewHead, g.previews = cur.Head, map[string]mergePreview{}
	}
	p, asked := g.previews[row.ref]
	if asked || row.tag || row.head {
		return p
	}
	g.previews[row.ref] = p
	path, head := g.path, cur.Head
	go func() {
		got, err := g.store.client.GitMergePreview(g.store.ctx, path, row.ref)
		g.mu.Lock()
		if g.previewHead == head {
			g.previews[row.ref] = mergePreview{done: true, failed: err != nil, GitMergePreview: got}
		}
		g.mu.Unlock()
		g.invalidate()
	}()
	return p
}

// merge runs it off the UI goroutine. A clean merge toasts the command with an
// Undo; one that stops says so and points at the Review, where it ends.
func (g *gitPanel) merge(row gitTreeRow, in gitPanelInput) {
	path, into := g.path, branchOrHead(in.cur.Branch)
	behind := 0
	if st, ok := in.status(row.worktree); ok && row.worktree != "" {
		behind = st.Files
	}
	go func() {
		res, err := g.store.client.GitMerge(g.store.ctx, path, row.ref)
		g.store.refresh()
		if g.wrote != nil {
			g.wrote(path)
		}
		if err != nil {
			g.notify(ui.Toast{Variant: ui.ToastError, Title: i18n.T("git.merge.failed", i18n.Args{"name": row.name}), Description: backend.ErrorText(err)})
			return
		}
		if n := len(res.Conflicts); n > 0 {
			g.notify(ui.Toast{Variant: ui.ToastWarning, Title: i18n.T("git.merge.stopped", i18n.Args{"count": n}),
				Action: &ui.ToastButton{Label: i18n.T("dock.rightDock.tabReview"), OnClick: g.showReview}})
			return
		}
		description := "git merge " + row.name
		if behind > 0 {
			description += "\n" + i18n.T("git.merge.leftBehind", i18n.Args{"count": behind})
		}
		g.notify(ui.Toast{Title: i18n.T("git.merge.merged", i18n.Args{"source": row.name, "branch": into}), Description: description,
			Duration: revertUndoFor, Action: &ui.ToastButton{Label: i18n.T("git.branch.undo"), OnClick: func() {
				g.runBranchOp(i18n.T("git.merge.undoFailed"), func(ctx context.Context, c *backend.Client, path string) error {
					return c.GitMergeUndo(ctx, path, res.Before, res.After)
				}, func() {
					if g.wrote != nil {
						g.wrote(path)
					}
				})
			}}})
	}()
}
