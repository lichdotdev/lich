package main

import (
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

// actions is the header's buttons, left to right: the hand-off a failing or
// conflicting pull request offers, the session, the pending review while one
// is written, Merge, and ⋯ with Approve, Refresh and Open on GitHub.
func (v *pullView) actions(gtx C, th *ui.Theme, p *pullsScreen, d backend.PRDetail, act *pullsAction) D {
	busy := v.busyNow()
	blocked := ""
	if d.IsCrossRepository && !d.MaintainerCanModify {
		blocked = i18n.T("pulls.pulls.forkBlocked")
	}
	if v.session.Clicked(gtx) && blocked == "" {
		act.session = &d
	}
	reviewBlocked := mergeReasonForReview(d)
	var items []ui.WrapChild
	if handoff := v.handoffButton(gtx, th, p, d, busy, blocked, act); handoff != nil {
		items = append(items, ui.WrapItem(handoff))
	}
	items = append(items, ui.WrapItem(func(gtx C) D { return v.sessionButton(gtx, th, p, d, busy, blocked) }))
	if len(v.pendingOf(d.URL).snapshot()) > 0 {
		items = append(items, ui.WrapItem(func(gtx C) D { return v.reviewButton(gtx, th, p, d, reviewBlocked) }))
	}
	items = append(items,
		ui.WrapItem(func(gtx C) D { return v.mergeButton(gtx, th, p, d, busy, act) }),
		ui.WrapItem(func(gtx C) D { return v.moreButton(gtx, th, p, d, busy, reviewBlocked, act) }),
	)
	return ui.Wrap{GapX: ui.Space(2), GapY: ui.Space(2), Justify: ui.JustifyEnd}.Layout(gtx, items...)
}

// moreButton is ⋯: Approve while no review is being written, Refresh and
// Open on GitHub.
func (v *pullView) moreButton(gtx C, th *ui.Theme, p *pullsScreen, d backend.PRDetail, busy, reviewBlocked string, act *pullsAction) D {
	var entries []ui.MenuEntry
	if len(v.pendingOf(d.URL).snapshot()) == 0 {
		label := i18n.T("pulls.pullRequestView.approve")
		switch {
		case busy == "approve":
			label = i18n.T("pulls.pullRequestView.approving")
		case d.ReviewDecision == "APPROVED":
			label = i18n.T("pulls.pullRequestView.approved")
		}
		entries = append(entries, ui.MenuEntry{Label: label, Icon: icPullCheck,
			Disabled: busy != "" || reviewBlocked != "" || d.ReviewDecision == "APPROVED",
			OnClick:  func() { act.approve = true }})
	}
	url := d.URL
	entries = append(entries,
		ui.MenuEntry{Kind: ui.MenuSeparator},
		ui.MenuEntry{Label: i18n.T("pulls.pullRequestView.refresh"), Icon: icPullRefresh, OnClick: func() { act.refresh = true }},
		ui.MenuEntry{Label: i18n.T("pulls.pullsEmptyState.openOnGitHub"), Icon: icPullOut, OnClick: func() { act.external = url }},
	)
	b := th.Button(&v.moreClick, "")
	b.Icon, b.Variant, b.Size = icPullMore, ui.VariantGhost, ui.SizeIconSM
	return v.withTip(gtx, th, p, "more", i18n.T("pulls.pullsNav.more"), func(gtx C) D {
		return th.DropdownMenu(p.root, &v.moreMenu, entries).Layout(gtx, b.Layout)
	})
}

// handoffButton is Resolve conflicts or Fix checks, handing the pull request
// to its session; nil when nothing is wrong with it.
func (v *pullView) handoffButton(gtx C, th *ui.Theme, p *pullsScreen, d backend.PRDetail, busy, blocked string, act *pullsAction) layout.Widget {
	var conflicts []string
	if conflictsWithBase(d) {
		conflicts = p.store.conflictFiles(p.project.Path, &d).val
	}
	label, prompt, ok := pullHandoff(d, conflicts)
	if !ok {
		return nil
	}
	disabled := blocked != "" || busy != ""
	if v.handoff.Clicked(gtx) && !disabled {
		act.session, act.handoff = &d, prompt
	}
	return func(gtx C) D {
		b := th.Button(&v.handoff, label)
		b.Icon, b.Variant, b.Size, b.Disabled = icPullWrench, ui.VariantOutline, ui.SizeSM, disabled
		return v.withTip(gtx, th, p, "handoff", blocked, b.Layout)
	}
}

// sessionButton is Open in Session, or Go to session when a session already
// sits in the branch's checkout.
func (v *pullView) sessionButton(gtx C, th *ui.Theme, p *pullsScreen, d backend.PRDetail, busy, blocked string) D {
	label := i18n.T("pulls.pulls.openInSession")
	if id, _ := p.reviewSession(d); id != "" {
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
func mergeReasonForReview(d backend.PRDetail) string {
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
func (v *pullView) mergeButton(gtx C, th *ui.Theme, p *pullsScreen, d backend.PRDetail, busy string, act *pullsAction) D {
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
func (v *pullView) conflictsStat(gtx C, th *ui.Theme, p *pullsScreen, d backend.PRDetail, face layout.Widget) D {
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

func (v *pullView) panelLayout(gtx C, th *ui.Theme, p *pullsScreen, d backend.PRDetail, act *pullsAction) D {
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
		return v.overview(gtx, th, p, d, act)
	}
	return v.panelList(gtx, th, rows)
}

func (v *pullView) panelList(gtx C, th *ui.Theme, rows []layout.Widget) D {
	return layout.Inset{Top: ui.Space(1), Bottom: ui.Space(1)}.Layout(gtx, func(gtx C) D {
		return th.ScrollArea(&v.panel).Layout(gtx, len(rows), func(gtx C, i int) D { return rows[i](gtx) })
	})
}

// The Overview's columns: the description held to a reading width, the facts
// beside it; an island narrower than overviewSplitWidth puts the facts over
// the description.
var (
	overviewProseWidth = unit.Dp(720)
	overviewFactsWidth = unit.Dp(280)
	overviewSplitWidth = unit.Dp(760)
)

// overview is the description beside the pull request's facts.
func (v *pullView) overview(gtx C, th *ui.Theme, p *pullsScreen, d backend.PRDetail, act *pullsAction) D {
	rows := v.overviewRows(th, d, act)
	if gtx.Constraints.Max.X < gtx.Dp(overviewSplitWidth) {
		facts := func(gtx C) D {
			return layout.Inset{Left: pullsPadX, Right: pullsPadX, Top: ui.Space(4)}.Layout(gtx, func(gtx C) D { return v.facts(gtx, th, p, d) })
		}
		return v.panelList(gtx, th, append([]layout.Widget{facts}, rows...))
	}
	return layout.Flex{}.Layout(gtx,
		layout.Flexed(1, func(gtx C) D { return v.panelList(gtx, th, rows) }),
		layout.Rigid(func(gtx C) D { return verticalRule(gtx, th) }),
		layout.Rigid(func(gtx C) D {
			w := gtx.Dp(overviewFactsWidth)
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = w, w
			return layout.Inset{Left: ui.Space(5), Right: ui.Space(5), Top: ui.Space(5)}.Layout(gtx, func(gtx C) D { return v.facts(gtx, th, p, d) })
		}),
	)
}

// facts is what the status line says on the other tabs, told in full: who
// reviews it, the checks, whether it merges, and the checkout and sessions
// its branch sits in.
func (v *pullView) facts(gtx C, th *ui.Theme, p *pullsScreen, d backend.PRDetail) D {
	vs := v.verdicts(gtx, th, p, d)
	var children []layout.FlexChild
	section := func(title string, body ...layout.Widget) {
		if len(children) > 0 {
			children = append(children, ui.VGap(5))
		}
		children = append(children, layout.Rigid(sectionHeading(th, title)))
		for _, w := range body {
			if w != nil {
				children = append(children, ui.VGap(2), layout.Rigid(w))
			}
		}
	}
	section(i18n.T("pulls.pullsOverview.reviewers"), vs.review, func(gtx C) D { return reviewerChips(gtx, th, d.Reviewers) })
	if vs.checks != nil {
		section(i18n.T("pulls.pullRequestView.tab.checks"), vs.checks)
	}
	if vs.merge != nil {
		section(i18n.T("pulls.pullRequestView.merge"), vs.merge)
	}
	section(i18n.T("pulls.pullsNav.checkout"), v.checkoutFact(th, p, d)...)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// checkoutFact is the checkout the branch sits in and the sessions in it, or
// that it sits in none.
func (v *pullView) checkoutFact(th *ui.Theme, p *pullsScreen, d backend.PRDetail) []layout.Widget {
	wt, ok := p.checkedOut(d.HeadRefName)
	if !ok {
		return []layout.Widget{th.Text(ui.TextSM, i18n.T("pulls.pullsNav.notCheckedOut")).Muted().Layout}
	}
	out := []layout.Widget{func(gtx C) D { return metaGlyph(gtx, th, icCheckout, checkoutName(wt), th.Foreground) }}
	for _, id := range p.sessionsIn(p.project.ID, wt.Path) {
		out = append(out, func(gtx C) D { return metaGlyph(gtx, th, icPullSession, p.sessionLabel(id), th.MutedForeground) })
	}
	return out
}

// overviewRows is the description under its heading, held to a reading
// width.
func (v *pullView) overviewRows(th *ui.Theme, d backend.PRDetail, act *pullsAction) []layout.Widget {
	pad := func(top float32, w layout.Widget) layout.Widget {
		return func(gtx C) D {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(overviewProseWidth+2*pullsPadX))
			return layout.Inset{Left: pullsPadX, Right: pullsPadX, Top: ui.Space(top)}.Layout(gtx, w)
		}
	}
	body := strings.TrimSpace(d.Body)
	return []layout.Widget{
		pad(4, func(gtx C) D { return v.descriptionHeading(gtx, th, d) }),
		pad(2, func(gtx C) D {
			if v.edit.bodyURL == d.URL {
				return v.descriptionField(gtx, th, d, act)
			}
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
func reviewerChips(gtx C, th *ui.Theme, reviewers []backend.PRReviewer) D {
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
func (v *pullView) commitRows(gtx C, th *ui.Theme, d backend.PRDetail) []layout.Widget {
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
func commitRow(gtx C, th *ui.Theme, c backend.PRCommit, click *widget.Clickable, hasBody, open bool, now time.Time) D {
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
func (v *pullView) checkRows(gtx C, th *ui.Theme, d backend.PRDetail, act *pullsAction) []layout.Widget {
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
					layout.Rigid(func(gtx C) D { return pullGlyph(gtx, th, ic, ui.Space(4), fg) }),
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

// panelRow is a tab's py-2 row, hover-filled like a sidebar card while it
// does something.
func panelRow(gtx C, th *ui.Theme, click *widget.Clickable, live bool, content func(gtx C, th *ui.Theme) D) D {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return click.Layout(gtx, func(gtx C) D {
		bg := th.Surface
		if live && click.Hovered() {
			bg = th.Over(ui.Alpha(th.Accent, ghostHoverFill))
		}
		return layout.Inset{Left: pullsRowEdge, Right: pullsRowEdge}.Layout(gtx, func(gtx C) D {
			return layout.Background{}.Layout(gtx,
				func(gtx C) D { return ui.Fill(gtx, bg, ui.RadiusMD) },
				func(gtx C) D {
					return layout.Inset{Left: pullsRowPadX, Right: pullsRowPadX, Top: ui.Space(2), Bottom: ui.Space(2)}.Layout(gtx, func(gtx C) D {
						return content(gtx, th.On(bg))
					})
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
