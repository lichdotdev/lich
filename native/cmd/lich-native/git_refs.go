package main

import (
	"cmp"
	"image"
	"slices"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var icGitTag = icons.Lucide("tag")

// gitTreeRow is one line of the branch tree: a group's label, or a ref. ref is
// the full name the log is narrowed by, so a tag and a branch of one name are
// two rows; branch is the local branch a session could be on.
// name is git's short name for the ref, origin/x for a remote branch, and
// worktree where a local branch is checked out.
type gitTreeRow struct {
	label, ref, branch, name, worktree string
	group, tag, head, remote           bool
}

// gitTreeRows lays the refs out under Local, each remote and Tags, keeping the
// ones whose name holds filter. A group nothing matched is left out.
func gitTreeRows(refs []backend.GitRef, filter string) []gitTreeRow {
	filter = strings.ToLower(filter)
	keep := slices.DeleteFunc(slices.Clone(refs), func(r backend.GitRef) bool {
		return !strings.Contains(strings.ToLower(r.Name), filter)
	})
	var local, tags []gitTreeRow
	remotes := map[string][]gitTreeRow{}
	for _, r := range keep {
		switch r.Kind {
		case backend.GitRefLocal:
			local = append(local, gitTreeRow{label: r.Name, ref: "refs/heads/" + r.Name, branch: r.Name, name: r.Name, head: r.Head, worktree: r.Worktree})
		case backend.GitRefTag:
			tags = append(tags, gitTreeRow{label: r.Name, ref: "refs/tags/" + r.Name, name: r.Name, tag: true})
		default:
			remote, name, _ := strings.Cut(r.Name, "/")
			remotes[remote] = append(remotes[remote], gitTreeRow{label: name, ref: "refs/remotes/" + r.Name, name: r.Name, remote: true})
		}
	}
	var rows []gitTreeRow
	add := func(group string, items []gitTreeRow) {
		if len(items) > 0 {
			rows = append(append(rows, gitTreeRow{label: group, group: true}), items...)
		}
	}
	add(i18n.T("git.panel.local"), local)
	names := make([]string, 0, len(remotes))
	for name := range remotes {
		names = append(names, name)
	}
	slices.SortFunc(names, cmp.Compare[string])
	for _, name := range names {
		add(name, remotes[name])
	}
	add(i18n.T("git.panel.tags"), tags)
	return rows
}

func (g *gitPanel) treeColumn(gtx C, th *ui.Theme, view gitLogView, in gitPanelInput) D {
	g.menuOpen = false
	rows := gitTreeRows(view.refs.Refs, strings.TrimSpace(g.treeField.Text()))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(gitFilterHeight)))
			return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
				return layout.W.Layout(gtx, func(gtx C) D {
					field := th.Input(&g.treeField, i18n.T("git.panel.branchPlaceholder"))
					field.Height, field.TextSize = gitFieldHeight, ui.TextXS
					return field.Layout(gtx)
				})
			})
		}),
		layout.Flexed(1, func(gtx C) D {
			list := gitScroll(th, &g.tree)
			return layout.Inset{Left: ui.Space(1), Right: ui.Space(1)}.Layout(gtx, func(gtx C) D {
				return list.Layout(gtx, len(rows)+1, func(gtx C, i int) D {
					if i == 0 {
						return g.headRow(gtx, th, view)
					}
					return g.treeRow(gtx, th, rows[i-1], in)
				})
			})
		}),
	)
}

// headRow narrows the log to what HEAD reaches, and names where HEAD is.
func (g *gitPanel) headRow(gtx C, th *ui.Theme, view gitLogView) D {
	name := ""
	if i := slices.IndexFunc(view.refs.Refs, func(r backend.GitRef) bool { return r.Head }); i >= 0 {
		name = view.refs.Refs[i].Name
	} else if head := view.refs.Head; len(head) >= shortHashLen {
		name = head[:shortHashLen]
	}
	return g.refRow(gtx, th, gitRow(g.treeRows, "HEAD"), gitTreeRow{label: i18n.T("git.panel.head", i18n.Args{"name": name}), ref: "HEAD"}, nil, nil)
}

func (g *gitPanel) treeRow(gtx C, th *ui.Theme, row gitTreeRow, in gitPanelInput) D {
	if row.group {
		return layout.Inset{Left: ui.Space(2.5), Top: ui.Space(2), Bottom: ui.Space(0.5)}.Layout(gtx,
			th.Text(gitGroupLabelSize, strings.ToUpper(row.label)).Muted().Layout)
	}
	var session *sessionView
	if sv, ok := in.sessions[row.branch]; ok && row.branch != "" {
		session = &sv
	}
	r := gitRow(g.treeRows, row.ref)
	entries := func() []ui.MenuEntry { return g.branchEntries(row, in) }
	if e := g.editing; e == nil || !e.create || e.from.ref != row.ref {
		return g.refRow(gtx, th, r, row, session, entries)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return g.refRow(gtx, th, r, row, session, entries) }),
		layout.Rigid(func(gtx C) D { return g.newBranchRow(gtx, th) }),
	)
}

// gitGroupLabelSize is text2XS, the size of a group's uppercase label.
const gitGroupLabelSize = unit.Sp(10.5)

const shortHashLen = 8

// refRow is a selectable ref: choosing it narrows the log, and choosing it
// again goes back to every branch.
func (g *gitPanel) refRow(gtx C, th *ui.Theme, r *gitRowState, row gitTreeRow, session *sessionView, entries func() []ui.MenuEntry) D {
	if entries == nil {
		return g.refRowBody(gtx, th, r, row, session, nil)
	}
	more := func(gtx C) D {
		return th.DropdownMenu(g.root, &r.menu, entries()).Layout(gtx, func(gtx C) D {
			return th.Tooltip(g.root, &r.moreTip, i18n.T("git.branch.options", i18n.Args{"name": row.name})).Layout(gtx, func(gtx C) D {
				return th.CornerButton(&r.more, icGroupMore, r.menu.Opened()).Layout(gtx)
			})
		})
	}
	if r.menu.Opened() || r.context.Opened() {
		g.menuOpen = true
	}
	return th.ContextMenuLazy(g.root, &r.context, entries).Layout(gtx, func(gtx C) D {
		return g.refRowBody(gtx, th, r, row, session, more)
	})
}

// refRowBody is the row itself: its glyph, its name or the field renaming it,
// the ring of a session on it and, under the pointer, the ⋯ of its menu.
func (g *gitPanel) refRowBody(gtx C, th *ui.Theme, r *gitRowState, row gitTreeRow, session *sessionView, more layout.Widget) D {
	renaming := g.editing != nil && !g.editing.create && g.editing.from.ref == row.ref
	for r.click.Clicked(gtx) {
		if renaming {
			continue
		}
		if g.filter.ref == row.ref {
			g.filter.ref = ""
		} else {
			g.filter.ref = row.ref
		}
	}
	return r.click.Layout(gtx, func(gtx C) D {
		size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(gitRowHeight))
		gtx.Constraints = layout.Exact(size)
		hovered := r.click.Hovered() || r.menu.Opened() || r.context.Opened()
		rowFill(gtx, th, g.filter.ref == row.ref, hovered)
		ic := icBranch
		if row.tag {
			ic = icGitTag
		}
		tone := th.MutedForeground
		l := th.Text(ui.TextXS, gitCleanLine(row.label))
		if row.head {
			tone = th.Foreground
			l = l.Weight(font.Medium)
		}
		return layout.Inset{Left: ui.Space(1.5), Right: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return ic.Layout(gtx, ui.Space(3), tone) }),
				ui.Gap(1.5),
				layout.Flexed(1, func(gtx C) D {
					if renaming {
						return g.renameField(gtx, th)
					}
					return l.Truncated().Layout(gtx)
				}),
				layout.Rigid(func(gtx C) D {
					if session == nil {
						return D{}
					}
					return g.sessionMark(gtx, th, r, *session)
				}),
				layout.Rigid(func(gtx C) D {
					size := gtx.Dp(cardSlotSize)
					if more == nil || !hovered {
						return D{Size: image.Pt(0, size)}
					}
					gtx.Constraints = layout.Exact(image.Pt(size, size))
					return layout.Inset{Left: ui.Space(1)}.Layout(gtx, more)
				}),
			)
		})
	})
}
