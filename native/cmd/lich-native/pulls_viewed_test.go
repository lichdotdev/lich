package main

import (
	"testing"
	"time"

	"github.com/lichdotdev/lich/native/backend"
)

// A card built when the Files tab opens takes the tick the store kept for its
// file: the same diff comes back ticked and folded, a moved one open and
// flagged, and a file never ticked as it always was.
func TestAKeptTickRestoresAgainstTheFilesDiff(t *testing.T) {
	f := onlyFile(t, twoHunks)
	for _, tc := range []struct {
		name         string
		kept         map[string]backend.ViewedFile
		open, viewed bool
		changedSince bool
	}{
		{"unchanged", map[string]backend.ViewedFile{f.path: {Path: f.path, Hash: diffHash(f.text())}}, false, true, false},
		{"changed", map[string]backend.ViewedFile{f.path: {Path: f.path, Hash: diffHash("the diff at the review")}}, true, false, true},
		{"never ticked", map[string]backend.ViewedFile{"other.go": {Path: "other.go", Hash: diffHash(f.text())}}, true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &reviewPanel{pr: &prFiles{viewed: &viewedTicks{loaded: true, files: tc.kept}}}
			c := newFileCard(p, f, false, 0)
			if c.open != tc.open || p.pr.isViewed(c) != tc.viewed || p.pr.changedSinceReview(c) != tc.changedSince {
				t.Errorf("open %v, viewed %v, changed since review %v; want %v, %v, %v",
					c.open, p.pr.isViewed(c), p.pr.changedSinceReview(c), tc.open, tc.viewed, tc.changedSince)
			}
		})
	}
}

func TestChangedSinceCountsMovedFilesAndDatesTheLastTick(t *testing.T) {
	ticks := &viewedTicks{files: map[string]backend.ViewedFile{
		"a.go": {Hash: "a1", ViewedAt: 100},
		"b.go": {Hash: "b0", ViewedAt: 300},
		"c.go": {Hash: "c0", ViewedAt: 200},
	}}
	now := map[string]string{"a.go": "a1", "b.go": "b1", "c.go": "c1", "d.go": "d0"}
	n, at := ticks.changedSince([]string{"a.go", "b.go", "c.go", "d.go"}, func(path string) string { return now[path] })
	if n != 2 || at.Unix() != 300 {
		t.Errorf("changedSince = %d, %v; want 2 files and the tick at 300", n, at.Unix())
	}
}

func TestReviewDayNamesTheYearOnlyWhenItIsNotThisOne(t *testing.T) {
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	for at, want := range map[time.Time]string{
		time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC): "Nov 22",
		time.Date(2025, 12, 30, 9, 0, 0, 0, time.UTC): "Dec 30, 2025",
	} {
		if got := reviewDay(at, now); got != want {
			t.Errorf("reviewDay(%v) = %q, want %q", at, got, want)
		}
	}
}
