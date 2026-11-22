package main

import (
	"image"
	"strings"
	"testing"

	"gioui.org/f32"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/ui"
	"github.com/lichdotdev/lich/native/ui/icons"
)

type prCase struct {
	name   string
	pr     backend.PullRequest
	logo   *icons.Icon
	glyph  *icons.Icon
	tone   func(*ui.Theme) any
	number string
}

func prCases() []prCase {
	pass := func(th *ui.Theme) any { return th.TonePass }
	return []prCase{
		{"open", backend.PullRequest{Number: 990, State: "OPEN", Forge: api.ForgeGitHub}, icGitHub, icPROpen, pass, "#990"},
		{"no state", backend.PullRequest{Number: 990, Forge: api.ForgeGitHub}, icGitHub, icPROpen, pass, "#990"},
		{"draft", backend.PullRequest{Number: 990, State: "OPEN", IsDraft: true, Forge: api.ForgeGitHub}, icGitHub, icPRDraft, func(th *ui.Theme) any { return th.MutedForeground }, "#990"},
		{"merged", backend.PullRequest{Number: 990, State: "MERGED", Forge: api.ForgeGitHub}, icGitHub, icPRMerged, func(th *ui.Theme) any { return th.ToneMerged }, "#990"},
		{"closed draft", backend.PullRequest{Number: 990, State: "CLOSED", IsDraft: true, Forge: api.ForgeGitHub}, icGitHub, icPRClosed, func(th *ui.Theme) any { return th.Destructive }, "#990"},
		{"gitlab", backend.PullRequest{Number: 12, State: "OPEN", Forge: api.ForgeGitLab}, icGitLab, icPROpen, pass, "!12"},
		{"unknown forge", backend.PullRequest{Number: 990, State: "OPEN"}, icPR, icPROpen, pass, "#990"},
	}
}

func TestPRLookFollowsStateAndForge(t *testing.T) {
	th := &ui.Theme{Palette: ui.Dark}
	for _, c := range prCases() {
		l := prLookOf(th, &c.pr)
		if l.logo != c.logo || l.glyph != c.glyph || any(l.tone) != c.tone(th) || l.number != c.number {
			t.Errorf("%s: logo %p glyph %p tone %v number %q; want logo %p glyph %p tone %v number %q",
				c.name, l.logo, l.glyph, l.tone, l.number, c.logo, c.glyph, c.tone(th), c.number)
		}
	}
}

func TestOnlyMergedAndClosedPRsAreNotOpen(t *testing.T) {
	for state, open := range map[string]bool{"": true, "OPEN": true, "MERGED": false, "CLOSED": false} {
		if got := isOpenPR(&backend.PullRequest{State: state}); got != open {
			t.Errorf("isOpenPR(%q) = %v, want %v", state, got, open)
		}
	}
	if isOpenPR(nil) {
		t.Error("isOpenPR(nil) = true")
	}
}

// The row and the tooltip draw every state with its number, and no two states
// alike.
func TestRowAndTooltipDrawEachPRState(t *testing.T) {
	places := map[string]func(sv *sessionView) func(C, *ui.Theme){
		"row": func(sv *sessionView) func(C, *ui.Theme) {
			s, chip := newSidebar(&ui.Root{}), new(ui.Clickable)
			more := func(gtx C) D { return D{Size: image.Pt(16, 16)} }
			return func(gtx C, th *ui.Theme) { s.cardBody(gtx, th, sv, chip, more, false) }
		},
		"tooltip": func(sv *sessionView) func(C, *ui.Theme) {
			return func(gtx C, th *ui.Theme) { branchSection(th, sv)(gtx) }
		},
	}
	for place, draw := range places {
		paints := map[string]string{}
		for _, c := range prCases() {
			if c.name == "no state" {
				continue
			}
			sv := sessionView{ID: "s1", Label: "Docs", shown: "/repo", git: backend.DiffStats{Branch: "feat"}, pr: &c.pr}
			r := newFrameRig(image.Pt(400, 40), draw(&sv))
			drawn, _ := r.frame()
			if !r.shows(c.number) {
				t.Errorf("%s %s: no %q drawn:\n%s", place, c.name, c.number, drawn)
			}
			paint := strings.SplitN(drawn, "\n", 2)[0]
			if other, ok := paints[paint]; ok {
				t.Errorf("%s: %s draws the same as %s", place, c.name, other)
			}
			paints[paint] = c.name
		}
	}
}

// Hovering the row's badge brightens its logo and leaves the state's glyph and
// number in their tone.
func TestHoverBrightensOnlyTheLogo(t *testing.T) {
	pr := backend.PullRequest{Number: 990, State: "MERGED", Forge: api.ForgeGitHub}
	chip := new(ui.Clickable)
	r := newFrameRig(image.Pt(80, 20), func(gtx C, th *ui.Theme) { prChipLayout(gtx, th, chip, &pr) })
	r.frame()
	rest := renderRig(t, r)
	number := labelBox(t, r, "#990")
	r.hover(f32.Pt(float32(number.Min.X+2), float32(number.Min.Y+number.Dy()/2)))
	r.frame()
	hovered := renderRig(t, r)
	logo := image.Rect(0, 0, 12, r.size.Y)
	tinted := image.Rect(logo.Max.X+1, 0, r.size.X, r.size.Y)
	if inkShift(rest, hovered, logo) == 0 {
		t.Error("hover left the logo as it was")
	}
	if d := inkShift(rest, hovered, tinted); d != 0 {
		t.Errorf("hover moved the glyph or the number by %d", d)
	}
}
