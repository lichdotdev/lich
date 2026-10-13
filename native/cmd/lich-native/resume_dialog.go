package main

import (
	"context"
	"fmt"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

// ponytail: English until the native window has i18n.
const (
	resumeTitle            = "Resume previous session?"
	resumeDescription      = "%s left a conversation behind (%s). Resume it to pick the conversation up where it stopped, or start new for an empty one."
	resumeAlwaysFor        = "Always do this for %s"
	resumeChangeInSettings = "· change it in Settings › Providers"
	resumeThisProvider     = "this provider"
	resumeStartNew         = "Start new"
	resumeResume           = "Resume"
	resumeInProfile        = "It resumes in %s · %s, the profile it started in."
	resumeInProfileNoMail  = "It resumes in %s, the profile it started in."
)

// resumeDialog is ResumeSessionDialog: whether a restored card continues
// the conversation it ran before the restart or starts a new one.
// Dismissing is Start new, since the card needs a terminal either way, but
// only the buttons remember: a dismiss is not a choice anyone made.
type resumeDialog struct {
	dialog   ui.DialogState
	remember widget.Bool
	// shown is the session last asked about, kept so the closing fade still
	// reads it; a new one unticks the box.
	shown         lichclient.Session
	provider      string
	rememberLabel widget.Clickable
	startNew      widget.Clickable
	resume        widget.Clickable
}

// layout draws the prompt the gate raised, if any, and hands it the answer.
// Call it before the window's content, so it takes Escape ahead of the
// terminal.
func (r *resumeDialog) layout(ctx context.Context, gtx C, th *ui.Theme, g *spawnGate, m *model) {
	s := g.prompt()
	open := s != nil
	if open && r.shown.ID != s.ID {
		r.shown, r.provider, r.remember.Value = *s, m.providerName(s.Kind), false
	}
	if r.rememberLabel.Clicked(gtx) {
		r.remember.Value = !r.remember.Value
	}
	switch {
	case r.startNew.Clicked(gtx):
		go g.answer(ctx, restoreFresh, r.remember.Value)
	case r.resume.Clicked(gtx):
		go g.answer(ctx, restoreResume, r.remember.Value)
	}
	pop := th.On(th.Popover)
	description := fmt.Sprintf(resumeDescription, r.shown.Label, r.shown.ProviderSessionID)
	if line := m.resumeProfileLine(r.shown); line != "" {
		description += " " + line
	}
	d := th.Dialog(&r.dialog, open, resumeTitle, description)
	d.MaxWidth = ui.DialogWidthXL
	d.Body = func(gtx C) D { return r.rememberRow(gtx, pop, r.provider) }
	startNew := pop.Button(&r.startNew, resumeStartNew)
	startNew.Variant = ui.VariantGhost
	d.Actions = []layout.Widget{startNew.Layout, pop.Button(&r.resume, resumeResume).Layout}
	if d.Layout(gtx) {
		go g.answer(ctx, restoreFresh, false)
	}
}

// rememberRow is the checkbox and its label (flex items-center gap-2); a
// click on the label ticks the box, as htmlFor does.
func (r *resumeDialog) rememberRow(gtx C, th *ui.Theme, provider string) D {
	caption := th.Label(fmt.Sprintf(resumeAlwaysFor, provider))
	caption.Weight = font.Normal
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(th.Checkbox(&r.remember).Layout),
		ui.Gap(2),
		layout.Rigid(func(gtx C) D {
			return r.rememberLabel.Layout(gtx, func(gtx C) D {
				return layout.Flex{Alignment: layout.Baseline}.Layout(gtx,
					layout.Rigid(caption.Layout),
					ui.Gap(2),
					layout.Rigid(th.Text(ui.TextSM, resumeChangeInSettings).Muted().Layout),
				)
			})
		}),
	)
}

// providerName is how the prompt names provider kind.
func (m *model) providerName(kind string) string {
	if p, ok := m.providers.find(kind); ok {
		return p.name
	}
	return resumeThisProvider
}
