package quota

import (
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
)

// paceNow is the clock every case here is written against; markAhead takes it
// as a parameter, which is the whole reason none of this needs a real one.
var paceNow = time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)

// resetIn spells a window's next reset as the providers report it: RFC 3339,
// that many seconds after paceNow.
func resetIn(seconds int) string {
	return paceNow.Add(time.Duration(seconds) * time.Second).Format(time.RFC3339)
}

// The boundaries are pinned as literals on purpose: a test that reads paceGrace
// keeps passing after someone edits it, which is the exact change it exists to
// catch. 604800 is a week, 302400 half of one and 86400 the grace. paceNow is
// 2026-09-04T12:00:00Z, so a literal RunsOutAt is that plus the seconds the
// projection computes by hand: at half the clock and 51% spent, the 49% left
// lasts 302400*49/51 = 290541 seconds.
func TestProjectRunOut(t *testing.T) {
	tests := []struct {
		name   string
		window Window
		want   string
	}{{
		name:   "weekly spending exactly on its clock lasts to the reset, not past it",
		window: Window{Seconds: 604800, Percent: 50, ResetsAt: resetIn(302400)},
		want:   "",
	}, {
		name:   "weekly one point faster than its clock runs out before the reset",
		window: Window{Seconds: 604800, Percent: 51, ResetsAt: resetIn(302400)},
		want:   "2026-09-07T20:42:21Z",
	}, {
		name:   "weekly spending under its own clock",
		window: Window{Seconds: 604800, Percent: 40, ResetsAt: resetIn(302400)},
		want:   "",
	}, {
		name:   "a second short of the grace, spent fast",
		window: Window{Seconds: 604800, Percent: 20, ResetsAt: resetIn(604800 - 86399)},
		want:   "",
	}, {
		name:   "the grace exactly elapsed, spent fast: 20% a day empties it in four more",
		window: Window{Seconds: 604800, Percent: 20, ResetsAt: resetIn(604800 - 86400)},
		want:   "2026-09-08T12:00:00Z",
	}, {
		name:   "nothing spent has no rate to project",
		window: Window{Seconds: 604800, Percent: 0, ResetsAt: resetIn(302400)},
		want:   "",
	}, {
		name:   "a spent window has already run out",
		window: Window{Seconds: 604800, Percent: 100, ResetsAt: resetIn(302400)},
		want:   "",
	}, {
		name:   "the five-hour window is never projected",
		window: Window{Seconds: 18000, Percent: 90, ResetsAt: resetIn(3600)},
		want:   "",
	}, {
		// 2592000 is Codex's 30-day window: past the grace and spent faster than
		// its clock, so only the weekly-only guard keeps it unprojected.
		name:   "the free tier's monthly window is not weekly, so it is not projected",
		window: Window{Seconds: 2592000, Percent: 90, ResetsAt: resetIn(1296000)},
		want:   "",
	}, {
		name:   "a model-scoped weekly cap is projected like the account-wide one",
		window: Window{Label: "Fable", Seconds: 604800, Percent: 51, ResetsAt: resetIn(302400)},
		want:   "2026-09-07T20:42:21Z",
	}, {
		name:   "a reset three cycles further out still places the current one",
		window: Window{Seconds: 604800, Percent: 51, ResetsAt: resetIn(302400 + 3*604800)},
		want:   "2026-09-07T20:42:21Z",
	}, {
		name:   "a reset already in the past projects nothing",
		window: Window{Seconds: 604800, Percent: 60, ResetsAt: resetIn(-3600)},
		want:   "",
	}, {
		name:   "no reset time, nothing to project against",
		window: Window{Seconds: 604800, Percent: 60},
		want:   "",
	}, {
		name:   "an unparseable reset time projects nothing",
		window: Window{Seconds: 604800, Percent: 60, ResetsAt: "next tuesday"},
		want:   "",
	}, {
		name:   "a window whose length the provider never reported",
		window: Window{Percent: 60, ResetsAt: resetIn(302400)},
		want:   "",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := projectRunOut(tt.window, paceNow)
			if got.RunsOutAt != tt.want {
				t.Errorf("RunsOutAt = %q, want %q", got.RunsOutAt, tt.want)
			}
			got.RunsOutAt = tt.window.RunsOutAt
			if got != tt.window {
				t.Errorf("projectRunOut changed more than the projection: %+v, want %+v", got, tt.window)
			}
		})
	}
}

func TestPaceProjectsEveryWindowOfEveryPlan(t *testing.T) {
	plans := []Plan{{
		Provider: "claude",
		Windows: []Window{
			{Kind: api.QuotaWindowSession, Seconds: 18000, Percent: 90, ResetsAt: resetIn(3600)},
			{Kind: api.QuotaWindowWeekly, Seconds: 604800, Percent: 51, ResetsAt: resetIn(302400)},
		},
	}, {
		Provider: "codex",
		Windows: []Window{
			{Kind: api.QuotaWindowWeekly, Seconds: 604800, Percent: 40, ResetsAt: resetIn(302400)},
		},
	}}
	pace(plans, paceNow)

	want := []string{"", "2026-09-07T20:42:21Z", ""}
	got := []string{
		plans[0].Windows[0].RunsOutAt,
		plans[0].Windows[1].RunsOutAt,
		plans[1].Windows[0].RunsOutAt,
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("projections = %q, want %q", got, want)
		}
	}
}
