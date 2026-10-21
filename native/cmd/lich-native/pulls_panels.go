package main

import (
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/lichclient"
	"github.com/lichdotdev/lich/native/ui"
)

// actions is the header's buttons, left to right: the session, the hand-off,
// Approve, Merge, Refresh and Open.
func (v *pullView) actions(gtx C, th *ui.Theme, p *pullsScreen, d lichclient.PRDetail, act *pullsAction) D {
	busy := v.busyNow()
	handLabel, handPrompt, handOK := pullHandoff(d)
	blocked := ""
	if d.IsCrossRepository && !d.MaintainerCanModify {
		blocked = i18n.T("pulls.pulls.forkBlocked")
	}
	if v.session.Clicked(gtx) && blocked == "" {
		act.session = &d
	}
	if v.handoff.Clicked(gtx) && blocked == "" {
		act.session, act.handoff = &d, handPrompt
	}
	reviewBlocked := mergeReasonForReview(d)
	if v.approve.Clicked(gtx) && reviewBlocked == "" && d.ReviewDecision != "APPROVED" {
		act.approve = true
	}
	if v.refresh.Clicked(gtx) {
		act.refresh = true
	}
	if v.external.Clicked(gtx) {
		act.external = d.URL
	}
	items := []ui.WrapChild{ui.WrapItem(func(gtx C) D { return v.sessionButton(gtx, th, p, d, busy, blocked) })}
	if handOK {
		items = append(items, ui.WrapItem(func(gtx C) D {
			b := th.Button(&v.handoff, handLabel)
			b.Icon, b.Variant, b.Size, b.Disabled = icPullWrench, ui.VariantGhost, ui.SizeSM, blocked != "" || busy != ""
			return v.withTip(gtx, th, p, "handoff", blocked, b.Layout)
		}))
	}
	items = append(items,
		ui.WrapItem(func(gtx C) D { return v.reviewButton(gtx, th, p, d, busy, reviewBlocked, act) }),
		ui.WrapItem(func(gtx C) D { return v.mergeButton(gtx, th, p, d, busy, act) }),
		ui.WrapItem(func(gtx C) D {
			b := th.Button(&v.refresh, "")
			b.Icon, b.Variant, b.Size = icPullRefresh, ui.VariantGhost, ui.SizeIconSM
			return v.withTip(gtx, th, p, "refresh", i18n.T("pulls.pullRequestView.refresh"), b.Layout)
		}),
		ui.WrapItem(func(gtx C) D {
			b := th.Button(&v.external, i18n.T("pulls.pullRequestView.open"))
			b.Icon, b.Variant, b.Size = icPullOut, ui.VariantGhost, ui.SizeSM
			return b.Layout(gtx)
		}),
	)
	return ui.Wrap{GapX: ui.Space(2), GapY: ui.Space(2), Justify: ui.JustifyEnd}.Layout(gtx, items...)
}

// sessionButton is Open in Session, or Go to session when a session already
// sits in the branch's checkout.
func (v *pullView) sessionButton(gtx C, th *ui.Theme, p *pullsScreen, d lichclient.PRDetail, busy, blocked string) D {
	label := i18n.T("pulls.pulls.openInSession")
	if wt, ok := p.checkedOut(d.HeadRefName); ok && p.hasSessionAt(wt.Path) {
		label = i18n.T("pulls.pulls.goToSession")
	}
	if busy == "session" {
		label = i18n.T("pulls.pullRequestView.opening")
	}
	b := th.Button(&v.session, label)
	b.Icon, b.Variant, b.Size, b.Disabled = icPullSession, ui.VariantSecondary, ui.SizeSM, blocked != "" || busy != ""
	return v.withTip(gtx, th, p, "session", blocked, b.Layout)
}

// mergeReasonForReview is why a review cannot be left: only a pull request
// that is over refuses one, so a draft or a conflict still takes it.
func mergeReasonForReview(d lichclient.PRDetail) string {
	switch d.State {
	case "OPEN":
		return ""
	case "MERGED":
		return i18n.T("pulls.mergeGate.merged")
	}
	return i18n.T("pulls.mergeGate.closed")
}

// withTip wraps w in a tooltip saying text, w alone for none.
func (v *pullView) withTip(gtx C, th *ui.Theme, p *pullsScreen, key, text string, w layout.Widget) D {
	if text == "" {
		return w(gtx)
	}
	return th.Tooltip(p.root, v.tip(key), text).Layout(gtx, w)
}

// mergeButton is the Merge menu: the methods the base's rules allow, every
// one a bypass or none, enabled past a block only where a bypass is offered.
func (v *pullView) mergeButton(gtx C, th *ui.Theme, p *pullsScreen, d lichclient.PRDetail, busy string, act *pullsAction) D {
	rules := p.store.branchRules(p.project.Path, d.BaseRefName)
	bypass := canAdminOverride(d, rules)
	blocked := mergeBlockedReason(d) != "" && !bypass
	var entries []ui.MenuEntry
	for _, m := range allowedMergeMethods(rules) {
		label := mergeMethodLabel(m)
		if bypass {
			label = i18n.T("pulls.pullRequestView.methodBypass", i18n.Args{"method": label})
		}
		entries = append(entries, ui.MenuEntry{Label: label, OnClick: func() { act.merge, act.admin = m, bypass }})
	}
	entries = append(entries, v.editEntries(allowedMergeMethods(rules), bypass, d)...)
	label := i18n.T("pulls.pullRequestView.merge")
	if busy == "merge" {
		label = i18n.T("pulls.pullRequestView.merging")
	}
	if d.Checks.Failed > 0 && busy != "merge" {
		label = i18n.T("pulls.pullsPanels.mergeFailing", i18n.Args{"count": d.Checks.Failed})
	}
	b := th.Button(&v.mergeClick, label)
	b.Icon, b.Size, b.Disabled = icPullMerged, ui.SizeSM, blocked || busy != ""
	if b.Disabled {
		return v.withTip(gtx, th, p, "merge", mergeHint(d, rules), b.Layout)
	}
	menu := th.DropdownMenu(p.root, &v.mergeMenu, entries)
	return v.withTip(gtx, th, p, "merge", mergeHint(d, rules), func(gtx C) D { return menu.Layout(gtx, b.Layout) })
}

// conflictsPanelWidth and conflictsPanelHeight bound the conflicting files'
// panel; a longer list scrolls inside it.
const (
	conflictsPanelWidth  = unit.Dp(520)
	conflictsPanelHeight = unit.Dp(260)
)

// conflictsStat is the "Conflicts with" stat, face: once the files are known
// it opens a panel naming them, and until then its tooltip says why not.
func (v *pullView) conflictsStat(gtx C, th *ui.Theme, p *pullsScreen, d lichclient.PRDetail, face layout.Widget) D {
	read := p.store.conflictFiles(p.project.Path, &d)
	if hint := conflictsHint(read); hint != "" {
		return v.withTip(gtx, th, p, "conflicts", hint, face)
	}
	files := read.val
	pop := th.DropdownPanel(p.root, &v.conflictsPanel, conflictsPanelWidth, func(gtx C, th *ui.Theme) D {
		return v.conflictsList(gtx, th, files)
	})
	count := i18n.T("pulls.pullsStats.conflictFiles", i18n.Args{"count": len(files)})
	return pop.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(face),
			ui.Gap(1.5),
			layout.Rigid(th.Text(ui.TextXS, count).Muted().Layout),
			ui.Gap(1),
			layout.Rigid(func(gtx C) D { return icChevronDown.Layout(gtx, ui.Space(3.5), th.MutedForeground) }),
		)
	})
}

// conflictsHint is why the conflicting files cannot be listed yet, empty
// once they can.
func conflictsHint(read pullsRead[[]string]) string {
	switch {
	case !read.loaded:
		return i18n.T("pulls.pullsConflicts.loading")
	case read.err != "":
		return i18n.T("pulls.pullsConflicts.failed", i18n.Args{"error": read.err})
	case len(read.val) == 0:
		return i18n.T("pulls.pullsConflicts.stale")
	}
	return ""
}

// conflictsList is the panel's content: the count, then a row per file.
func (v *pullView) conflictsList(gtx C, th *ui.Theme, files []string) D {
	gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(conflictsPanelHeight))
	heading := i18n.T("pulls.pullsConflicts.count", i18n.Args{"count": len(files)})
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1.5), Bottom: ui.Space(1)}.Layout(gtx, sectionHeading(th, heading))
		}),
		layout.Rigid(func(gtx C) D {
			return th.ScrollArea(&v.conflictsRows).Layout(gtx, len(files), func(gtx C, i int) D { return conflictRow(gtx, th, files[i]) })
		}),
	)
}

// conflictRow is one conflicting file: its name, then its directory dimmed,
// the directory cut first when the row runs out of room.
func conflictRow(gtx C, th *ui.Theme, file string) D {
	dir, base := splitPath(file)
	return layout.Inset{Left: ui.Space(2), Right: ui.Space(2), Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return truncated(gtx, th.Text(ui.TextXS, base).Mono().Weight(font.Medium)) }),
			ui.Gap(2),
			layout.Flexed(1, func(gtx C) D { return truncated(gtx, th.Text(ui.TextXS, dir).Mono().Muted()) }),
		)
	})
}

func (v *pullView) panelLayout(gtx C, th *ui.Theme, p *pullsScreen, d lichclient.PRDetail, act *pullsAction) D {
	var rows []layout.Widget
	switch v.openTab(d) {
	case tabFiles:
		return v.filesTab(gtx, th, p, d, act)
	case tabConversation:
		rows = v.conversationRows(gtx, th, p, d, act)
	case tabCommits:
		rows = v.commitRows(gtx, th, d)
	case tabChecks:
		rows = v.checkRows(gtx, th, d, act)
	default:
		rows = v.overviewRows(th, d, act)
	}
	return layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return th.ScrollArea(&v.panel).Layout(gtx, len(rows), func(gtx C, i int) D { return rows[i](gtx) })
	})
}

// overviewRows is PullsOverview: who opened it, who reviews it, and its
// description.
func (v *pullView) overviewRows(th *ui.Theme, d lichclient.PRDetail, act *pullsAction) []layout.Widget {
	pad := func(top float32, w layout.Widget) layout.Widget {
		return func(gtx C) D {
			return layout.Inset{Left: pullsPadX, Right: pullsPadX, Top: ui.Space(top)}.Layout(gtx, w)
		}
	}
	body := strings.TrimSpace(d.Body)
	return []layout.Widget{
		pad(4, th.Text(ui.TextSM, i18n.T("pulls.pullsOverview.openedBy", i18n.Args{"author": d.Author})).Muted().Layout),
		pad(5, sectionHeading(th, i18n.T("pulls.pullsOverview.reviewers"))),
		pad(2, func(gtx C) D { return reviewerChips(gtx, th, d.Reviewers) }),
		pad(5, sectionHeading(th, i18n.T("pulls.pullsOverview.description"))),
		pad(2, func(gtx C) D {
			if body == "" {
				return th.Text(ui.TextSM, i18n.T("pulls.pullsOverview.noDescription")).Muted().Layout(gtx)
			}
			return v.md.layout(gtx, th, "body:"+d.URL, body, act.open)
		}),
	}
}

func sectionHeading(th *ui.Theme, text string) layout.Widget {
	return th.Text(ui.TextXS, strings.ToUpper(text)).Muted().Weight(font.Medium).Layout
}

// reviewVerdict is what a review state says its author did, ok false for a
// state GitHub has but this window has no words for.
func reviewVerdict(state string) (text string, ok bool) {
	switch state {
	case "APPROVED":
		return i18n.T("pulls.verdict.approved"), true
	case "CHANGES_REQUESTED":
		return i18n.T("pulls.verdict.changesRequested"), true
	case "COMMENTED":
		return i18n.T("pulls.verdict.commented"), true
	case "DISMISSED":
		return i18n.T("pulls.verdict.dismissed"), true
	}
	return "", false
}

// reviewerChips is the reviewers row: each login and what they said.
func reviewerChips(gtx C, th *ui.Theme, reviewers []lichclient.PRReviewer) D {
	if len(reviewers) == 0 {
		return th.Text(ui.TextSM, i18n.T("pulls.pullsOverview.nobodyYet")).Muted().Layout(gtx)
	}
	items := make([]ui.WrapChild, len(reviewers))
	for i, r := range reviewers {
		fg := th.MutedForeground
		ic := icPullDashed
		switch r.State {
		case "APPROVED":
			fg, ic = th.TonePass, icPullCheck
		case "CHANGES_REQUESTED":
			fg, ic = th.Destructive, icPullX
		}
		if r.IsTeam {
			ic = icPullTeam
		}
		verdict, _ := reviewVerdict(r.State)
		if r.State == "" {
			verdict = i18n.T("pulls.verdict.requested")
		}
		items[i] = ui.WrapItem(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return ic.Layout(gtx, ui.Space(3.5), fg) }),
				ui.Gap(1.5),
				layout.Rigid(th.Text(ui.TextSM, r.Login).Layout),
				ui.Gap(1),
				layout.Rigid(th.Text(ui.TextXS, verdict).Muted().Layout),
			)
		})
	}
	return ui.Wrap{GapX: ui.Space(4), GapY: ui.Space(1.5)}.Layout(gtx, items...)
}

// commitRows is PullsCommits: each commit, its body behind a click.
func (v *pullView) commitRows(gtx C, th *ui.Theme, d lichclient.PRDetail) []layout.Widget {
	if len(d.Commits) == 0 {
		return []layout.Widget{func(gtx C) D { return panelNotice(gtx, th, i18n.T("pulls.pullsCommits.none")) }}
	}
	now := gtx.Now
	rows := make([]layout.Widget, len(d.Commits))
	for i, c := range d.Commits {
		open := boolOf(v.commitOpen, c.OID)
		click := clickableOf(v.commitClicks, c.OID)
		body := strings.TrimSpace(c.Body)
		rows[i] = func(gtx C) D {
			if click.Clicked(gtx) && body != "" {
				open.Value = !open.Value
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return commitRow(gtx, th, c, click, body != "", open.Value, now) }),
				layout.Rigid(func(gtx C) D {
					if !open.Value {
						return D{}
					}
					return layout.Inset{Left: pullsPadX, Right: pullsPadX, Bottom: ui.Space(3)}.Layout(gtx, func(gtx C) D {
						return wrapped(gtx, th.Text(ui.TextXS, body).Muted())
					})
				}),
			)
		}
	}
	return rows
}

// commitRow is a commit's headline, who and when, the chevron when it has a
// body to open, and its short oid.
func commitRow(gtx C, th *ui.Theme, c lichclient.PRCommit, click *widget.Clickable, hasBody, open bool, now time.Time) D {
	return panelRow(gtx, th, click, hasBody, func(gtx C, th *ui.Theme) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(th.Text(ui.TextSM, c.Headline).Weight(font.Medium).Layout),
					layout.Rigid(th.Text(ui.TextXS, commitMeta(c, now)).Muted().Layout),
				)
			}),
			layout.Rigid(func(gtx C) D {
				if !hasBody {
					return D{}
				}
				ic := icPullRight
				if open {
					ic = icPullDown
				}
				return layout.Inset{Right: ui.Space(2)}.Layout(gtx, func(gtx C) D { return ic.Layout(gtx, ui.Space(3.5), th.MutedForeground) })
			}),
			layout.Rigid(th.Text(ui.TextXS, c.OID[:min(len(c.OID), 7)]).Mono().Muted().Layout),
		)
	})
}

// checkRows is PullsChecks: each check, its run opened in the browser.
func (v *pullView) checkRows(gtx C, th *ui.Theme, d lichclient.PRDetail, act *pullsAction) []layout.Widget {
	if len(d.CheckRuns) == 0 {
		return []layout.Widget{func(gtx C) D { return panelNotice(gtx, th, i18n.T("pulls.pullsChecks.none")) }}
	}
	now := gtx.Now
	rows := make([]layout.Widget, len(d.CheckRuns))
	for i, c := range d.CheckRuns {
		click := clickableOf(v.checkClicks, c.Name+"\n"+c.URL)
		rows[i] = func(gtx C) D {
			if click.Clicked(gtx) && c.URL != "" {
				act.external = c.URL
			}
			ic, fg := icPullCheck, th.TonePass
			switch c.State {
			case "failed":
				ic, fg = icPullX, th.Destructive
			case "pending":
				ic, fg = icPullRunning, th.ToneWait
			}
			return panelRow(gtx, th, click, c.URL != "", func(gtx C, th *ui.Theme) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return pullGlyph(gtx, ic, ui.Space(4), fg) }),
					ui.Gap(3),
					layout.Flexed(1, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(th.Text(ui.TextSM, c.Name).Layout),
							layout.Rigid(func(gtx C) D {
								if c.Description == "" {
									return D{}
								}
								return th.Text(ui.TextXS, c.Description).Muted().Layout(gtx)
							}),
						)
					}),
					layout.Rigid(func(gtx C) D {
						if !c.Required {
							return D{}
						}
						return layout.Inset{Right: ui.Space(3)}.Layout(gtx, paletteChip(th, i18n.T("pulls.pullsChecks.required"), th.MutedForeground))
					}),
					layout.Rigid(th.Text(ui.TextXS, checkDuration(c.StartedAt, c.CompletedAt, now)).Muted().Layout),
				)
			})
		}
	}
	return rows
}

// panelRow is a tab's px-6 py-2 row, hover-filled while it does something.
func panelRow(gtx C, th *ui.Theme, click *widget.Clickable, live bool, content func(gtx C, th *ui.Theme) D) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return click.Layout(gtx, func(gtx C) D {
		bg := th.Surface
		if live && click.Hovered() {
			bg = th.Over(ui.Alpha(th.Accent, ghostHoverFill))
		}
		return layout.Background{}.Layout(gtx,
			func(gtx C) D { return ui.Fill(gtx, bg, 0) },
			func(gtx C) D {
				return layout.Inset{Left: pullsPadX, Right: pullsPadX, Top: ui.Space(2), Bottom: ui.Space(2)}.Layout(gtx, func(gtx C) D {
					return content(gtx, th.On(bg))
				})
			})
	})
}

func panelNotice(gtx C, th *ui.Theme, text string) D {
	return layout.Inset{Left: pullsPadX, Right: pullsPadX, Top: ui.Space(5), Bottom: ui.Space(5)}.Layout(gtx,
		th.Text(ui.TextSM, text).Muted().Layout)
}

func boolOf(m map[string]*widget.Bool, id string) *widget.Bool {
	b, ok := m[id]
	if !ok {
		b = new(widget.Bool)
		m[id] = b
	}
	return b
}
