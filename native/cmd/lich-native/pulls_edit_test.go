package main

import (
	"testing"
	"time"

	"github.com/lichdotdev/lich/native/lichclient"
)

func TestHeldEditShowsTheSentValueUntilAReadCatchesUp(t *testing.T) {
	sent := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	const url = "https://github.com/o/r/pull/7"
	before := pullsRead[*lichclient.PRDetail]{loaded: true, at: sent.Add(-time.Minute)}
	cases := []struct {
		name     string
		current  string
		read     pullsRead[*lichclient.PRDetail]
		staledAt time.Time
		want     string
	}{
		{"the read from before the send", "Old", before, sent.Add(-time.Minute), "New"},
		{"a read in flight when the write's stale landed", "Old",
			pullsRead[*lichclient.PRDetail]{loaded: true, stale: true, at: sent.Add(2 * time.Second)}, sent.Add(time.Second), "New"},
		{"a read that equals what was sent", "New", before, sent.Add(-time.Minute), "New"},
		{"a read fetched after the write's stale, normalised by GitHub", "New\r\n",
			pullsRead[*lichclient.PRDetail]{loaded: true, at: sent.Add(2 * time.Second)}, sent.Add(time.Second), "New\r\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var h heldEdit
			h.hold(url, "New", sent)
			if got := h.over(url, c.current, c.read, c.staledAt); got != c.want {
				t.Errorf("over = %q, want %q", got, c.want)
			}
		})
	}
}

func TestHeldEditStaysWithItsPullRequest(t *testing.T) {
	var h heldEdit
	h.hold("https://github.com/o/r/pull/7", "New", time.Now())
	if got := h.over("https://github.com/o/r/pull/8", "Other", pullsRead[*lichclient.PRDetail]{}, time.Time{}); got != "Other" {
		t.Errorf("another pull request shows %q, want its own title", got)
	}
}

func TestHeldEditReleasedOnceCaughtUp(t *testing.T) {
	var h heldEdit
	sent := time.Now()
	h.hold("u", "New", sent)
	h.over("u", "New", pullsRead[*lichclient.PRDetail]{}, time.Time{})
	if got := h.over("u", "Renamed on GitHub", pullsRead[*lichclient.PRDetail]{}, time.Time{}); got != "Renamed on GitHub" {
		t.Errorf("after catching up shows %q, want the read", got)
	}
}
