package main

import (
	"image/color"
	"strconv"

	"gioui.org/layout"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

var (
	icPR       = icons.Lucide("git-pull-request-arrow")
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

func prLookOf(th *ui.Theme, pr *backend.PullRequest) prLook {
	l := prLook{logo: icPR, number: "#" + strconv.Itoa(pr.Number)}
	switch pr.Forge {
	case api.ForgeGitHub:
		l.logo = icGitHub
	case api.ForgeGitLab:
		l.logo, l.number = icGitLab, "!"+strconv.Itoa(pr.Number)
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

// prBadge draws pr on a session's row and in its tooltip; logo is the ink of
// the forge's logo, which the row brightens on hover.
func prBadge(gtx C, th *ui.Theme, pr *backend.PullRequest, logo color.NRGBA) D {
	l := prLookOf(th, pr)
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx C) D { return l.logo.Layout(gtx, 12, logo) }),
		ui.Gap(1),
		layout.Rigid(func(gtx C) D { return l.glyph.Layout(gtx, 12, l.tone) }),
		ui.Gap(1),
		layout.Rigid(th.Text(ui.TextXS, l.number).In(l.tone).Layout),
	)
}
