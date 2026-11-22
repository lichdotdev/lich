package main

import (
	"errors"
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
	"github.com/lichdotdev/lich/native/ui"
)

var epoch = time.Unix(0, 0)

// resetAfter is a reset time that far after the rig's clock starts, with half a minute
// over so the rig's frames, a second apart, do not tip the minute.
func resetAfter(d time.Duration) string {
	return epoch.Add(d + 30*time.Second).UTC().Format(time.RFC3339)
}

// usagePlan is a Claude plan whose windows arrive in the wrong order, with a
// five-hour one, a weekly one on course to run out and a model's own cap.
func usagePlan() backend.QuotaPlan {
	return backend.QuotaPlan{
		Provider: "claude", Name: "Claude Code", Plan: "Max 5x", Status: "ok", Account: "dev@example.com",
		Windows: []backend.QuotaWindow{
			{Label: "Fable", Seconds: 604800, Percent: 6, ResetsAt: resetAfter(6 * 24 * time.Hour)},
			{Kind: api.QuotaWindowWeekly, Seconds: 604800, Percent: 60, ResetsAt: resetAfter(3 * 24 * time.Hour), RunsOutAt: resetAfter(33 * time.Hour)},
			{Kind: api.QuotaWindowSession, Seconds: 18000, Percent: 25, ResetsAt: resetAfter(4*time.Hour + 12*time.Minute)},
		},
	}
}

// tipFrames draws a plan's usage alone, as the account row's panel does.
func tipFrames(t *testing.T, plan backend.QuotaPlan) *frameRig {
	t.Helper()
	r := newFrameRig(image.Pt(400, 700), func(gtx C, th *ui.Theme) {
		ui.Stack(usageBlockGap, profileUsageRows(th.On(th.Popover), profileFocus{kind: plan.Provider}, plan, true, gtx.Now, nil)...)(gtx)
	})
	r.settle(t)
	return r
}

func wantShown(t *testing.T, r *frameRig, labels ...string) {
	t.Helper()
	for _, label := range labels {
		if !r.shows(label) {
			t.Errorf("the tooltip lacks %q:\n%s", label, r.drawn())
		}
	}
}

func wantAbsent(t *testing.T, r *frameRig, labels ...string) {
	t.Helper()
	for _, label := range labels {
		if r.shows(label) {
			t.Errorf("the tooltip shows %q, which the reading never carried:\n%s", label, r.drawn())
		}
	}
}

func TestTheUsageTooltipListsTheSessionThenTheWeekThenTheModels(t *testing.T) {
	r := tipFrames(t, usagePlan())
	wantShown(t, r, "Session · 5h", "25%", "Resets in 4h 12m", "Weekly · 7d", "60%", "Resets in 3d 0h", "Runs out in 1d 9h", "Fable · 7d", "6%", "dev@example.com")
	var ys []float32
	for _, label := range []string{"Session · 5h", "Weekly · 7d", "Fable · 7d"} {
		ys = append(ys, r.at(t, label).Y)
	}
	if !(ys[0] < ys[1] && ys[1] < ys[2]) {
		t.Errorf("session, week and model sit at %v, want them top to bottom", ys)
	}
}

func TestTheRunsOutLineIsOnlyForAWindowThatRunsOut(t *testing.T) {
	plan := usagePlan()
	plan.Windows[1].RunsOutAt = ""
	wantAbsent(t, tipFrames(t, plan), "Runs out in 1d 9h")
}

func TestExtraUsageShowsWhatIsSpentAgainstTheCapAndOnlyWhenOn(t *testing.T) {
	plan := usagePlan()
	wantAbsent(t, tipFrames(t, plan), "Extra usage")

	plan.Extra = &backend.QuotaExtra{Used: 1250, Limit: new(int64(5000)), Currency: "USD", Decimals: 2}
	wantShown(t, tipFrames(t, plan), "Extra usage", "$12.50 of $50.00")

	plan.Extra = &backend.QuotaExtra{Used: 14157, Currency: "USD", Decimals: 2}
	wantShown(t, tipFrames(t, plan), "Extra usage", "$141.57 · no limit")

	i18n.SetCurrent(i18n.PtBR)
	defer i18n.SetCurrent(i18n.EN)
	plan.Extra = &backend.QuotaExtra{Used: 1250, Limit: new(int64(5000)), Currency: "USD", Decimals: 2}
	wantShown(t, tipFrames(t, plan), "Uso extra", "$"+nbsp+"12,50 de $"+nbsp+"50,00")
}

func TestResetCreditsShowWhenThereAreSome(t *testing.T) {
	plan := backend.QuotaPlan{Provider: "codex", Name: "Codex", Status: "ok", Windows: []backend.QuotaWindow{
		{Kind: api.QuotaWindowSession, Seconds: 18000, Percent: 4, ResetsAt: resetAfter(time.Hour)},
	}}
	wantAbsent(t, tipFrames(t, plan), "Reset credits")
	plan.ResetCredits = 1
	wantShown(t, tipFrames(t, plan), "Reset credits", "1 available")
	plan.ResetCredits = 3
	wantShown(t, tipFrames(t, plan), "3 available")
}

func TestALockedWindowSaysSoAndKeepsItsReason(t *testing.T) {
	plan := usagePlan()
	plan.Windows[2].LockedReason = "Session limit reached"
	wantShown(t, tipFrames(t, plan), "Locked", "Session limit reached")
}

func TestTheProfilePanelSaysWhyThereIsNoUsage(t *testing.T) {
	draw := func(f profileFocus, plan backend.QuotaPlan, loaded bool) *frameRig {
		r := newFrameRig(image.Pt(400, 300), func(gtx C, th *ui.Theme) {
			if rows := profileUsageRows(th.On(th.Popover), f, plan, loaded, gtx.Now, nil); len(rows) > 0 {
				ui.Stack(usageBlockGap, rows...)(gtx)
			}
		})
		r.settle(t)
		return r
	}
	claude := profileFocus{kind: claudeKind, book: &profileBook{provider: claudeProfileProvider}}
	omp := profileFocus{book: &profileBook{provider: ompProfileProvider}}
	cursor := profileFocus{book: &profileBook{provider: cursorProfileProvider}, choice: projectProfile{shown: "work"}}
	cursorPersonal := profileFocus{kind: cursorKind, book: &profileBook{provider: cursorProfileProvider}}

	wantShown(t, draw(omp, backend.QuotaPlan{}, false), "Usage is not measured per profile for oh-my-pi.")
	// Cursor CLI has a plan, but lich reads it for the machine's login alone: another profile's panel never draws it.
	wantShown(t, draw(cursor, usagePlan(), true), "Usage is not measured per profile for Cursor CLI.")
	wantAbsent(t, draw(cursor, usagePlan(), true), "Session · 5h")
	// Personal is the machine's login, the one lich reads Cursor CLI for.
	wantShown(t, draw(cursorPersonal, usagePlan(), true), "Session · 5h")
	wantShown(t, draw(claude, backend.QuotaPlan{Status: "signed-out"}, true), "Signed out.")
	wantShown(t, draw(claude, backend.QuotaPlan{Status: "error"}, true), "Plan usage could not be read.")
	wantShown(t, draw(claude, backend.QuotaPlan{Status: "unknown"}, true), "Plan usage could not be read.")
	if rows := profileUsageRows(new(ui.Theme), claude, backend.QuotaPlan{}, false, epoch, nil); rows != nil {
		t.Error("a reading still in flight drew rows")
	}
	if rows := profileUsageRows(new(ui.Theme), claude, usagePlan(), true, epoch, nil); len(rows) == 0 {
		t.Error("a read plan has no rows")
	}
}

func TestSameQuotaSeesTheExtraSpendTheCreditsAndTheProjection(t *testing.T) {
	base := func() []backend.QuotaPlan { return []backend.QuotaPlan{usagePlan()} }
	if !sameQuota(base(), base()) {
		t.Fatal("two equal readings differ")
	}
	changes := map[string]func(*backend.QuotaPlan){
		"extra appears":     func(p *backend.QuotaPlan) { p.Extra = &backend.QuotaExtra{Used: 1} },
		"credits":           func(p *backend.QuotaPlan) { p.ResetCredits = 1 },
		"projection moves":  func(p *backend.QuotaPlan) { p.Windows[1].RunsOutAt = resetAfter(34 * time.Hour) },
		"projection ends":   func(p *backend.QuotaPlan) { p.Windows[1].RunsOutAt = "" },
		"extra spend moves": func(p *backend.QuotaPlan) { p.Extra = &backend.QuotaExtra{Used: 2} },
	}
	for name, change := range changes {
		a, b := base(), base()
		a[0].Extra = &backend.QuotaExtra{Used: 1}
		if name == "extra appears" {
			a[0].Extra = nil
		}
		change(&b[0])
		if sameQuota(a, b) {
			t.Errorf("%s: the reading was taken for the same", name)
		}
	}
	a, b := base(), base()
	a[0].Extra = &backend.QuotaExtra{Used: 1, Limit: new(int64(100))}
	b[0].Extra = &backend.QuotaExtra{Used: 1}
	if sameQuota(a, b) {
		t.Error("a cap that went away was taken for the same")
	}
}

// profileFrames is the sidebar with the profile footer in it, over a Claude
// plan seeded for the machine's login. The action of the last frame that had
// one is kept.
func profileFrames(t *testing.T) (*model, *frameRig, *sidebarAction) {
	t.Helper()
	_, m, _ := profileFooterRig(t)
	root := new(ui.Root)
	s := newSidebar(root)
	s.open = true
	act := new(sidebarAction)
	r := newFrameRig(image.Pt(400, 900), func(gtx C, th *ui.Theme) {
		root.Layout(gtx, func(gtx C) D {
			_, a := s.layout(gtx, th, m, active(m))
			if a.profilePick.key != "" || a.useReset.grantID != "" {
				*act = a
			}
			return D{Size: gtx.Constraints.Max}
		})
	})
	return m, r, act
}

func hoverProfile(t *testing.T, r *frameRig) {
	t.Helper()
	p := r.at(t, "Personal")
	r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(p.X, p.Y)})
	r.frame()
	r.now = r.now.Add(time.Second)
	r.frame()
	r.settle(t)
}

func openProfilePanel(t *testing.T, r *frameRig) {
	t.Helper()
	r.clickLabel(t, "Personal")
	r.settle(t)
}

func seedUsage(m *model, plan backend.QuotaPlan) { m.quota.seed("", []backend.QuotaPlan{plan}) }

func TestHoveringTheProfileFooterOpensNothing(t *testing.T) {
	m, r, _ := profileFrames(t)
	seedUsage(m, usagePlan())
	r.settle(t)
	hoverProfile(t, r)
	wantAbsent(t, r, "Session · 5h", "Claude Code · Max 5x")
}

func TestClickingTheProfileFooterOpensTheUsageOverTheProfiles(t *testing.T) {
	m, r, _ := profileFrames(t)
	plan := usagePlan()
	plan.Extra = &backend.QuotaExtra{Used: 1250, Limit: new(int64(5000)), Currency: "USD", Decimals: 2}
	seedUsage(m, plan)
	r.settle(t)
	wantAbsent(t, r, "Session · 5h")
	openProfilePanel(t, r)
	wantShown(t, r, "Claude Code · Max 5x", "Session · 5h", "Weekly · 7d", "Extra usage", "$12.50 of $50.00", "dev@example.com", "Work")
	if usage, work := r.at(t, "Session · 5h").Y, r.at(t, "Work").Y; usage >= work {
		t.Errorf("the usage sits at %v and the profiles at %v, want the usage on top", usage, work)
	}
}

func TestTheProfilePanelStillSwitchesProfiles(t *testing.T) {
	m, r, act := profileFrames(t)
	seedUsage(m, usagePlan())
	openProfilePanel(t, r)
	r.clickLabel(t, "Work")
	if act.profilePick.key != claudeProfileKey || act.profilePick.value != "work" {
		t.Fatalf("pick %+v, want Claude's profile set to work", act.profilePick)
	}
	r.settle(t)
	wantAbsent(t, r, "Session · 5h")
}

func TestTheProfilePanelOpensWithTheProfilesBeforeTheFirstReading(t *testing.T) {
	_, r, _ := profileFrames(t)
	r.settle(t)
	openProfilePanel(t, r)
	wantShown(t, r, "Work")
	wantAbsent(t, r, "Session · 5h", "Plan usage could not be read.")
}

func clickablePlan() backend.QuotaPlan {
	plan := usagePlan()
	plan.Resets = []backend.QuotaReset{
		{ID: "g-full", Kind: api.QuotaResetFull, Clears: []string{"session", "weekly", "model"}, Left: 1, EndsAt: endsOn(time.December, 10)},
		{ID: "g-held", Label: "Paused grant", Left: 1, EndsAt: endsOn(time.January, 2), Held: true},
	}
	return plan
}

func TestAHeldGrantHasNoUseButtonAndAnActiveOneAsksFirst(t *testing.T) {
	m, r, act := profileFrames(t)
	seedUsage(m, clickablePlan())
	openProfilePanel(t, r)
	r.settle(t)
	if n := countLabel(r, "Use"); n != 1 {
		t.Fatalf("%d Use buttons, want one (the held grant has none):\n%s", n, r.drawn())
	}
	r.clickLabel(t, "Use")
	r.settle(t)
	wantShown(t, r, "Use one Full reset?", "Clears the session and weekly limits.", "Cancel", "Use")
	if act.useReset.grantID != "" {
		t.Fatal("the first click spent a reset")
	}
	r.clickLabel(t, "Cancel")
	r.settle(t)
	wantAbsent(t, r, "Use one Full reset?")
	wantShown(t, r, "Use")
}

func TestConfirmingSendsTheSpendOfThatGrantForThatProfile(t *testing.T) {
	m, r, act := profileFrames(t)
	seedUsage(m, clickablePlan())
	openProfilePanel(t, r)
	r.clickLabel(t, "Use")
	r.settle(t)
	r.clickLabel(t, "Use")
	if got := act.useReset; got.grantID != "g-full" || got.profile != "personal" || got.key != "" {
		t.Fatalf("request %+v, want g-full for personal", got)
	}
}

func TestACodexPanelOffersNoUse(t *testing.T) {
	codex := profileFocus{kind: codexKind, book: &profileBook{provider: codexProfileProvider}}
	plan := clickablePlan()
	plan.Provider = "codex"
	r := newFrameRig(image.Pt(400, 700), func(gtx C, th *ui.Theme) {
		ui.Stack(usageBlockGap, profileUsageRows(th.On(th.Popover), codex, plan, true, gtx.Now, new(resetUse))...)(gtx)
	})
	r.settle(t)
	wantShown(t, r, "Resets")
	wantAbsent(t, r, "Use")
}

func countLabel(r *frameRig, label string) int {
	n := 0
	for _, node := range r.nodes() {
		if node.Desc.Label == label {
			n++
		}
	}
	return n
}

func TestFlattenEntriesOpensEachSubmenuUnderALabel(t *testing.T) {
	sub := []ui.MenuEntry{{Label: "Work"}}
	nested := []ui.MenuEntry{{Kind: ui.MenuSubTrigger, Label: "Codex", Sub: sub}}
	got := flattenEntries([]ui.MenuEntry{{Label: "Personal"}, {Kind: ui.MenuSubTrigger, Label: "Other providers", Sub: nested}})
	var labels []string
	for _, e := range got {
		labels = append(labels, e.Label)
	}
	if len(got) != 3 || labels[0] != "Personal" || labels[1] != "Codex" || labels[2] != "Work" || got[1].Kind != ui.MenuLabel {
		t.Errorf("flattened to %v, want Personal, a Codex label and Work, with no label for Other providers", labels)
	}
}

func TestEveryAnswerToASpendHasItsOwnToast(t *testing.T) {
	now := epoch
	cases := []struct {
		name    string
		res     api.UseResetResult
		err     error
		variant ui.ToastVariant
		title   string
	}{
		{"reset", api.UseResetResult{Status: api.UseResetReset}, nil, ui.ToastSuccess, "Limit reset"},
		{"already used", api.UseResetResult{Status: api.UseResetAlreadyUsed}, nil, ui.ToastSuccess, "Limit reset"},
		{"not limited", api.UseResetResult{Status: api.UseResetNotLimited}, nil, ui.ToastInfo, "Nothing to reset"},
		{"ineligible", api.UseResetResult{Status: api.UseResetIneligible}, nil, ui.ToastWarning, "Claude is not offering this reset right now"},
		{"cooldown with a time", api.UseResetResult{Status: api.UseResetCooldown, CooldownUntil: resetAfter(12 * time.Minute)}, nil, ui.ToastWarning, "Resets are cooling down. Try again in 12m."},
		{"cooldown without", api.UseResetResult{Status: api.UseResetCooldown}, nil, ui.ToastWarning, "Resets are cooling down. Try again later."},
		{"error", api.UseResetResult{}, errors.New("sign in to Claude Code again to use resets"), ui.ToastError, "Could not use the reset: sign in to Claude Code again to use resets"},
	}
	for _, c := range cases {
		got := useResetToast(c.res, c.err, now)
		if got.Variant != c.variant || got.Title != c.title {
			t.Errorf("%s: toast %v %q, want %v %q", c.name, got.Variant, got.Title, c.variant, c.title)
		}
	}
}

func TestWhatAResetClearsIsSaidInWords(t *testing.T) {
	cases := []struct {
		r    backend.QuotaReset
		want string
	}{
		{backend.QuotaReset{Clears: []string{"session", "weekly", "model"}}, "Clears the session and weekly limits."},
		{backend.QuotaReset{Label: "Fable", Clears: []string{"session", "model"}}, "Clears the session limit and the weekly limit of Fable."},
		{backend.QuotaReset{Clears: []string{"session"}}, "Clears the session limit."},
		{backend.QuotaReset{}, ""},
	}
	for _, c := range cases {
		if got := resetClears(c.r); got != c.want {
			t.Errorf("%+v: %q, want %q", c.r.Clears, got, c.want)
		}
	}
}

// endsOn is a grant's end at noon, local time, on that day: the day the tooltip
// reads is the local one whatever zone the test runs in.
func endsOn(month time.Month, day int) string {
	return time.Date(2026, month, day, 12, 0, 0, 0, time.Local).UTC().Format(time.RFC3339)
}

func resetsPlan() backend.QuotaPlan {
	plan := usagePlan()
	plan.Resets = []backend.QuotaReset{
		{Kind: api.QuotaResetFull, Left: 1, EndsAt: endsOn(time.December, 10)},
		{Label: "Fable", Left: 2, EndsAt: endsOn(time.March, 9)},
		{Label: "Paused grant", Left: 1, EndsAt: endsOn(time.January, 2), Held: true},
	}
	return plan
}

func TestTheResetsBlockCountsEveryResetLeftAndListsEachGrant(t *testing.T) {
	r := tipFrames(t, resetsPlan())
	wantShown(t, r, "Resets", "4 left", "Full", "×1", " until Dec 10", "Fable", "×2", " until Mar 9", "Paused grant", " until Jan 2")
}

func TestTheResetsBlockHidesWhatIsNotLive(t *testing.T) {
	plan := resetsPlan()
	plan.Resets = []backend.QuotaReset{
		{Kind: api.QuotaResetFull, Left: 0, EndsAt: endsOn(time.December, 10)},
		{Label: "Fable", Left: 2, EndsAt: "1969-12-31T00:00:00Z"},
		{Label: "No end", Left: 1},
	}
	wantAbsent(t, tipFrames(t, plan), "Resets", "Fable")
	plan.Resets = nil
	wantAbsent(t, tipFrames(t, plan), "Resets")

	plan = resetsPlan()
	plan.Resets = plan.Resets[:1]
	wantShown(t, tipFrames(t, plan), "1 left")
	wantAbsent(t, tipFrames(t, plan), "Fable")
}

func TestAHeldGrantIsMutedAndAnActiveOneIsNot(t *testing.T) {
	live := liveResets(resetsPlan().Resets, epoch)
	if len(live) != 3 || live[0].Held || live[1].Held || !live[2].Held {
		t.Fatalf("grants %+v, want only the last held", live)
	}
}

func TestTheResetsBlockFollowsTheInterfaceLanguage(t *testing.T) {
	i18n.SetCurrent(i18n.PtBR)
	defer i18n.SetCurrent(i18n.EN)
	wantShown(t, tipFrames(t, resetsPlan()), "4 restantes", "Completo", " até 10 de dez", " até 9 de mar")
	if got := resetName(backend.QuotaReset{Label: "Fable"}); got != "Fable" {
		t.Errorf("a provider's name stays verbatim: %q", got)
	}
}

func TestSameQuotaSeesTheResetGrants(t *testing.T) {
	a, b := []backend.QuotaPlan{resetsPlan()}, []backend.QuotaPlan{resetsPlan()}
	if !sameQuota(a, b) {
		t.Fatal("equal grants differ")
	}
	b[0].Resets[1].Left = 1
	if sameQuota(a, b) {
		t.Error("a grant spent went unseen")
	}
}
