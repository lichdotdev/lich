package project

import (
	"slices"
	"testing"
)

func TestToReviewers(t *testing.T) {
	t.Parallel()
	t.Run("the two halves merge into one roster", func(t *testing.T) {
		reviewers := toReviewers(
			[]ghLatestReview{{Author: ghPRAuthor{Login: "ana"}, State: "APPROVED"}},
			[]ghReviewRequest{{TypeName: "User", Login: "bo"}},
		)
		want := []PRReviewer{
			{Login: "ana", State: "APPROVED"},
			{Login: "bo"},
		}
		if !slices.Equal(reviewers, want) {
			t.Errorf("wrong roster: %+v", reviewers)
		}
	})

	// The one case the merge exists for: GitHub keeps the old verdict in
	// latestReviews when a reviewer is asked again, and what stands is the answer
	// now owed — a roster still reading "approved" would hide the re-request.
	t.Run("a re-request outranks the verdict it follows", func(t *testing.T) {
		reviewers := toReviewers(
			[]ghLatestReview{{Author: ghPRAuthor{Login: "ana"}, State: "CHANGES_REQUESTED"}},
			[]ghReviewRequest{{TypeName: "User", Login: "ana"}},
		)
		if len(reviewers) != 1 {
			t.Fatalf("expected one entry, got %+v", reviewers)
		}
		if reviewers[0].Login != "ana" || reviewers[0].State != "" {
			t.Errorf("expected ana pending, got %+v", reviewers[0])
		}
	})

	t.Run("a team rides its slug and is marked as one", func(t *testing.T) {
		reviewers := toReviewers(nil, []ghReviewRequest{{TypeName: "Team", Slug: "platform"}})
		if len(reviewers) != 1 || reviewers[0].Login != "platform" || !reviewers[0].IsTeam {
			t.Errorf("wrong team entry: %+v", reviewers)
		}
	})

	t.Run("a deleted account leaves no nameless entry", func(t *testing.T) {
		reviewers := toReviewers(
			[]ghLatestReview{{State: "APPROVED"}},
			[]ghReviewRequest{{TypeName: "User"}},
		)
		if reviewers != nil {
			t.Errorf("expected no roster, got %+v", reviewers)
		}
	})

	t.Run("nothing on either side is no roster", func(t *testing.T) {
		if reviewers := toReviewers(nil, nil); reviewers != nil {
			t.Errorf("expected nil, got %+v", reviewers)
		}
	})
}

func TestParsePRDetailReviewers(t *testing.T) {
	t.Parallel()
	out := []byte(`{
		"number": 7,
		"state": "OPEN",
		"author": {"login": "ana"},
		"latestReviews": [{"author": {"login": "bo"}, "state": "APPROVED"}],
		"reviewRequests": [{"__typename": "User", "login": "cai"}]
	}`)
	pr, err := parsePRDetail(out, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr == nil {
		t.Fatal("expected a detail, got nil")
	}
	if pr.Author != "ana" {
		t.Errorf("wrong author: %q", pr.Author)
	}
	want := []PRReviewer{{Login: "bo", State: "APPROVED"}, {Login: "cai"}}
	if !slices.Equal(pr.Reviewers, want) {
		t.Errorf("wrong roster: %+v", pr.Reviewers)
	}
}
