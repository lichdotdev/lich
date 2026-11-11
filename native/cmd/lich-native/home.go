package main

import (
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

// homeView is the empty screen of Home: the folder glyph, the title, one line
// of copy, and the button to the directory picker. th is on the island it
// fills.
func homeView(gtx C, th *ui.Theme, open *widget.Clickable) D {
	button := th.Button(open, i18n.T("shell.home.openProject"))
	button.Icon = icFolderOpen
	return th.EmptyState(icFolderOpen, i18n.T("shell.home.title"), i18n.T("shell.home.description"), button.Layout).Layout(gtx)
}

var icSquareTerminal = icons.Lucide("square-terminal")

// emptySessionsView is the empty screen over a project with no session open.
// noAgent is a project whose new session would be a shell, so the button says
// what it will spawn. th is on the island it fills.
func emptySessionsView(gtx C, th *ui.Theme, open *widget.Clickable, noAgent bool, spot func(C, string, layout.Widget) D) D {
	description, text, icon := i18n.T("shell.emptySessions.description"), i18n.T("shell.emptySessions.newSession"), icPlus
	if noAgent {
		description, text, icon = i18n.T("shell.emptySessions.noAgent"), i18n.T("shell.emptySessions.newTerminal"), icSquareTerminal
	}
	button := th.Button(open, text)
	button.Icon = icon
	return th.EmptyState(icSquareTerminal, i18n.T("shell.emptySessions.title"), description, func(gtx C) D { return spot(gtx, tourNewEmpty, button.Layout) }).Layout(gtx)
}
