package main

import (
	"strconv"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// namedConflicts is how many conflicting files the tooltip names before
// counting the rest.
const namedConflicts = 3

var (
	icSandboxed = icons.Lucide("shield")
	icConflict  = icons.Lucide("triangle-alert")
	icBranch    = icons.Lucide("git-branch")
	icPR        = icons.Lucide("git-pull-request-arrow")
	icModel     = icons.Lucide("cpu")
)

// sessionTooltip is what the card's one line has no room for: the full label,
// what the turn is doing, the full path, the shield's symlinks left out, the
// branch and its standing against the base when no checkout row above says
// them, and last the model the provider says it is running beside how long
// the status has lasted.
func sessionTooltip(th *ui.Theme, sv *sessionView) layout.Widget {
	th = th.On(th.Popover)
	path := sv.shown
	if sv.host != "" {
		path = unknownCwd(sv.host)
	}
	sections := []layout.Widget{th.Text(ui.TextXS, sv.Label).Weight(font.Medium).Layout}
	if rung, ok := statusRung(th, sv); ok {
		sections = append(sections, rung)
	}
	sections = append(sections, para(th.Text(ui.TextXS, path).Muted().Mono()).Layout)
	if len(sv.sandbox.SkippedLinks) > 0 {
		sections = append(sections, sandboxSection(th, sv.sandbox))
	}
	// Under a checkout row, the checkout's facts are that row's tooltip.
	if sv.git.Branch != "" && !sv.place.hideGit {
		sections = append(sections, branchSection(th, sv))
	}
	if !sv.place.hideGit {
		sections = append(sections, baseSections(th, sv.base)...)
	}
	sections = append(sections, scheduleTipSections(th, sv, time.Now())...)
	if sv.running != "" || sv.aged {
		sections = append(sections, footLine(th, sv))
	}
	return func(gtx C) D { return stack(gtx, 1.5, sections) }
}

// para lets a label wrap instead of truncating.
func para(l ui.LabelStyle) ui.LabelStyle {
	l.MaxLines = 0
	return l
}

// stack lays widgets out top to bottom, gap apart (flex-col gap-N).
func stack(gtx C, gap float32, ws []layout.Widget) D {
	children := make([]layout.FlexChild, 0, 2*len(ws))
	for i, w := range ws {
		if i > 0 {
			children = append(children, ui.VGap(gap))
		}
		children = append(children, layout.Rigid(w))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// iconLine is a glyph and what follows it, gap-1.5 apart.
func iconLine(gtx C, ic *icons.Icon, glyph ui.LabelStyle, rest ...layout.FlexChild) D {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx C) D { return ic.Layout(gtx, 12, glyph.Color) }),
		ui.Gap(1.5),
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, append(children, rest...)...)
}

// footLine is the tooltip's last line: the model the provider runs and, in
// the corner, how long the status has lasted while it has a clock. The
// tooltip is redrawn when that reading changes.
func footLine(th *ui.Theme, sv *sessionView) layout.Widget {
	return func(gtx C) D {
		model := func(gtx C) D { return D{Size: gtx.Constraints.Min} }
		if sv.running != "" {
			model = modelSection(th, sv.Model, sv.running)
		}
		children := []layout.FlexChild{layout.Flexed(1, model)}
		if sv.aged {
			children = append(children, ui.Gap(1.5), layout.Rigid(th.Text(ui.TextXS, formatAge(sv.age)).Muted().Layout))
			gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(nextAgeChange(sv.age))})
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	}
}

// sandboxSection names what a confined session's sandbox left out of its home
// for being a symlink.
func sandboxSection(th *ui.Theme, s sandboxEvent) layout.Widget {
	line := th.Text(ui.TextXS, i18n.T("sidebar.sessionTooltip.notMounted", i18n.Args{"paths": strings.Join(s.SkippedLinks, ", ")})).Muted()
	return func(gtx C) D { return iconLine(gtx, icSandboxed, line, layout.Flexed(1, para(line).Layout)) }
}

func branchSection(th *ui.Theme, sv *sessionView) layout.Widget {
	return func(gtx C) D {
		muted := th.Text(ui.TextXS, sv.git.Branch).Muted()
		children := []layout.FlexChild{
			layout.Rigid(func(gtx C) D { return iconLine(gtx, icBranch, muted, layout.Rigid(muted.Layout)) }),
		}
		if sv.pr != nil {
			number := th.Text(ui.TextXS, "#"+strconv.Itoa(sv.pr.Number)).Muted()
			children = append(children, ui.Gap(2), layout.Rigid(func(gtx C) D {
				return iconLine(gtx, icPR, number, layout.Rigid(number.Layout))
			}))
		}
		if sv.git.Files > 0 {
			children = append(children, ui.Gap(2), layout.Rigid(func(gtx C) D { return th.DiffStat(sv.git.Added, sv.git.Deleted).Layout(gtx) }))
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	}
}

// baseSections spell out the base readout: commits behind the
// named base, then the conflict, then the first files it is in.
func baseSections(th *ui.Theme, b *backend.BaseStatus) []layout.Widget {
	if b == nil {
		return nil
	}
	var out []layout.Widget
	if b.Behind > 0 {
		out = append(out, para(th.Text(ui.TextXS, i18n.T("git.baseStatus.behind", i18n.Args{"count": b.Behind, "base": b.Base})).Muted()).Layout)
	}
	if n := len(b.Conflicts); n > 0 {
		line := th.Text(ui.TextXS, i18n.T("git.baseStatus.conflict", i18n.Args{"count": n})).In(th.ToneWait)
		out = append(out,
			func(gtx C) D { return iconLine(gtx, icConflict, line, layout.Flexed(1, line.Layout)) },
			conflictPaths(th, b.Conflicts),
		)
	}
	return out
}

// modelRun is a stretch of the model line; name marks a model name set inside
// the sentence, drawn in mono to stand apart from its words.
type modelRun struct {
	text string
	name bool
}

// modelRuns is the model line: the model running, and the one the session
// opened on when the two differ. requested is "" for a session opened on the
// provider's default, which nothing can differ from.
func modelRuns(requested, running string) []modelRun {
	if requested == "" || sameModel(requested, running) {
		return []modelRun{{text: formatModel(running)}}
	}
	names := map[string]string{"requested": formatModel(requested), "running": formatModel(running)}
	var runs []modelRun
	for i, part := range strings.Split(i18n.T("sidebar.sessionTooltip.modelChanged", i18n.Args{
		"requested": valueMarker + "requested" + valueMarker,
		"running":   valueMarker + "running" + valueMarker,
	}), valueMarker) {
		if i%2 == 1 {
			runs = append(runs, modelRun{text: names[part], name: true})
		} else if part != "" {
			runs = append(runs, modelRun{text: part})
		}
	}
	return runs
}

func modelSection(th *ui.Theme, requested, running string) layout.Widget {
	return func(gtx C) D {
		var children []layout.FlexChild
		for _, r := range modelRuns(requested, running) {
			l := th.Text(ui.TextXS, r.text).Muted()
			if r.name {
				l = l.Mono()
			}
			children = append(children, layout.Rigid(l.Layout))
		}
		return iconLine(gtx, icModel, th.Text(ui.TextXS, "").Muted(), children...)
	}
}

// sameModel is whether running is the model requested asked for: equal once
// both are shortened, or requested is the family alias ("opus") of a full id
// ("claude-opus-5-5", snapshot suffix or not). A context suffix on the alias
// ("opus[1m]") names the same model.
func sameModel(requested, running string) bool {
	requested, _, _ = strings.Cut(requested, "[")
	short := formatModel(running)
	if formatModel(requested) == short {
		return true
	}
	family, _, _ := strings.Cut(short, " ")
	return requested == family
}

func conflictPaths(th *ui.Theme, paths []string) layout.Widget {
	var rows []layout.Widget
	for _, p := range paths[:min(len(paths), namedConflicts)] {
		rows = append(rows, para(th.Text(ui.TextXS, p).Muted().Mono()).Layout)
	}
	if more := len(paths) - namedConflicts; more > 0 {
		rows = append(rows, th.Text(ui.TextXS, i18n.T("sidebar.sessionTooltip.morePaths", i18n.Args{"count": more})).Muted().Layout)
	}
	return func(gtx C) D {
		return layout.Inset{Left: ui.Space(4)}.Layout(gtx, func(gtx C) D { return stack(gtx, 0.5, rows) })
	}
}

func plural(n int, one, other string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + other
}
