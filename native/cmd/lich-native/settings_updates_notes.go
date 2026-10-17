package main

import (
	"fmt"
	"image"
	"image/color"
	"regexp"
	"strings"
	"unicode"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/i18n"
	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

const (
	notesLeadCh     = 22 // max-w-[22ch] on the headline
	notesLeadBodyCh = 56
	notesItemBodyCh = 60
	notesBodyHeight = 60 // max-h-[60vh], as a percentage of what the panel is given
	notesSpaceEm    = 0.27
	notesCodeEm     = 0.85 // text-[0.85em] on a code span
	notesCountSep   = " · "
)

var (
	notesSmall       = unit.Sp(13) // text-[0.8125rem]
	notesLeadSize    = unit.Sp(24)
	icNotesWarning   = icons.Lucide("triangle-alert")
	icNotesNote      = icons.Lucide("info")
	icNotesOpened    = icons.Lucide("chevron-down")
	icNotesClosed    = icons.Lucide("chevron-right")
	icNotesChangelog = icons.Lucide("arrow-up-right")
	notesSky         = color.NRGBA{R: 0x00, G: 0xa6, B: 0xf4, A: 0xff} // Tailwind sky-500, the "Fixed" dot
	notesGroupDot    = unit.Dp(8)
)

// inlineStyle is how a run of a changelog line is set: the markup the
// CHANGELOG uses is **bold** lead-ins and `code` spans, nothing else.
type inlineStyle uint8

const (
	runPlain inlineStyle = iota
	runBold
	runCode
)

type inlineRun struct {
	text  string
	style inlineStyle
}

var inlineMarks = regexp.MustCompile("\\*\\*[^*]+\\*\\*|`[^`]+`")

// inlineRuns is renderInline's split of text into plain, bold and code runs.
func inlineRuns(text string) []inlineRun {
	var runs []inlineRun
	last := 0
	for _, m := range inlineMarks.FindAllStringIndex(text, -1) {
		if m[0] > last {
			runs = append(runs, inlineRun{text[last:m[0]], runPlain})
		}
		part := text[m[0]:m[1]]
		if strings.HasPrefix(part, "**") {
			runs = append(runs, inlineRun{part[2 : len(part)-2], runBold})
		} else {
			runs = append(runs, inlineRun{part[1 : len(part)-1], runCode})
		}
		last = m[1]
	}
	if last < len(text) {
		runs = append(runs, inlineRun{text[last:], runPlain})
	}
	return runs
}

// inlineWords cuts the runs at whitespace into words, each a list of runs:
// `code`, stays one word, so the comma never lands on a line of its own.
func inlineWords(text string) [][]inlineRun {
	var words [][]inlineRun
	var word []inlineRun
	flush := func() {
		if word != nil {
			words = append(words, word)
			word = nil
		}
	}
	for _, run := range inlineRuns(text) {
		rest := run.text
		for rest != "" {
			switch i := strings.IndexFunc(rest, unicode.IsSpace); {
			case i < 0:
				word = append(word, inlineRun{rest, run.style})
				rest = ""
			case i == 0:
				flush()
				rest = strings.TrimLeftFunc(rest, unicode.IsSpace)
			default:
				word = append(word, inlineRun{rest[:i], run.style})
				rest = rest[i:]
			}
		}
	}
	flush()
	return words
}

// splitLeadIn is splitLeadIn of patch-notes-layout.ts: the bold sentence an
// item opens with, and the paragraph behind it.
func splitLeadIn(item string) (headline, body string) {
	if !strings.HasPrefix(item, "**") {
		return item, ""
	}
	end := strings.Index(item[2:], "**")
	if end < 0 {
		return item, ""
	}
	return strings.TrimSpace(item[2 : 2+end]), strings.TrimSpace(item[2+end+2:])
}

// layoutHighlights is layoutHighlights of patch-notes-layout.ts: the first
// important block is the headline, every other block a callout under it.
func layoutHighlights(highlights []lichclient.PatchNotesHighlight) (lead *lichclient.PatchNotesHighlight, callouts []lichclient.PatchNotesHighlight) {
	for i := range highlights {
		if lead == nil && highlights[i].Kind == "important" {
			lead = &highlights[i]
			continue
		}
		callouts = append(callouts, highlights[i])
	}
	return lead, callouts
}

// noteDot is dotColor: the group's semantic accent, apart from the app's.
func noteDot(th *ui.Theme, label string) color.NRGBA {
	switch strings.ToLower(label) {
	case "added":
		return th.TonePass
	case "changed":
		return th.ToneWait
	case "fixed":
		return notesSky
	}
	return th.MutedForeground
}

// richStyle is the paragraph a changelog line is set in.
type richStyle struct {
	size, leading unit.Sp
	ink           color.NRGBA
	weight        font.Weight
}

// richText sets text as words that wrap, each run in its own face.
func richText(th *ui.Theme, text string, st richStyle) layout.Widget {
	words := inlineWords(text)
	items := make([]ui.WrapChild, len(words))
	for i, word := range words {
		items[i] = ui.WrapItem(func(gtx C) D {
			children := make([]layout.FlexChild, len(word))
			for j, run := range word {
				children[j] = layout.Rigid(st.run(th, run))
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
		})
	}
	return func(gtx C) D {
		return ui.Wrap{GapX: unit.Dp(float32(st.size) * notesSpaceEm)}.Layout(gtx, items...)
	}
}

func (st richStyle) run(th *ui.Theme, r inlineRun) layout.Widget {
	switch r.style {
	case runBold:
		return th.Text(st.size, r.text).Leading(st.leading).In(th.Foreground).Weight(font.Medium).Layout
	case runCode:
		size := unit.Sp(float32(st.size) * notesCodeEm)
		l := th.Text(size, r.text).Leading(size * 1.4).In(st.ink).Mono()
		return func(gtx C) D {
			return layout.Background{}.Layout(gtx,
				func(gtx C) D { return ui.Fill(gtx, th.Over(th.Accent), unit.Dp(4)) },
				func(gtx C) D { return layout.Inset{Left: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, l.Layout) },
			)
		}
	}
	return th.Text(st.size, r.text).Leading(st.leading).In(st.ink).Weight(st.weight).Layout
}

// cappedBy is w no wider than ch characters of size, a ch taken as 0.6em as
// the pane's descriptions are.
func cappedBy(w layout.Widget, size unit.Sp, ch int) layout.Widget {
	return func(gtx C) D {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Sp(size)*ch*6/10)
		return w(gtx)
	}
}

type notesItemKey struct{ group, item int }

// patchNotesView is the What's new dialog's state: its widgets and which
// items are open.
type patchNotesView struct {
	open bool
	// startup is the popup PatchNotesGate opened at start: it shows with
	// Settings closed, and closing it records the version as seen.
	startup          bool
	dialog           ui.DialogState
	list             widget.List
	gotIt, changelog widget.Clickable
	rows             map[notesItemKey]*widget.Clickable
	expanded         map[notesItemKey]bool
}

func (v *patchNotesView) show() { v.open, v.expanded = true, nil }

func (v *patchNotesView) row(k notesItemKey) *widget.Clickable {
	if v.rows == nil {
		v.rows = map[notesItemKey]*widget.Clickable{}
	}
	if v.rows[k] == nil {
		v.rows[k] = new(widget.Clickable)
	}
	return v.rows[k]
}

// layoutPatchNotesDialog is PatchNotesDialog over the window, with the
// window's constraints as every dialog.
func (s *settingsScreen) layoutPatchNotesDialog(gtx C, th *ui.Theme) {
	v := &s.updates.patch
	s.takeStartupNotes()
	notes := s.updates.patchNotes()
	if notes == nil {
		return
	}
	if v.gotIt.Clicked(gtx) {
		s.closePatchNotes(notes.Version)
	}
	if v.changelog.Clicked(gtx) {
		s.openExternal(helpRepoURL + updatesReleaseTagAt + notes.Version)
	}
	on := th.On(th.Popover)
	got := on.Button(&v.gotIt, i18n.T("shell.patchNotesDialog.gotIt"))
	got.Size = ui.SizeSM
	dlg := th.Dialog(&v.dialog, v.open && (s.open || v.startup), i18n.T("shell.patchNotesDialog.title", i18n.Args{"version": "v" + notes.Version}), notesCounts(notes.Groups))
	dlg.MaxWidth, dlg.HideClose = ui.DialogWidthXL, true
	dlg.Body = func(gtx C) D { return v.body(gtx, on, notes) }
	dlg.Actions = []layout.Widget{func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return v.changelogLink(gtx, on) }),
			layout.Rigid(got.Layout),
		)
	}}
	if dlg.Layout(gtx) {
		s.closePatchNotes(notes.Version)
	}
}

func (s *settingsScreen) takeStartupNotes() {
	u := &s.updates
	u.mu.Lock()
	take := u.startupNotes
	u.startupNotes = false
	u.mu.Unlock()
	if take {
		u.patch.show()
		u.patch.startup = true
	}
}

func (s *settingsScreen) closePatchNotes(version string) {
	v := &s.updates.patch
	if v.startup {
		s.recordPatchNotes(version)
	}
	v.open, v.startup = false, false
}

// changelogLink is the muted "View full changelog ↗" that goes foreground on
// hover.
func (v *patchNotesView) changelogLink(gtx C, th *ui.Theme) D {
	return v.changelog.Layout(gtx, func(gtx C) D {
		ink := th.MutedForeground
		if v.changelog.Hovered() {
			ink = th.Foreground
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(th.Text(notesSmall, i18n.T("shell.patchNotesDialog.viewChangelog")).In(ink).Layout),
			ui.Gap(1),
			layout.Rigid(func(gtx C) D { return icNotesChangelog.Layout(gtx, unit.Dp(14), ink) }),
		)
	})
}

// body is the release: the counts, the headline, the callouts and the groups,
// in a scroll area capped at the share of the panel the web gives it.
func (v *patchNotesView) body(gtx C, th *ui.Theme, notes *lichclient.PatchNotes) D {
	var rows []layout.Widget
	lead, callouts := layoutHighlights(notes.Highlights)
	if lead != nil {
		rows = append(rows, notesLead(th, *lead))
	}
	for _, h := range callouts {
		rows = append(rows, notesCallout(th, h))
	}
	for g, group := range notes.Groups {
		rows = append(rows, notesGroupHeading(th, group.Label, g > 0 || len(callouts) > 0))
		for i, item := range group.Items {
			rows = append(rows, v.item(th, notesItemKey{g, i}, item))
		}
	}
	v.list.Axis = layout.Vertical
	gtx.Constraints.Max.Y = gtx.Constraints.Max.Y * notesBodyHeight / 100
	return th.ScrollArea(&v.list).Layout(gtx, len(rows), func(gtx C, i int) D { return rows[i](gtx) })
}

// notesCounts is the line of "3 added" counts the web's header ends on.
func notesCounts(groups []lichclient.PatchNotesGroup) string {
	parts := make([]string, len(groups))
	for i, g := range groups {
		parts[i] = notesGroupCount(g.Label, len(g.Items))
	}
	return strings.Join(parts, notesCountSep)
}

func notesDotOf(gtx C, c color.NRGBA, size unit.Dp) D {
	px := gtx.Dp(size)
	gtx.Constraints = layout.Exact(image.Pt(px, px))
	return ui.Fill(gtx, c, size/2)
}

// notesLead is the headline: the largest type the app uses and nothing else.
func notesLead(th *ui.Theme, h lichclient.PatchNotesHighlight) layout.Widget {
	headline, body := splitLeadIn(h.Text)
	head := richText(th, headline, richStyle{size: notesLeadSize, leading: notesLeadSize * 5 / 4, ink: th.Foreground, weight: font.SemiBold})
	children := []layout.FlexChild{layout.Rigid(cappedBy(head, notesLeadSize, notesLeadCh))}
	if body != "" {
		text := richText(th, body, richStyle{size: ui.TextSM, leading: 23, ink: th.MutedForeground})
		children = append(children, ui.VGap(2.5), layout.Rigid(cappedBy(text, ui.TextSM, notesLeadBodyCh)))
	}
	return func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...) }),
			ui.VGap(4),
			layout.Rigid(th.Separator),
			ui.VGap(4),
		)
	}
}

// notesCallout is a block under the headline: a warning paints in the
// destructive tint, a note sits on the accent fill.
func notesCallout(th *ui.Theme, h lichclient.PatchNotesHighlight) layout.Widget {
	warning := h.Kind == "warning"
	icon, iconInk, fill := icNotesNote, th.MutedForeground, th.Over(ui.Alpha(th.Accent, 0.6))
	if warning {
		icon, iconInk, fill = icNotesWarning, th.Destructive, th.Over(ui.Alpha(th.Destructive, 0.1))
	}
	text := richText(th, h.Text, richStyle{size: notesSmall, leading: 21, ink: th.MutedForeground})
	return func(gtx C) D {
		return layout.Inset{Bottom: ui.Space(2)}.Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			macro := op.Record(gtx.Ops)
			d := layout.Inset{Left: ui.Space(3), Right: ui.Space(3), Top: ui.Space(2.5), Bottom: ui.Space(2.5)}.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Start}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{Top: ui.Space(0.5)}.Layout(gtx, func(gtx C) D { return icon.Layout(gtx, ui.Space(4), iconInk) })
					}),
					ui.Gap(2.5),
					layout.Flexed(1, text),
				)
			})
			call := macro.Stop()
			bg := gtx
			bg.Constraints = layout.Exact(d.Size)
			ui.Fill(bg, fill, ui.RadiusMD)
			if warning {
				insetBorder(gtx, d.Size, gtx.Dp(ui.RadiusMD), th.Over(ui.Alpha(th.Destructive, 0.3)))
			}
			call.Add(gtx.Ops)
			return d
		})
	}
}

// notesGroupLabel is a group's heading in the interface language. The
// changelog keeps Keep a Changelog's six headings in English in every
// translation; any other heading is shown as written.
func notesGroupLabel(label string) string {
	switch strings.ToLower(label) {
	case "added":
		return i18n.T("shell.patchNotesDialog.groups.added")
	case "changed":
		return i18n.T("shell.patchNotesDialog.groups.changed")
	case "deprecated":
		return i18n.T("shell.patchNotesDialog.groups.deprecated")
	case "removed":
		return i18n.T("shell.patchNotesDialog.groups.removed")
	case "fixed":
		return i18n.T("shell.patchNotesDialog.groups.fixed")
	case "security":
		return i18n.T("shell.patchNotesDialog.groups.security")
	}
	return label
}

// notesGroupCount is notesGroupLabel's "3 added", for the header.
func notesGroupCount(label string, items int) string {
	count := i18n.Args{"count": items}
	switch strings.ToLower(label) {
	case "added":
		return i18n.T("shell.patchNotesDialog.counts.added", count)
	case "changed":
		return i18n.T("shell.patchNotesDialog.counts.changed", count)
	case "deprecated":
		return i18n.T("shell.patchNotesDialog.counts.deprecated", count)
	case "removed":
		return i18n.T("shell.patchNotesDialog.counts.removed", count)
	case "fixed":
		return i18n.T("shell.patchNotesDialog.counts.fixed", count)
	case "security":
		return i18n.T("shell.patchNotesDialog.counts.security", count)
	}
	return fmt.Sprintf("%d %s", items, strings.ToLower(label))
}

// notesGroupHeading is a group's divider and its dot and label.
func notesGroupHeading(th *ui.Theme, label string, divider bool) layout.Widget {
	return func(gtx C) D {
		children := []layout.FlexChild{}
		if divider {
			children = append(children, ui.VGap(1), layout.Rigid(th.Separator))
		}
		children = append(children, ui.VGap(3.5), layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return notesDotOf(gtx, noteDot(th, label), notesGroupDot) }),
				ui.Gap(1.5),
				layout.Rigid(th.Text(ui.TextXS, strings.ToUpper(notesGroupLabel(label))).Muted().Weight(font.SemiBold).Layout),
			)
		}), ui.VGap(1.5))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}

// item is one changelog entry: its bold lead-in, with the paragraph behind
// it opening on click. An entry without one is its own row.
func (v *patchNotesView) item(th *ui.Theme, k notesItemKey, text string) layout.Widget {
	headline, body := splitLeadIn(text)
	title := richText(th, headline, richStyle{size: notesSmall, leading: 18, ink: th.Foreground, weight: font.Medium})
	chevronSlot := unit.Dp(14)
	line := func(gtx C, chevron *icons.Icon) D {
		return layout.Inset{Top: ui.Space(1.5), Bottom: ui.Space(1.5)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Alignment: layout.Start}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					if chevron == nil {
						return D{Size: image.Pt(gtx.Dp(chevronSlot), 0)}
					}
					return layout.Inset{Top: ui.Space(0.5)}.Layout(gtx, func(gtx C) D { return chevron.Layout(gtx, chevronSlot, th.MutedForeground) })
				}),
				ui.Gap(2),
				layout.Flexed(1, title),
			)
		})
	}
	if body == "" {
		return func(gtx C) D { return line(gtx, nil) }
	}
	click := v.row(k)
	paragraph := cappedBy(richText(th, body, richStyle{size: notesSmall, leading: 21, ink: th.MutedForeground}), notesSmall, notesItemBodyCh)
	return func(gtx C) D {
		if click.Clicked(gtx) {
			if v.expanded == nil {
				v.expanded = map[notesItemKey]bool{}
			}
			v.expanded[k] = !v.expanded[k]
		}
		open := v.expanded[k]
		chevron := icNotesClosed
		if open {
			chevron = icNotesOpened
		}
		children := []layout.FlexChild{layout.Rigid(func(gtx C) D {
			return click.Layout(gtx, func(gtx C) D {
				macro := op.Record(gtx.Ops)
				d := line(gtx, chevron)
				call := macro.Stop()
				if click.Hovered() {
					hover := gtx
					hover.Constraints = layout.Exact(d.Size)
					ui.Fill(hover, th.Over(ui.Alpha(th.Accent, ghostHoverFill)), ui.RadiusMD)
				}
				call.Add(gtx.Ops)
				return d
			})
		})}
		if open {
			children = append(children, layout.Rigid(func(gtx C) D {
				return layout.Inset{Left: ui.Space(5.5), Bottom: ui.Space(2)}.Layout(gtx, paragraph)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}
