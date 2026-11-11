package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// scheduleAtTime is the dialog's kind for a prompt due at a time; every other
// kind is an api.Watch condition.
const scheduleAtTime = "time"

// scheduleClockLayout is how a parked time reads back in the When field, one of
// the forms api.ParseWhen takes.
const scheduleClockLayout = "2006-01-02 15:04"

var (
	icSchedule      = icons.Lucide("clock")
	icWatch         = icons.Lucide("radar")
	icChecksFailed  = icons.Lucide("circle-x")
	icChecksPassed  = icons.Lucide("circle-check")
	icReviewArrives = icons.Lucide("message-square-text")
	schedulePromptH = ui.Space(40)
	// scheduleSlotH is the When field's height with its label and echo, kept by
	// the line that replaces it, so the centred dialog does not move under the
	// pointer when the trigger changes.
	scheduleSlotH = ui.Space(19.5)
)

var scheduleKinds = []string{scheduleAtTime, api.WatchChecksFailed, api.WatchChecksPassed, api.WatchReview}

// scheduleDialog is the card's Schedule prompt…: one session, one trigger.
type scheduleDialog struct {
	id, label, branch, kind, by string
	number, runs                int
	open, focus, armed          bool
	dialog                      ui.DialogState
	when, prompt                widget.Editor
	options                     [4]widget.Clickable
	cancel, save, remove        widget.Clickable
}

// show opens the dialog on what sv holds: its watch when it has one, as the
// tooltip's first line says, else its parked prompt.
func (d *scheduleDialog) show(sv sessionView) {
	d.id, d.label, d.branch, d.number = sv.ID, sv.Label, sv.git.Branch, 0
	if sv.pr != nil {
		d.number = sv.pr.Number
	}
	d.kind, d.by, d.runs, d.armed = scheduleAtTime, "", 0, true
	d.when.SetText("")
	d.prompt.SetText("")
	switch {
	case sv.Watch.On != "":
		d.kind, d.by, d.runs = sv.Watch.On, sv.Watch.By, sv.Watch.Runs
		d.prompt.SetText(sv.Watch.Prompt)
	case sv.ScheduledPrompt != "":
		d.by = sv.ScheduledBy
		d.when.SetText(time.Unix(sv.ScheduledAt, 0).Format(scheduleClockLayout))
		d.prompt.SetText(sv.ScheduledPrompt)
	default:
		d.armed = false
	}
	d.open, d.focus = true, true
}

// pick moves to kind, swapping the prompt for kind's own unless the person has
// written something else.
func (d *scheduleDialog) pick(kind string) {
	if strings.TrimSpace(d.prompt.Text()) == schedulePromptFor(d.kind) {
		d.prompt.SetText(schedulePromptFor(kind))
	}
	d.kind, d.focus = kind, true
}

// schedulePromptFor is the prompt a condition starts with, "" for one where
// only the person knows what comes next.
func schedulePromptFor(kind string) string {
	switch kind {
	case api.WatchChecksFailed:
		return i18n.Prompt("prompts.schedule.checksFailed")
	case api.WatchReview:
		return i18n.Prompt("prompts.schedule.review")
	}
	return ""
}

func scheduleKindLabel(kind string) string {
	switch kind {
	case api.WatchChecksFailed:
		return i18n.T("sidebar.scheduleDialog.checksFailed")
	case api.WatchChecksPassed:
		return i18n.T("sidebar.scheduleDialog.checksPassed")
	case api.WatchReview:
		return i18n.T("sidebar.scheduleDialog.review")
	}
	return i18n.T("sidebar.scheduleDialog.atTime")
}

func scheduleKindIcon(kind string) *icons.Icon {
	switch kind {
	case api.WatchChecksFailed:
		return icChecksFailed
	case api.WatchChecksPassed:
		return icChecksPassed
	case api.WatchReview:
		return icReviewArrives
	}
	return icSchedule
}

func scheduleKindNote(kind string) string {
	if kind == api.WatchChecksFailed {
		return i18n.T("sidebar.scheduleDialog.staysOn", i18n.Args{"limit": api.WatchRunLimit})
	}
	return i18n.T("sidebar.scheduleDialog.runsOnce")
}

// sendOptions is what the dialog's Schedule sends, false while it cannot be sent.
func (d *scheduleDialog) sendOptions(now time.Time) (api.SetScheduleOptions, bool) {
	opts := api.SetScheduleOptions{SessionID: d.id, Prompt: strings.TrimSpace(d.prompt.Text())}
	if d.kind != scheduleAtTime {
		opts.On = d.kind
		return opts, opts.Prompt != ""
	}
	at, ok := api.ParseWhen(d.when.Text(), now)
	opts.At = at.Unix()
	return opts, ok && opts.Prompt != ""
}

// layout draws the dialog. Lay it out before the content, so it claims Escape
// ahead of anything under it.
func (d *scheduleDialog) layout(ctx context.Context, gtx C, th *ui.Theme, client *backend.Client, notify func(ui.Toast)) {
	if !d.open && !d.dialog.Animating(gtx) {
		return
	}
	opts, ok := d.sendOptions(time.Now())
	send := func(o api.SetScheduleOptions) {
		d.open = false
		go func() {
			if err := client.SetSchedule(ctx, o); err != nil {
				notify(ui.Toast{Variant: ui.ToastError, Title: i18n.T("sidebar.scheduleDialog.failed"), Description: err.Error()})
			}
		}()
	}
	if d.save.Clicked(gtx) && ok && d.open {
		send(opts)
	}
	if d.remove.Clicked(gtx) && d.open {
		send(api.SetScheduleOptions{SessionID: d.id})
	}
	if d.cancel.Clicked(gtx) {
		d.open = false
	}
	for i, kind := range scheduleKinds {
		if d.options[i].Clicked(gtx) {
			d.pick(kind)
		}
	}
	on := th.On(th.Popover)
	title := i18n.T("sidebar.scheduleDialog.title")
	if d.armed {
		title = i18n.T("sidebar.scheduleDialog.titleArmed")
	}
	dialog := th.Dialog(&d.dialog, d.open, title, i18n.T("sidebar.scheduleDialog.description", i18n.Args{"label": d.label}))
	dialog.Body = func(gtx C) D { return d.body(gtx, on) }
	dialog.Actions = d.actions(on, ok)
	if dialog.Layout(gtx) {
		d.open = false
	}
	// After the dialog, which takes the focus for its panel the frame it opens.
	if d.focus && d.open {
		tag := &d.prompt
		if d.kind == scheduleAtTime {
			tag = &d.when
		}
		gtx.Execute(key.FocusCmd{Tag: tag})
		d.focus = false
	}
}

func (d *scheduleDialog) actions(th *ui.Theme, ok bool) []layout.Widget {
	var actions []layout.Widget
	if d.armed {
		remove := th.Button(&d.remove, i18n.T("sidebar.scheduleDialog.remove"))
		remove.Variant, remove.Size = ui.VariantDestructive, ui.SizeSM
		actions = append(actions, remove.Layout)
	}
	cancel := th.Button(&d.cancel, i18n.T("common.action.cancel"))
	cancel.Variant, cancel.Size = ui.VariantGhost, ui.SizeSM
	label := i18n.T("sidebar.scheduleDialog.schedule")
	if d.armed {
		label = i18n.T("sidebar.scheduleDialog.save")
	}
	save := th.Button(&d.save, label)
	save.Size, save.Disabled = ui.SizeSM, !ok
	return append(actions, cancel.Layout, save.Layout)
}

func (d *scheduleDialog) body(gtx C, th *ui.Theme) D {
	children := []layout.FlexChild{layout.Rigid(func(gtx C) D { return d.kindList(gtx, th) }), ui.VGap(4)}
	if d.kind == scheduleAtTime {
		children = append(children, layout.Rigid(func(gtx C) D { return d.whenField(gtx, th) }))
	} else {
		children = append(children, layout.Rigid(func(gtx C) D {
			line := d.pullRequestLine(gtx, th)
			line.Size.Y = max(line.Size.Y, gtx.Dp(scheduleSlotH))
			return line
		}))
	}
	children = append(children, ui.VGap(4), layout.Rigid(func(gtx C) D { return d.promptField(gtx, th) }))
	if who := d.setByLine(); who != "" {
		children = append(children, ui.VGap(4), layout.Rigid(para(th.Text(ui.TextXS, who).Muted()).Layout))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (d *scheduleDialog) kindList(gtx C, th *ui.Theme) D {
	rows := make([]layout.FlexChild, 0, 2*len(scheduleKinds))
	for i, kind := range scheduleKinds {
		if i > 0 {
			rows = append(rows, ui.VGap(0.5))
		}
		rows = append(rows, layout.Rigid(func(gtx C) D {
			return th.NavRow(gtx, &d.options[i], d.kind == kind, navRowHeight, navRowPadX, func(gtx C, th *ui.Theme) D {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx C) D { return scheduleKindIcon(kind).Layout(gtx, ui.Space(3.5), th.MutedForeground) }),
					ui.Gap(2),
					layout.Flexed(1, th.Text(ui.TextSM, scheduleKindLabel(kind)).Layout),
					layout.Rigid(d.kindHint(th, kind)),
				)
			})
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}

func (d *scheduleDialog) kindHint(th *ui.Theme, kind string) layout.Widget {
	switch kind {
	case api.WatchChecksFailed:
		return th.Text(ui.TextXS, i18n.T("sidebar.scheduleDialog.keepsWatching")).Muted().Mono().Layout
	case api.WatchChecksPassed, api.WatchReview:
		return th.Text(ui.TextXS, i18n.T("sidebar.scheduleDialog.once")).Muted().Mono().Layout
	}
	return func(C) D { return D{} }
}

func (d *scheduleDialog) whenField(gtx C, th *ui.Theme) D {
	d.when.SingleLine = true
	input := th.Input(&d.when, i18n.T("sidebar.scheduleDialog.whenPlaceholder"))
	input.Mono = true
	echo, ok := scheduleEcho(d.when.Text(), time.Now())
	line := th.Text(ui.TextXS, echo).Muted()
	if !ok {
		line = line.In(th.Destructive)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(th.Label(i18n.T("sidebar.scheduleDialog.when")).Layout),
		ui.VGap(1.5),
		layout.Rigid(input.Layout),
		ui.VGap(1.5),
		layout.Rigid(line.Layout),
	)
}

// scheduleEcho is the line under When: the moment what was typed lands on, or
// how to write one. An empty field is not yet a mistake.
func scheduleEcho(typed string, now time.Time) (string, bool) {
	if strings.TrimSpace(typed) == "" {
		return " ", true
	}
	at, ok := api.ParseWhen(typed, now)
	if !ok {
		return i18n.T("sidebar.scheduleDialog.whenError"), false
	}
	args := i18n.Args{"clock": at.Format("15:04"), "left": scheduleLeft(at.Sub(now)), "date": at.Format("2006-01-02")}
	switch at.YearDay() - now.YearDay() {
	case 0:
		if at.Year() == now.Year() {
			return i18n.T("sidebar.scheduleDialog.landsToday", args), true
		}
	case 1:
		if at.Year() == now.Year() {
			return i18n.T("sidebar.scheduleDialog.landsTomorrow", args), true
		}
	}
	return i18n.T("sidebar.scheduleDialog.landsOn", args), true
}

// scheduleLeft is the time until a moment in the units lateBy speaks: "45m",
// "2h 30m", "3d". Two days out, the minutes say nothing.
func scheduleLeft(d time.Duration) string {
	minutes := int(d.Round(time.Minute) / time.Minute)
	hours := minutes / 60
	switch {
	case hours >= 48:
		return fmt.Sprintf("%dd", (hours+12)/24)
	case hours > 0 && minutes%60 > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes%60)
	case hours > 0:
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dm", minutes)
}

func (d *scheduleDialog) pullRequestLine(gtx C, th *ui.Theme) D {
	if d.number == 0 {
		return para(th.Text(ui.TextXS, i18n.T("sidebar.scheduleDialog.noPR")).Muted()).Layout(gtx)
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return icMenuPR.Layout(gtx, ui.Space(3), th.MutedForeground) }),
		ui.Gap(1.5),
		layout.Rigid(th.Text(ui.TextXS, fmt.Sprintf("#%d", d.number)).Muted().Mono().Layout),
		ui.Gap(1.5),
		layout.Flexed(1, th.Text(ui.TextXS, d.branch).Muted().Layout),
	)
}

func (d *scheduleDialog) promptField(gtx C, th *ui.Theme) D {
	area := th.Textarea(&d.prompt, i18n.T("sidebar.scheduleDialog.promptPlaceholder"))
	area.MaxHeight = schedulePromptH
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(th.Label(i18n.T("sidebar.scheduleDialog.prompt")).Layout),
		ui.VGap(1.5),
		layout.Rigid(area.Layout),
		ui.VGap(1.5),
		layout.Rigid(para(th.Text(ui.TextXS, scheduleKindNote(d.kind)).Muted()).Layout),
	)
}

// setByLine is who set what the dialog opened on, and how far a failing-checks
// watch has run, "" for a new one.
func (d *scheduleDialog) setByLine() string {
	if !d.armed {
		return ""
	}
	var parts []string
	switch d.by {
	case api.ScheduledByAgent:
		parts = append(parts, i18n.T("sidebar.scheduleDialog.setByAgent"))
	case api.ScheduledByLich:
		parts = append(parts, i18n.T("sidebar.scheduleDialog.setByLich"))
	}
	if d.kind == api.WatchChecksFailed {
		parts = append(parts, i18n.T("sidebar.scheduleDialog.runsUsed", i18n.Args{"runs": d.runs, "limit": api.WatchRunLimit}))
	}
	return strings.Join(parts, " ")
}

// scheduleMark is the glyph a row earns for what it holds, nil for nothing.
func scheduleMark(sv *sessionView) *icons.Icon {
	switch {
	case sv.Watch.On != "":
		return icWatch
	case sv.ScheduledPrompt != "":
		return icSchedule
	}
	return nil
}

func (u *window) showToast(t ui.Toast) { u.toaster.Show(t) }

// scheduleItem is the card menu's way to the dialog.
func scheduleItem(sv *sessionView, act *sidebarAction) ui.MenuEntry {
	id := sv.ID
	label := i18n.T("sidebar.sessionCard.schedule")
	if scheduleMark(sv) != nil {
		label = i18n.T("sidebar.sessionCard.scheduled")
	}
	return ui.MenuEntry{Label: label, Icon: icSchedule, OnClick: func() { act.schedule = id }}
}

// scheduleTipSections are the tooltip's lines for what a row holds: a watch and
// lich's own continuation can sit on one session together.
func scheduleTipSections(th *ui.Theme, sv *sessionView, now time.Time) []layout.Widget {
	var sections []layout.Widget
	if w := sv.Watch; w.On != "" {
		sections = append(sections, scheduleTipLine(th, icWatch, watchTip(w.On), watchTipDetail(w)))
	}
	if sv.ScheduledPrompt != "" {
		when := scheduleTipWhen(time.Unix(sv.ScheduledAt, 0), now)
		title := i18n.T("sidebar.scheduleTip.at", i18n.Args{"when": when})
		detail := ""
		if sv.ScheduledBy == api.ScheduledByLich {
			title = i18n.T("sidebar.scheduleTip.resume", i18n.Args{"when": when})
		} else if sv.ScheduledBy == api.ScheduledByAgent {
			detail = i18n.T("sidebar.scheduleDialog.setByAgent")
		}
		sections = append(sections, scheduleTipLine(th, icSchedule, title, detail))
	}
	return sections
}

func watchTip(on string) string {
	switch on {
	case api.WatchChecksFailed:
		return i18n.T("sidebar.scheduleTip.checksFailed")
	case api.WatchChecksPassed:
		return i18n.T("sidebar.scheduleTip.checksPassed")
	}
	return i18n.T("sidebar.scheduleTip.review")
}

func watchTipDetail(w api.SessionWatch) string {
	var parts []string
	if w.On == api.WatchChecksFailed {
		parts = append(parts, i18n.T("sidebar.scheduleDialog.runsUsed", i18n.Args{"runs": w.Runs, "limit": api.WatchRunLimit}))
	}
	if w.By == api.ScheduledByAgent {
		parts = append(parts, i18n.T("sidebar.scheduleDialog.setByAgent"))
	}
	return strings.Join(parts, " ")
}

func scheduleTipWhen(at, now time.Time) string {
	if at.Year() == now.Year() && at.YearDay() == now.YearDay() {
		return at.Format("15:04")
	}
	return at.Format(scheduleClockLayout)
}

func scheduleTipLine(th *ui.Theme, ic *icons.Icon, title, detail string) layout.Widget {
	lines := []layout.Widget{para(th.Text(ui.TextXS, title)).Layout}
	if detail != "" {
		lines = append(lines, para(th.Text(ui.TextXS, detail).Muted()).Layout)
	}
	return func(gtx C) D {
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return ic.Layout(gtx, ui.Space(3), th.MutedForeground) }),
			ui.Gap(1.5),
			layout.Flexed(1, func(gtx C) D { return stack(gtx, 0.5, lines) }),
		)
	}
}
