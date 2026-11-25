package store

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
)

const (
	viewedPR    = "https://github.com/acme/app/pull/7"
	viewedOther = "https://gitlab.com/acme/app/-/merge_requests/7"
)

func viewedFiles(t *testing.T, svc *Service, pr string) []api.ViewedFile {
	t.Helper()
	got, err := svc.ViewedFiles(context.Background(), api.ViewedFilesOptions{PR: pr})
	if err != nil {
		t.Fatalf("ViewedFiles: %v", err)
	}
	return got
}

func TestViewedFilesAreKeptPerPullRequest(t *testing.T) {
	svc := newTestStore(t)
	at := time.Unix(1_800_000_000, 0)
	for _, m := range []api.MarkViewedOptions{
		{PR: viewedPR, Path: "b.go", Hash: "h1"},
		{PR: viewedPR, Path: "a.go", Hash: "h2"},
		{PR: viewedOther, Path: "a.go", Hash: "h3"},
	} {
		if err := svc.markViewed(m, at); err != nil {
			t.Fatal(err)
		}
	}
	want := []api.ViewedFile{{Path: "a.go", Hash: "h2", ViewedAt: at.Unix()}, {Path: "b.go", Hash: "h1", ViewedAt: at.Unix()}}
	if got := viewedFiles(t, svc, viewedPR); !slices.Equal(got, want) {
		t.Errorf("ViewedFiles = %+v, want %+v", got, want)
	}
	if got := viewedFiles(t, svc, "https://github.com/acme/app/pull/8"); got == nil || len(got) != 0 {
		t.Errorf("a pull request never ticked = %#v, want an empty list", got)
	}
}

func TestMarkViewedAgainReplacesTheHashAndTime(t *testing.T) {
	svc := newTestStore(t)
	if err := svc.markViewed(api.MarkViewedOptions{PR: viewedPR, Path: "a.go", Hash: "old"}, time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.MarkViewed(context.Background(), api.MarkViewedOptions{PR: viewedPR, Path: "a.go", Hash: "new"}); err != nil {
		t.Fatal(err)
	}
	got := viewedFiles(t, svc, viewedPR)
	if len(got) != 1 || got[0].Hash != "new" || got[0].ViewedAt <= 100 {
		t.Errorf("ViewedFiles = %+v, want one row hashed new and stamped now", got)
	}
}

func TestUnmarkViewedDeletesOnlyThatFile(t *testing.T) {
	svc := newTestStore(t)
	ctx := context.Background()
	for _, m := range []api.MarkViewedOptions{{PR: viewedPR, Path: "a.go", Hash: "h"}, {PR: viewedOther, Path: "a.go", Hash: "h"}} {
		if _, err := svc.MarkViewed(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if _, err := svc.UnmarkViewed(ctx, api.UnmarkViewedOptions{PR: viewedPR, Path: "a.go"}); err != nil {
			t.Fatalf("UnmarkViewed: %v", err)
		}
	}
	if got := viewedFiles(t, svc, viewedPR); len(got) != 0 {
		t.Errorf("after untick = %+v, want none", got)
	}
	if got := viewedFiles(t, svc, viewedOther); len(got) != 1 {
		t.Errorf("the other pull request = %+v, want its tick kept", got)
	}
}

// A pull request is pruned by its latest tick, so one ticked again inside the
// window keeps the ticks it got months ago.
func TestPruneViewedFilesDropsPullRequestsLeftAlone(t *testing.T) {
	svc := newTestStore(t)
	now := time.Unix(1_800_000_000, 0)
	old, recent := now.Add(-viewedFilesTTL-time.Hour), now.Add(-time.Hour)
	for _, r := range []struct {
		opts api.MarkViewedOptions
		at   time.Time
	}{
		{api.MarkViewedOptions{PR: viewedOther, Path: "a.go", Hash: "h"}, old},
		{api.MarkViewedOptions{PR: viewedPR, Path: "a.go", Hash: "h"}, old},
		{api.MarkViewedOptions{PR: viewedPR, Path: "b.go", Hash: "h"}, recent},
	} {
		if err := svc.markViewed(r.opts, r.at); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.pruneViewedFiles(now); err != nil {
		t.Fatal(err)
	}
	if got := viewedFiles(t, svc, viewedOther); len(got) != 0 {
		t.Errorf("left alone past the TTL = %+v, want pruned", got)
	}
	if got := viewedFiles(t, svc, viewedPR); len(got) != 2 {
		t.Errorf("ticked recently = %+v, want both ticks kept", got)
	}
}
