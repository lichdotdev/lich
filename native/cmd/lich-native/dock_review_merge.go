package main

// A merge in progress in the Review: the strip over the files that names what
// is coming in and ends the merge, and what a conflicted file offers instead
// of a revert.

import (
	"context"
	"image/color"
	"slices"
	"strings"

	"gioui.org/layout"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// mergeStripTint is how much of its tone the strip mixes into the background,
// the tint DESIGN.md gives an added or removed row.
const mergeStripTint = 0.12

type mergeStrip struct {
	commit, abort, keep, confirm ui.Clickable
	commitTip                    ui.Tooltip
	// asking is Abort pressed and waiting on its confirmation.
	asking bool
}

// reverts is whether rel offers a revert: never on a pull request, and never
// mid-merge, where going back to HEAD drops what the merge brought.
func (p *reviewPanel) reverts(rel string) bool {
	return p.pr == nil && p.git.Merging == "" && !slices.Contains(p.git.Conflicted, rel)
}

func (p *reviewPanel) conflicted(rel string) bool {
	return p.pr == nil && slices.Contains(p.git.Conflicted, rel)
}

// conflictsFirst moves the conflicted files to the top, keeping git's order
// within each part.
func conflictsFirst(order, conflicted []string) {
	slices.SortStableFunc(order, func(a, b string) int {
		ca, cb := slices.Contains(conflicted, a), slices.Contains(conflicted, b)
		switch {
		case ca == cb:
			return 0
		case ca:
			return -1
		}
		return 1
	})
}

// hasConflictMarkers is whether a file's diff still adds git's markers.
func hasConflictMarkers(patch string) bool {
	for line := range strings.Lines(patch) {
		for _, marker := range []string{"+<<<<<<< ", "+=======\n", "+>>>>>>> "} {
			if strings.HasPrefix(line, marker) {
				return true
			}
		}
	}
	return false
}

func (p *reviewPanel) layoutMerge(gtx C, th *ui.Theme) D {
	m := &p.merge
	if p.pr != nil || p.git.Merging == "" {
		m.asking = false
		return D{}
	}
	left := len(p.git.Conflicted)
	if m.commit.Clicked(gtx) && left == 0 {
		p.endMerge(true)
	}
	if m.abort.Clicked(gtx) {
		m.asking = true
	}
	if m.keep.Clicked(gtx) {
		m.asking = false
	}
	if m.confirm.Clicked(gtx) {
		m.asking = false
		p.endMerge(false)
	}
	tone := th.ToneWait
	switch {
	case m.asking:
		tone = th.Destructive
	case left == 0:
		tone = th.TonePass
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Background{}.Layout(gtx,
		func(gtx C) D { return ui.Fill(gtx, ui.MixOKLab(th.Background, tone, mergeStripTint), 0) },
		func(gtx C) D {
			return layout.Inset{Left: ui.Space(3), Right: ui.Space(2), Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, p.mergeRow(th, tone, left)...)
			})
		})
}

func (p *reviewPanel) mergeRow(th *ui.Theme, tone color.NRGBA, left int) []layout.FlexChild {
	m := &p.merge
	if m.asking {
		abort := th.Button(&m.confirm, i18n.T("git.merge.abort"))
		abort.Size, abort.Variant = ui.SizeSM, ui.VariantDestructive
		keep := th.Button(&m.keep, i18n.T("git.merge.keep"))
		keep.Size, keep.Variant = ui.SizeSM, ui.VariantGhost
		return []layout.FlexChild{
			layout.Flexed(1, ui.Stack(0.5,
				para(th.Text(ui.TextSM, i18n.T("git.merge.abortAsk")).In(tone)).Layout,
				th.Text(ui.TextXS, "git merge --abort").Muted().Mono().Layout,
			)),
			ui.Gap(2), layout.Rigid(keep.Layout), ui.Gap(2), layout.Rigid(abort.Layout),
		}
	}
	commit := th.Button(&m.commit, i18n.T("git.merge.commit"))
	commit.Size, commit.Disabled = ui.SizeSM, left > 0
	if left > 0 {
		commit.Variant = ui.VariantOutline
	}
	abort := th.Button(&m.abort, i18n.T("git.merge.abort"))
	abort.Size, abort.Variant = ui.SizeSM, ui.VariantGhost
	blocked := ""
	if left > 0 {
		blocked = i18n.T("git.merge.commitBlocked")
	}
	return []layout.FlexChild{
		layout.Flexed(1, para(th.Text(ui.TextSM, mergingText(p.git)).In(tone)).Layout),
		ui.Gap(2),
		layout.Rigid(abort.Layout), ui.Gap(2),
		layout.Rigid(func(gtx C) D { return th.Tooltip(p.root, &m.commitTip, blocked).Layout(gtx, commit.Layout) }),
	}
}

// endMerge commits the merge, or aborts it.
func (p *reviewPanel) endMerge(commit bool) {
	path, git := p.target, p.git
	go func() {
		defer p.written(path)
		if !commit {
			if err := p.client.GitMergeAbort(context.Background(), path); err != nil {
				p.notify(ui.Toast{Variant: ui.ToastError, Title: i18n.T("git.merge.abortFailed"), Description: backend.ErrorText(err)})
			}
			return
		}
		if err := p.client.GitMergeCommit(context.Background(), path); err != nil {
			p.notify(ui.Toast{Variant: ui.ToastError, Title: i18n.T("git.merge.commitFailed"), Description: backend.ErrorText(err)})
			return
		}
		branch := git.Branch
		if branch == "" {
			branch = "HEAD"
		}
		p.notify(ui.Toast{Title: i18n.T("git.merge.merged", i18n.Args{"source": git.Merging, "branch": branch})})
	}()
}

// markResolved stages a conflicted file as it stands.
func (p *reviewPanel) markResolved(rel string) {
	path := p.target
	_, name := splitPath(rel)
	go func() {
		defer p.written(path)
		if err := p.client.GitMarkResolved(context.Background(), path, rel); err != nil {
			p.notify(ui.Toast{Variant: ui.ToastError, Title: i18n.T("git.merge.markFailed", i18n.Args{"name": name}), Description: backend.ErrorText(err)})
		}
	}()
}

// conflictActions is a conflicted file's header: the state where the line
// counts stand, and Mark resolved where the revert stands, held back while
// the markers are still in.
func (c *fileCard) conflictActions(th *ui.Theme, p *reviewPanel) []layout.FlexChild {
	state := th.Text(ui.TextXS, i18n.T("git.merge.inConflict")).In(th.ToneWait)
	label := i18n.T("git.merge.markResolved")
	if hasConflictMarkers(c.gitText) {
		label = i18n.T("git.resolve.open")
	}
	act := th.Button(&c.resolve, label)
	act.Size, act.Variant = ui.SizeXS, ui.VariantOutline
	return []layout.FlexChild{
		ui.Gap(2),
		layout.Rigid(func(gtx C) D { return ui.IconLine(gtx, icConflict, state.Color, layout.Rigid(state.Layout)) }),
		ui.Gap(2),
		layout.Rigid(act.Layout),
	}
}
