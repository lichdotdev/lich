package main

import (
	"image/color"
	"regexp"
	"strings"
	"unicode"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

const (
	notesLeadCh     = 22 // max-w-[22ch] on the headline
	notesLeadBodyCh = 56
	notesBodyHeight = 60 // max-h-[60vh], as a percentage of what the panel is given
	notesSpaceEm    = 0.27
	notesCodeEm     = 0.85 // text-[0.85em] on a code span
)

var (
	notesLeadSize    = unit.Sp(24)
	icNotesWarning   = icons.Lucide("triangle-alert")
	icNotesCaution   = icons.Lucide("octagon-alert")
	icNotesNote      = icons.Lucide("info")
	icNotesChangelog = icons.Lucide("arrow-up-right")
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

// splitLeadIn splits an item into the bold sentence it opens with and the
// paragraph behind it. An item with no bold opening gives its first sentence.
func splitLeadIn(item string) (headline, body string) {
	if !strings.HasPrefix(item, "**") {
		return splitFirstSentence(item)
	}
	end := strings.Index(item[2:], "**")
	if end < 0 {
		return item, ""
	}
	return strings.TrimSpace(item[2 : 2+end]), strings.TrimSpace(item[2+end+2:])
}

// splitFirstSentence cuts at the first full stop followed by a space that is
// outside a code span, where a file name carries one.
func splitFirstSentence(item string) (headline, body string) {
	inCode := false
	for i := 0; i < len(item)-1; i++ {
		switch {
		case item[i] == '`':
			inCode = !inCode
		case item[i] == '.' && item[i+1] == ' ' && !inCode:
			return item[:i+1], strings.TrimSpace(item[i+1:])
		}
	}
	return item, ""
}

// layoutHighlights makes the first important block the headline and every
// other block a callout under it.
func layoutHighlights(highlights []backend.PatchNotesHighlight) (lead *backend.PatchNotesHighlight, callouts []backend.PatchNotesHighlight) {
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

// patchNotesView is the What's new dialog's state.
type patchNotesView struct {
	open bool
	// startup is the popup PatchNotesGate opened at start: it shows with
	// Settings closed, and closing it records the version as seen.
	startup          bool
	dialog           ui.DialogState
	list             widget.List
	gotIt, changelog widget.Clickable
}

func (v *patchNotesView) show() { v.open = true }

// layoutPatchNotesDialog is PatchNotesDialog over the window, with the
// window's constraints as every dialog.
func (s *settingsScreen) layoutPatchNotesDialog(gtx C, th *ui.Theme) {
	v := &s.updates.patch
	s.takeStartupNotes()
	notes := s.updates.patchNotes()
	if notes == nil {
		return
	}
	if !(v.open && (s.open || v.startup)) && !v.dialog.Animating(gtx) {
		return
	}
	if v.gotIt.Clicked(gtx) {
		s.closePatchNotes(notes.Version)
	}
	if v.changelog.Clicked(gtx) {
		s.openExternal(helpRepoURL + updatesReleaseTagAt + notes.Version)
	}
	open := v.open && (s.open || v.startup)
	on := th.On(th.Popover)
	got := on.Button(&v.gotIt, i18n.T("shell.patchNotesDialog.gotIt"))
	got.Size = ui.SizeSM
	dlg := th.Dialog(&v.dialog, open, i18n.T("shell.patchNotesDialog.title", i18n.Args{"version": "v" + notes.Version}), "")
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
			layout.Rigid(th.Text(settingsDescSize, i18n.T("shell.patchNotesDialog.viewChangelog")).In(ink).Layout),
			ui.Gap(1),
			layout.Rigid(func(gtx C) D { return icNotesChangelog.Layout(gtx, unit.Dp(14), ink) }),
		)
	})
}

// body is the release: the headline, the callouts and the groups,
// in a scroll area capped at a share of the panel.
func (v *patchNotesView) body(gtx C, th *ui.Theme, notes *backend.PatchNotes) D {
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
		for _, item := range group.Items {
			rows = append(rows, notesItem(th, item))
		}
	}
	v.list.Axis = layout.Vertical
	gtx.Constraints.Max.Y = gtx.Constraints.Max.Y * notesBodyHeight / 100
	return th.ScrollArea(&v.list).Layout(gtx, len(rows), func(gtx C, i int) D { return rows[i](gtx) })
}

// notesLead is the headline: the largest type the app uses and nothing else.
func notesLead(th *ui.Theme, h backend.PatchNotesHighlight) layout.Widget {
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
// calloutTone is a callout's glyph and ink, and whether it is drawn as danger:
// a warning and a caution, the block of a required update, are; a note is not.
func calloutTone(th *ui.Theme, kind string) (icon *icons.Icon, ink color.NRGBA, danger bool) {
	switch kind {
	case "caution":
		return icNotesCaution, th.Destructive, true
	case "warning":
		return icNotesWarning, th.Destructive, true
	}
	return icNotesNote, th.MutedForeground, false
}

func notesCallout(th *ui.Theme, h backend.PatchNotesHighlight) layout.Widget {
	icon, iconInk, danger := calloutTone(th, h.Kind)
	fill := th.Over(ui.Alpha(th.Accent, 0.6))
	if danger {
		fill = th.Over(ui.Alpha(th.Destructive, 0.1))
	}
	text := richText(th, h.Text, richStyle{size: settingsDescSize, leading: 21, ink: th.MutedForeground})
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
			if danger {
				ui.InsetBorder(gtx, d.Size, gtx.Dp(ui.RadiusMD), th.Over(ui.Alpha(th.Destructive, 0.3)))
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

// notesGroupHeading is a group's dot and label, set off from what is above it.
func notesGroupHeading(th *ui.Theme, label string, gap bool) layout.Widget {
	heading := th.SectionHeading(notesGroupLabel(label))
	heading.Weight = font.SemiBold
	return func(gtx C) D {
		children := []layout.FlexChild{}
		if gap {
			children = append(children, ui.VGap(4.5))
		}
		children = append(children, layout.Rigid(func(gtx C) D {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return ui.Dot(gtx, noteDot(th, label), notesGroupDot) }),
				ui.Gap(1.5),
				layout.Rigid(heading.Layout),
			)
		}), ui.VGap(1))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}

// notesItem is one changelog entry: its opening sentence as the title and the
// paragraph behind it, muted, under it.
func notesItem(th *ui.Theme, text string) layout.Widget {
	headline, body := splitLeadIn(text)
	children := []layout.FlexChild{
		layout.Rigid(richText(th, headline, richStyle{size: settingsDescSize, leading: 18, ink: th.Foreground, weight: font.Medium})),
	}
	if body != "" {
		children = append(children, ui.VGap(0.5),
			layout.Rigid(richText(th, body, richStyle{size: settingsDescSize, leading: 20, ink: th.MutedForeground})))
	}
	return func(gtx C) D {
		return layout.Inset{Top: ui.Space(1.75), Bottom: ui.Space(1.75)}.Layout(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	}
}
