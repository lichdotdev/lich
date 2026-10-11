package main

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/omartelo/lich/native/ui"
	"github.com/omartelo/lich/native/ui/icons"
)

// ponytail: English until the native window has i18n.
const (
	homeTitle       = "No project open"
	homeDescription = "Open a folder to start a terminal session."
	homeOpenProject = "Open project"
)

// homeView is Home.tsx, an EmptyScreen: the folder glyph, the title, one
// line of copy, and the button to the directory picker. th is on the
// island it fills.
func homeView(gtx C, th *ui.Theme, open *widget.Clickable) D {
	button := th.Button(open, homeOpenProject)
	button.Icon = icFolderOpen
	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return icFolderOpen.Layout(gtx, ui.Space(12), th.MutedForeground) }),
			ui.VGap(4),
			layout.Rigid(th.Text(18, homeTitle).Leading(28).Weight(font.SemiBold).Layout),
			layout.Rigid(th.Text(ui.TextSM, homeDescription).Muted().Layout),
			ui.VGap(4),
			layout.Rigid(button.Layout),
		)
	})
}

// ponytail: English until the native window has i18n.
const (
	emptySessionsTitle       = "No session open"
	emptySessionsDescription = "Open a session to start working in this project."
	emptySessionsNoAgent     = "No agent found on PATH, so this opens a terminal — set one in Settings › Providers."
	emptySessionsNewTerminal = "New terminal"
	emptySessionsNewSession  = "New session"
)

var icSquareTerminal = icons.Lucide("square-terminal")

// emptySessionsView is EmptySessions.tsx, an EmptyScreen over a project with
// no session open. noAgent is a project whose new session would be a shell,
// so the button says what it will spawn. th is on the island it fills.
func emptySessionsView(gtx C, th *ui.Theme, open *widget.Clickable, noAgent bool) D {
	description, text, icon := emptySessionsDescription, emptySessionsNewSession, icPlus
	if noAgent {
		description, text, icon = emptySessionsNoAgent, emptySessionsNewTerminal, icSquareTerminal
	}
	button := th.Button(open, text)
	button.Icon = icon
	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return icSquareTerminal.Layout(gtx, ui.Space(12), th.MutedForeground) }),
			ui.VGap(4),
			layout.Rigid(th.Text(18, emptySessionsTitle).Leading(28).Weight(font.SemiBold).Layout),
			layout.Rigid(th.Text(ui.TextSM, description).Muted().Layout),
			ui.VGap(4),
			layout.Rigid(button.Layout),
		)
	})
}
