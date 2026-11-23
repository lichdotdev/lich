package main

import (
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/i18n"
)

func TestCompactCountReadsInAShortForm(t *testing.T) {
	for n, want := range map[int]string{
		0: "0", 999: "999", 1000: "1K", 1500: "1.5K", 585653: "585.7K", 999949: "999.9K", 999950: "1M",
		1_000_000: "1M", 1_049_000: "1M", 2_300_000: "2.3M", 1_000_000_000: "1B",
	} {
		if got := compactCount(n, i18n.EN); got != want {
			t.Errorf("compactCount(%d) = %q, want %q", n, got, want)
		}
	}
	if got := compactCount(585653, i18n.PtBR); got != "585,7K" {
		t.Errorf("in Portuguese %q, want a decimal comma", got)
	}
	if got := compactCount(585653, i18n.JA); got != "585.7K" {
		t.Errorf("in Japanese %q, want a decimal point", got)
	}
}

func TestTheContextBarChangesToneAtSeventyAndNinety(t *testing.T) {
	for percent, want := range map[int]usageTone{
		0: toneMuted, 70: toneMuted, 71: toneWait, 90: toneWait, 91: toneDestructive, 100: toneDestructive,
	} {
		if got := contextBarTone(percent); got != want {
			t.Errorf("%d%% is tone %d, want %d", percent, got, want)
		}
	}
}

func TestTheContextTooltipHasLinesOnlyForWhatItKnows(t *testing.T) {
	u := sessionUsage{Percent: 58, Tokens: 585653, Window: 1_000_000}
	labels := func(rows []contextRow) (out []string) {
		for _, r := range rows {
			out = append(out, r.label)
		}
		return out
	}
	rows := contextRows(u, i18n.EN)
	if got := labels(rows); len(got) != 2 || got[0] != "Tokens" || got[1] != "Model window" {
		t.Errorf("lines %v with no cache and no cost, want Tokens and Model window", got)
	}
	if rows[0].value != "585.7K" || rows[0].rest != " / 1M" || rows[1].value != "1M" {
		t.Errorf("tokens %+v, window %+v", rows[0], rows[1])
	}
	u.Cache = &api.SessionCache{Read: 100}
	u.CacheReads = 124_600_000
	u.CostUSD = new(11.39)
	rows = contextRows(u, i18n.EN)
	if got := labels(rows); len(got) != 5 || got[2] != "Cache" || got[3] != "Cache reads" || got[4] != "Session cost" {
		t.Fatalf("lines %v with cache, cache reads and cost", got)
	}
	if rows[3].value != "124.6M" {
		t.Fatalf("cache reads %q, want 124.6M", rows[3].value)
	}
	if rows[2].value != "100% hit" || rows[4].value != "$11.39" {
		t.Errorf("cache %q, cost %q", rows[2].value, rows[4].value)
	}
}

// hoverRing brings the pointer to the context ring and waits for its tooltip.
func hoverRing(t *testing.T, r *frameRig) {
	t.Helper()
	p := r.at(t, "42%")
	r.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(p.X, p.Y)})
	r.frame()
	r.now = r.now.Add(time.Second)
	r.frame()
	r.settle(t)
}

func wantContextTip(t *testing.T, r *frameRig) {
	t.Helper()
	for _, label := range []string{"Context", "Tokens", "Model window", "200K", "Session cost", "$0.500"} {
		if !r.shows(label) {
			t.Errorf("the context tooltip lacks %q:\n%s", label, r.drawn())
		}
	}
	if r.shows("Until compaction") || r.shows("Compacted") {
		t.Error("the tooltip says what lich does not know")
	}
}

func TestTheContextTooltipOverTheRingInTheComposer(t *testing.T) {
	_, r, _ := chatFrames(t, api.ViewChat)
	r.settle(t)
	hoverRing(t, r)
	wantContextTip(t, r)
}

func TestTheContextTooltipOverTheRingInTheTerminalsBalloon(t *testing.T) {
	_, r := balloonFrames(t, busySession())
	r.settle(t)
	hoverRing(t, r)
	wantContextTip(t, r)
}

func TestTheComposerHasNoRingBeforeTheSessionReportsAWindow(t *testing.T) {
	in := busySession()
	in.usage.Window, in.hasUsage = 0, true
	_, r, _ := chatFramesIn(t, api.ViewChat, rigWindow, in)
	r.settle(t)
	if r.shows("42%") {
		t.Error("a ring drawn with no window to be a share of")
	}
}
