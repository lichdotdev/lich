package main

import (
	"testing"
	"time"

	"github.com/lichdotdev/lich/native/backend"
)

func TestHeldEditShowsTheSentValueUntilAReadCatchesUp(t *testing.T) {
	sent := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	const url = "https://github.com/o/r/pull/7"
	before := pullsRead[*backend.PRDetail]{loaded: true, at: sent.Add(-time.Minute)}
	cases := []struct {
		name     string
		current  string
		read     pullsRead[*backend.PRDetail]
		staledAt time.Time
		want     string
	}{
		{"the read from before the send", "Old", before, sent.Add(-time.Minute), "New"},
		{"a read in flight when the write's stale landed", "Old",
			pullsRead[*backend.PRDetail]{loaded: true, stale: true, at: sent.Add(2 * time.Second)}, sent.Add(time.Second), "New"},
		{"a read that equals what was sent", "New", before, sent.Add(-time.Minute), "New"},
		{"a read fetched after the write's stale, normalised by GitHub", "New\r\n",
			pullsRead[*backend.PRDetail]{loaded: true, at: sent.Add(2 * time.Second)}, sent.Add(time.Second), "New\r\n"},
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
	if got := h.over("https://github.com/o/r/pull/8", "Other", pullsRead[*backend.PRDetail]{}, time.Time{}); got != "Other" {
		t.Errorf("another pull request shows %q, want its own title", got)
	}
}

func TestHeldEditReleasedOnceCaughtUp(t *testing.T) {
	var h heldEdit
	sent := time.Now()
	h.hold("u", "New", sent)
	h.over("u", "New", pullsRead[*backend.PRDetail]{}, time.Time{})
	if got := h.over("u", "Renamed on GitHub", pullsRead[*backend.PRDetail]{}, time.Time{}); got != "Renamed on GitHub" {
		t.Errorf("after catching up shows %q, want the read", got)
	}
}

func TestEditSendsOnlyWhatChanged(t *testing.T) {
	d := backend.PRDetail{URL: "u", Title: "Fix the thing", Body: "The body."}
	cases := []struct {
		name, title, body string
		wantTitle         string
		wantBody          *string
	}{
		{"nothing changed", "Fix the thing", "The body.", "", nil},
		{"the title only", "Fix it", "The body.", "Fix it", nil},
		{"the title padded, not changed", "  Fix the thing ", "The body.", "", nil},
		{"a blank title keeps the old one", " ", "The body.", "", nil},
		{"the description emptied", "Fix the thing", "", "", new("")},
		{"both", "Fix it", "More.", "Fix it", new("More.")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var s editState
			s.start(d)
			s.title.SetText(c.title)
			s.body.SetText(c.body)
			title, body := s.changes(d)
			if title != c.wantTitle {
				t.Errorf("title = %q, want %q", title, c.wantTitle)
			}
			if (body == nil) != (c.wantBody == nil) || body != nil && *body != *c.wantBody {
				t.Errorf("body = %v, want %v", body, c.wantBody)
			}
		})
	}
}

func TestEditSaveWithNothingChangedJustCloses(t *testing.T) {
	d := backend.PRDetail{URL: "u", Title: "Fix the thing", Body: "The body."}
	var s editState
	s.start(d)
	var act pullsAction
	s.submit(d, time.Now(), &act)
	if s.editing(d) || act.title != "" || act.body != nil {
		t.Errorf("editing = %v, act = %+v; want closed with nothing sent", s.editing(d), act)
	}
}
