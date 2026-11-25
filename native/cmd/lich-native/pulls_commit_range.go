package main

import (
	"image"
	"image/color"
	"slices"
	"strconv"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// commitPickWidth is the commit picker's panel.
var commitPickWidth = unit.Dp(360)

// commitPick is the commits the Files tab shows: all of them, or the
// contiguous run first to last of url's pull request. anchor is the commit a
// Shift+click extends from.
type commitPick struct {
	url, first, last, anchor string

	panel ui.DropdownPanelState
	all   ui.Clickable
	rows  map[string]*ui.Clickable
	// views are the Commits tab's View changes, by oid.
	views map[string]*ui.Clickable
}

// span is the picked commits as indexes into commits, ok false for all of
// them, or for a pick a push has since taken out of the pull request.
func (k *commitPick) span(url string, commits []backend.PRCommit) (from, to int, ok bool) {
	if k.url != url || k.first == "" {
		return 0, 0, false
	}
	at := func(oid string) int {
		return slices.IndexFunc(commits, func(c backend.PRCommit) bool { return c.OID == oid })
	}
	from, to = at(k.first), at(k.last)
	return from, to, from >= 0 && to >= from
}

// pick is a click on commits[i]: it alone, or with extend the run from the
// anchor to it.
func (k *commitPick) pick(url string, commits []backend.PRCommit, i int, extend bool) {
	anchor := slices.IndexFunc(commits, func(c backend.PRCommit) bool { return c.OID == k.anchor })
	if !extend || k.url != url || anchor < 0 {
		anchor = i
		k.anchor = commits[i].OID
	}
	k.url, k.first, k.last = url, commits[min(anchor, i)].OID, commits[max(anchor, i)].OID
}

func (k *commitPick) clear() { k.first, k.last, k.anchor = "", "", "" }

// commitPicker is the Files tab's chip naming the commits shown, and the
// panel that picks them.
func (v *pullView) commitPicker(gtx C, th *ui.Theme, p *pullsScreen, d backend.PRDetail) D {
	k := &v.pick
	label := i18n.T("pulls.pullsFiles.allCommits")
	if from, to, ok := k.span(d.URL, d.Commits); ok {
		label = i18n.T("pulls.pullsFiles.commits", i18n.Args{"count": to - from + 1})
	}
	pop := th.DropdownPanel(p.root, &k.panel, commitPickWidth, func(gtx C, th *ui.Theme) D {
		return v.commitPickList(gtx, th, d)
	})
	return pop.Layout(gtx, func(gtx C) D {
		return buttonFace(gtx, th, true, k.panel.Opened(), false, func(gtx C, th *ui.Theme, fg color.NRGBA) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(th.Text(ui.TextXS, label).In(fg).Layout),
				ui.Gap(1),
				layout.Rigid(func(gtx C) D { return icPullDown.Layout(gtx, chatCrumbGlyph, fg) }),
			)
		})
	})
}

// commitPickList is the panel: All commits with the count, each commit
// oldest first, then the Shift+click hint.
func (v *pullView) commitPickList(gtx C, th *ui.Theme, d backend.PRDetail) D {
	k := &v.pick
	from, to, ranged := k.span(d.URL, d.Commits)
	if k.all.Clicked(gtx) {
		k.clear()
		k.panel.Close()
		gtx.Execute(op.InvalidateCmd{})
	}
	if k.rows == nil {
		k.rows = map[string]*ui.Clickable{}
	}
	now := gtx.Now
	rows := []layout.FlexChild{layout.Rigid(func(gtx C) D {
		return commitPickRow(gtx, th, &k.all, !ranged, func(gtx C, on *ui.Theme) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, on.Text(ui.TextSM, i18n.T("pulls.pullsFiles.allCommits")).Layout),
				layout.Rigid(on.Text(ui.TextXS, strconv.Itoa(len(d.Commits))).Muted().Layout),
			)
		})
	}), layout.Rigid(func(gtx C) D { return pickSeparator(gtx, th) })}
	for i, c := range d.Commits {
		click := clickableOf(k.rows, c.OID)
		for {
			e, ok := click.Update(gtx)
			if !ok {
				break
			}
			k.pick(d.URL, d.Commits, i, e.Modifiers.Contain(key.ModShift))
			if !e.Modifiers.Contain(key.ModShift) {
				k.panel.Close()
			}
			// The panel is drawn after the chip and the files: draw them again.
			gtx.Execute(op.InvalidateCmd{})
		}
		rows = append(rows, layout.Rigid(func(gtx C) D {
			return commitPickRow(gtx, th, click, ranged && i >= from && i <= to, func(gtx C, on *ui.Theme) D {
				if !click.Hovered() {
					return commitPickLine(gtx, on, c, now)
				}
				// Shift's cap where it acts: a Shift+click picks the range to here
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx C) D { return commitPickLine(gtx, on, c, now) }),
					ui.Gap(2),
					layout.Rigid(on.KeyHint("⇧", i18n.T("pulls.pullsFiles.rangeKey")).Layout),
				)
			})
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}

// commitPickLine is a commit's short oid, headline and age.
func commitPickLine(gtx C, th *ui.Theme, c backend.PRCommit, now time.Time) D {
	age := ""
	if at, err := time.Parse(time.RFC3339, c.Date); err == nil {
		age = agoLabel(at.Unix(), now)
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(th.Text(ui.TextXS, shortOID(c.OID)).Mono().Muted().Layout),
		ui.Gap(2),
		layout.Flexed(1, th.Text(ui.TextSM, c.Headline).Truncated().Layout),
		ui.Gap(2),
		layout.Rigid(th.Text(ui.TextXS, age).Muted().Layout),
	)
}

// commitPickRow is a SelectItem with its check at the start, as the panel's
// rows all are.
func commitPickRow(gtx C, th *ui.Theme, click *ui.Clickable, checked bool, content func(gtx C, on *ui.Theme) D) D {
	return click.Layout(gtx, func(gtx C) D {
		gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, gtx.Dp(fontRowHeight)))
		bg := th.Surface
		if click.Hovered() {
			bg = th.Accent
			ui.Fill(gtx, bg, ui.RadiusSM)
		}
		on := th.On(bg)
		return layout.Inset{Left: ui.Space(2), Right: ui.Space(2)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					if !checked {
						return D{Size: image.Pt(gtx.Dp(ui.Space(4)), 0)}
					}
					return icCheck.Layout(gtx, ui.Space(4), on.Foreground)
				}),
				ui.Gap(2),
				layout.Flexed(1, func(gtx C) D { return content(gtx, on) }),
			)
		})
	})
}

// viewChanges is a Commits row's View changes: the Files tab, showing that
// commit alone.
func (v *pullView) viewChanges(gtx C, th *ui.Theme, d backend.PRDetail, i int) D {
	k := &v.pick
	if k.views == nil {
		k.views = map[string]*ui.Clickable{}
	}
	click := clickableOf(k.views, d.Commits[i].OID)
	if click.Clicked(gtx) {
		k.pick(d.URL, d.Commits, i, false)
		v.showTab(d, tabFiles)
		// The tab strip is laid out before this row: draw the Files tab now.
		gtx.Execute(op.InvalidateCmd{})
	}
	b := th.Button(click, i18n.T("pulls.pullsCommits.viewChanges"))
	b.Variant, b.Size = ui.VariantGhost, ui.SizeXS
	return b.Layout(gtx)
}

// pickSeparator is the space between a menu's groups, where a hairline stood.
func pickSeparator(gtx C, th *ui.Theme) D {
	return layout.Spacer{Height: ui.Space(2)}.Layout(gtx)
}

func shortOID(oid string) string { return oid[:min(len(oid), 7)] }
