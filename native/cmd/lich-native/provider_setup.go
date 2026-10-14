package main

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui"
)

// ponytail: English until the native window has i18n (shell.providerSetupDialog).
const (
	setupChooseTitle  = "Choose your agents"
	setupNoneTitle    = "No coding agents found"
	setupChooseDesc   = "lich found these on your machine. Turn on the ones you use — the rest stay out of the New Session menu."
	setupNoneDesc     = "lich runs the agents already installed on your machine, and found none. Install one, then check again."
	setupChangeLater  = "You can change this any time in Settings › Providers."
	setupPointToBin   = "Already have one installed? Point lich at its binary in Settings › Providers."
	setupContinue     = "Continue"
	setupBodyHeight   = 60 // max-h share of the window the provider list may take, as the notes dialog
	setupDocsColumns  = 2
	setupDocsRowWidth = unit.Dp(200)
)

type setupDecision uint8

const (
	// setupWait is detection not having answered yet, not a bare machine.
	setupWait setupDecision = iota
	setupSkip
	setupShow
)

// decideProviderSetup is provider-setup.ts's: the dialog is the first launch
// with no default chosen, and preselects the first provider installed, ""
// when there is none.
func decideProviderSetup(m providersMachine) (setupDecision, string) {
	if !m.loaded || len(m.list) == 0 {
		return setupWait, ""
	}
	if m.globals[providerDefaultKey] != "" {
		return setupSkip, ""
	}
	for _, p := range m.list {
		if p.installed {
			return setupShow, p.id
		}
	}
	return setupShow, ""
}

// providerSetup is ProviderSetupGate: Continue, which stores the default,
// is the only way out.
type providerSetup struct {
	// shown latches the decision: every toggle inside the dialog moves
	// what it reads. installed is the shape, once a provider was found.
	shown, installed bool
	done             bool
	dialog           ui.DialogState
	list             widget.List
	cont             widget.Clickable
	docs             map[string]*widget.Clickable
}

// open decides once; the first time a provider is installed, at the
// decision or after Check again, it writes the absent providers off and the
// preselected one on, so the list is the whole truth.
func (g *providerSetup) open(s *settingsScreen) bool {
	if g.done {
		return false
	}
	decision, preselect := decideProviderSetup(s.providers.snapshot())
	if !g.shown {
		if decision != setupShow {
			g.done = decision == setupSkip
			return false
		}
		g.shown = true
	}
	if !g.installed && preselect != "" {
		g.installed = true
		for _, p := range s.providers.snapshot().list {
			if !p.installed && p.enabled {
				s.setProviderEnabled(p.id, false)
			}
		}
		s.setProviderEnabled(preselect, true)
	}
	return true
}

func (u *window) layoutProviderSetup(gtx C, th *ui.Theme) {
	g, s := &u.setup, u.settings
	if !g.open(s) {
		return
	}
	if g.cont.Clicked(gtx) {
		m := s.providers.snapshot()
		s.writeProviderKey(providerDefaultKey, "", resolveProjectDefaultProvider(m.launchList(), "", ""), s.providersChanged)
		g.done = true
		return
	}
	on := th.On(th.Popover)
	title, desc, foot := setupChooseTitle, setupChooseDesc, setupChangeLater
	if !g.installed {
		title, desc, foot = setupNoneTitle, setupNoneDesc, setupPointToBin
	}
	d := th.Dialog(&g.dialog, true, title, desc)
	d.MaxWidth, d.HideClose = ui.DialogWidthXL, true
	d.Body = func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx C) D {
				if g.installed {
					return g.providers(gtx, on, s)
				}
				return g.docsGrid(gtx, on, s)
			}),
			ui.VGap(4),
			layout.Rigid(on.Text(ui.TextSM, foot).Muted().Layout),
		)
	}
	actions := []layout.Widget{}
	if !g.installed {
		actions = append(actions, s.recheckButton(gtx, on))
	}
	d.Actions = append(actions, on.Button(&g.cont, setupContinue).Layout)
	d.Layout(gtx)
}

// providers is the Settings › Providers pane verbatim, so the first screen a
// user meets is the one they find again.
func (g *providerSetup) providers(gtx C, th *ui.Theme, s *settingsScreen) D {
	rows := s.providerPaneRows(th, s.snapshot(), 0)
	g.list.Axis = layout.Vertical
	gtx.Constraints.Max.Y = gtx.Constraints.Max.Y * setupBodyHeight / 100
	return th.ScrollArea(&g.list).Layout(gtx, len(rows), func(gtx C, i int) D { return rows[i](gtx) })
}

// docsGrid is every provider lich knows, each opening its install page.
func (g *providerSetup) docsGrid(gtx C, th *ui.Theme, s *settingsScreen) D {
	if g.docs == nil {
		g.docs = map[string]*widget.Clickable{}
	}
	list := s.providers.snapshot().list
	var rows []layout.FlexChild
	for i := 0; i < len(list); i += setupDocsColumns {
		var cells []layout.FlexChild
		for _, p := range list[i:min(i+setupDocsColumns, len(list))] {
			click := g.docs[p.id]
			if click == nil {
				click = new(widget.Clickable)
				g.docs[p.id] = click
			}
			if click.Clicked(gtx) {
				s.openExternal(p.docs)
			}
			cells = append(cells, layout.Rigid(func(gtx C) D {
				gtx.Constraints.Min.X = gtx.Dp(setupDocsRowWidth)
				return click.Layout(gtx, func(gtx C) D {
					if click.Hovered() {
						ui.Fill(gtx, th.Over(ui.Alpha(th.Accent, ghostHoverFill)), ui.RadiusMD)
					}
					return layout.UniformInset(ui.Space(2)).Layout(gtx, func(gtx C) D {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx C) D { return providerIcon(p.id).Layout(gtx, ui.Space(4), th.Foreground) }),
							ui.Gap(2),
							layout.Rigid(th.Text(ui.TextSM, p.name).Layout),
						)
					})
				})
			}))
		}
		rows = append(rows, layout.Rigid(func(gtx C) D { return layout.Flex{}.Layout(gtx, cells...) }))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}

// recheckButton is CheckAgainButton over the provider book.
func (s *settingsScreen) recheckButton(gtx C, th *ui.Theme) layout.Widget {
	pr := &s.providers
	if pr.recheck.Clicked(gtx) {
		pr.recheckPath(s)
	}
	pr.mu.Lock()
	checking := pr.rechecking
	pr.mu.Unlock()
	b := th.Button(&pr.recheck, providersCheckAgain)
	b.Variant, b.Disabled = ui.VariantOutline, checking
	if checking {
		b.Text = providersChecking
	}
	return b.Layout
}
