package main

import (
	"image/color"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icPROpen   = icons.Lucide("git-pull-request")
	icPRDraft  = icons.Lucide("git-pull-request-draft")
	icPRMerged = icons.Lucide("git-merge")
	icPRClosed = icons.Lucide("git-pull-request-closed")
	icGitHub   = icons.Brand("github")
	icGitLab   = icons.Brand("gitlab")
)

// The states a pull request reader reports besides open.
const (
	prStateMerged = "MERGED"
	prStateClosed = "CLOSED"
)

// isOpenPR answers whether pr is a pull request still open. A read with no
// state counts as open, as its badge draws it.
func isOpenPR(pr *backend.PullRequest) bool {
	return pr != nil && pr.State != prStateMerged && pr.State != prStateClosed
}

// prLook is what a pull request's badge draws: its forge's logo, then its
// state's glyph and its number in the state's tone.
type prLook struct {
	logo, glyph *icons.Icon
	tone        color.NRGBA
	number      string
}

// forge is the checkout's, from the forge store; a caller draws no badge while
// it is unknown, so the wrong logo never flashes.
func prLookOf(th *ui.Theme, pr *backend.PullRequest, forge string) prLook {
	l := prLook{logo: icGitHub, number: changeRef(forge, pr.Number)}
	if forge == api.ForgeGitLab {
		l.logo = icGitLab
	}
	switch {
	case pr.State == prStateMerged:
		l.glyph, l.tone = icPRMerged, th.ToneMerged
	case pr.State == prStateClosed:
		l.glyph, l.tone = icPRClosed, th.Destructive
	case pr.IsDraft:
		l.glyph, l.tone = icPRDraft, th.MutedForeground
	default:
		l.glyph, l.tone = icPROpen, th.TonePass
	}
	return l
}

// prBadge draws pr in a session's tooltip and on its chat composer's line, its
// number in size; logo is the ink of the forge's logo, which the composer
// brightens on hover.
func prBadge(gtx C, th *ui.Theme, pr *backend.PullRequest, forge string, logo color.NRGBA, size unit.Sp) D {
	l := prLookOf(th, pr, forge)
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return l.logo.Layout(gtx, 12, logo) }),
		ui.Gap(1),
		layout.Rigid(func(gtx C) D { return l.glyph.Layout(gtx, 12, l.tone) }),
		ui.Gap(1),
		layout.Rigid(th.Text(size, l.number).In(l.tone).Layout),
	)
}
