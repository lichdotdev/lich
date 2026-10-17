package main

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// homeView is Home.tsx, an EmptyScreen: the folder glyph, the title, one
// line of copy, and the button to the directory picker. th is on the
// island it fills.
func homeView(gtx C, th *ui.Theme, open *widget.Clickable) D {
	button := th.Button(open, i18n.T("shell.home.openProject"))
	button.Icon = icFolderOpen
	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return icFolderOpen.Layout(gtx, ui.Space(12), th.MutedForeground) }),
			ui.VGap(4),
			layout.Rigid(th.Text(18, i18n.T("shell.home.title")).Leading(28).Weight(font.SemiBold).Layout),
			layout.Rigid(th.Text(ui.TextSM, i18n.T("shell.home.description")).Muted().Layout),
			ui.VGap(4),
			layout.Rigid(button.Layout),
		)
	})
}

var icSquareTerminal = icons.Lucide("square-terminal")

// emptySessionsView is EmptySessions.tsx, an EmptyScreen over a project with
// no session open. noAgent is a project whose new session would be a shell,
// so the button says what it will spawn. th is on the island it fills.
func emptySessionsView(gtx C, th *ui.Theme, open *widget.Clickable, noAgent bool) D {
	description, text, icon := i18n.T("shell.emptySessions.description"), i18n.T("shell.emptySessions.newSession"), icPlus
	if noAgent {
		description, text, icon = i18n.T("shell.emptySessions.noAgent"), i18n.T("shell.emptySessions.newTerminal"), icSquareTerminal
	}
	button := th.Button(open, text)
	button.Icon = icon
	return layout.Center.Layout(gtx, func(gtx C) D {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx C) D { return icSquareTerminal.Layout(gtx, ui.Space(12), th.MutedForeground) }),
			ui.VGap(4),
			layout.Rigid(th.Text(18, i18n.T("shell.emptySessions.title")).Leading(28).Weight(font.SemiBold).Layout),
			layout.Rigid(th.Text(ui.TextSM, description).Muted().Layout),
			ui.VGap(4),
			layout.Rigid(button.Layout),
		)
	})
}
