package main

import (
	"image/color"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
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
	icPRLinked = icons.Lucide("link")
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

// prBadge draws a session's pull request in its tooltip and on its chat
// composer's line, its number in size, a link before it when it was linked by
// hand; logo is the ink of the forge's logo and the link, which the composer
// brightens on hover.
func prBadge(gtx C, th *ui.Theme, sv *sessionView, logo color.NRGBA, size unit.Sp) D {
	l := prLookOf(th, sv.pr, sv.prForge())
	children := []layout.FlexChild{layout.Rigid(func(gtx C) D { return l.logo.Layout(gtx, 12, logo) }), ui.Gap(1)}
	if sv.prLink != nil {
		children = append(children, layout.Rigid(func(gtx C) D { return icPRLinked.Layout(gtx, 12, logo) }), ui.Gap(1))
	}
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, append(children,
		layout.Rigid(func(gtx C) D { return l.glyph.Layout(gtx, 12, l.tone) }),
		ui.Gap(1),
		layout.Rigid(th.Text(size, l.number).In(l.tone).Layout),
	)...)
}

// prForge is the forge the session's pull request is on: a hand link's own,
// else the shown path's.
func (sv *sessionView) prForge() string {
	if sv.prLink != nil {
		return sv.prLink.Forge
	}
	return sv.forge
}

// prTarget is where a click on the session's pull request goes: a link made
// by number is the project's and opens on Pulls, one made by URL may be any
// repository's and opens in the browser; the branch's goes where prTarget
// sends it.
func (sv *sessionView) prTarget() (number int, url string) {
	switch {
	case sv.prLink == nil:
		return prTarget(sv.pr, sv.shown, sv.path)
	case sv.prLink.Here:
		return sv.pr.Number, ""
	}
	return 0, sv.pr.URL
}

// prTip is the tooltip over the session's pull request: its title, why its
// last read failed, and the other ones the session has.
func (sv *sessionView) prTip() string {
	lines := []string{sv.pr.Title}
	if sv.prLink != nil && sv.prLink.Error != "" {
		lines = append(lines, i18n.T("sidebar.sessionCard.prUnreadable", i18n.Args{"error": sv.prLink.Error}))
	}
	if len(sv.prOthers) > 0 {
		refs := make([]string, len(sv.prOthers))
		for i, o := range sv.prOthers {
			refs[i] = changeRef(sv.prForge(), o.Number)
		}
		lines = append(lines, i18n.T("sidebar.sessionCard.prAlsoLinked", i18n.Args{"numbers": strings.Join(refs, ", ")}))
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
