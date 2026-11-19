package main

import (
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

// tipFrames draws a plan's tooltip body alone.
func tipFrames(t *testing.T, plan backend.QuotaPlan) *frameRig {
	t.Helper()
	r := newFrameRig(image.Pt(400, 700), func(gtx C, th *ui.Theme) { planDetails(th.On(th.Popover), plan, gtx.Now)(gtx) })
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

func TestTheProfileTooltipSaysWhyThereIsNoUsage(t *testing.T) {
	draw := func(f profileFocus, plan backend.QuotaPlan, loaded bool) *frameRig {
		r := newFrameRig(image.Pt(400, 300), func(gtx C, th *ui.Theme) {
			if body := profileUsageTip(th.On(th.Popover), f, plan, loaded, gtx.Now); body != nil {
				body(gtx)
			}
		})
		r.settle(t)
		return r
	}
	claude := profileFocus{book: &profileBook{provider: claudeProfileProvider}}
	omp := profileFocus{book: &profileBook{provider: ompProfileProvider}}

	cursor := profileFocus{book: &profileBook{provider: cursorProfileProvider}}

	wantShown(t, draw(omp, backend.QuotaPlan{}, false), "Usage is not measured per profile for oh-my-pi.")
	// Cursor CLI has a plan, but lich reads it for the machine's login alone: another profile's tooltip never draws it.
	wantShown(t, draw(cursor, usagePlan(), true), "Usage is not measured per profile for Cursor CLI.")
	wantAbsent(t, draw(cursor, usagePlan(), true), "Session · 5h")
	wantShown(t, draw(claude, backend.QuotaPlan{Status: "signed-out"}, true), "Signed out.")
	wantShown(t, draw(claude, backend.QuotaPlan{Status: "error"}, true), "Plan usage could not be read.")
	wantShown(t, draw(claude, backend.QuotaPlan{Status: "unknown"}, true), "Plan usage could not be read.")
	if body := profileUsageTip(new(ui.Theme), claude, backend.QuotaPlan{}, false, epoch); body != nil {
		t.Error("a reading still in flight opened a tooltip")
	}
	if body := profileUsageTip(new(ui.Theme), claude, usagePlan(), true, epoch); body == nil {
		t.Error("a read plan has no tooltip")
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
// plan seeded for the machine's login.
func profileFrames(t *testing.T) (*model, *frameRig) {
	t.Helper()
	_, m, _ := profileFooterRig(t)
	root := new(ui.Root)
	s := newSidebar(root)
	s.open = true
	r := newFrameRig(image.Pt(400, 700), func(gtx C, th *ui.Theme) {
		root.Layout(gtx, func(gtx C) D {
			s.layout(gtx, th, m, active(m))
			return D{Size: gtx.Constraints.Max}
		})
	})
	return m, r
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

func TestHoveringTheProfileFooterShowsItsAccountsUsage(t *testing.T) {
	m, r := profileFrames(t)
	plan := usagePlan()
	plan.Extra = &backend.QuotaExtra{Used: 1250, Limit: new(int64(5000)), Currency: "USD", Decimals: 2}
	m.quota.seed("", []backend.QuotaPlan{plan})
	r.settle(t)
	wantAbsent(t, r, "Session · 5h")
	hoverProfile(t, r)
	wantShown(t, r, "Claude Code · Max 5x", "Session · 5h", "Weekly · 7d", "Extra usage", "$12.50 of $50.00", "dev@example.com")
}

func TestTheProfileFooterOpensNothingBeforeTheFirstReading(t *testing.T) {
	_, r := profileFrames(t)
	r.settle(t)
	hoverProfile(t, r)
	wantAbsent(t, r, "Session · 5h", "Plan usage could not be read.")
}
