package main

import (
	"slices"
	"testing"

	"github.com/lichdotdev/lich/native/backend"
)

func TestHasConflictMarkers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, patch string
		want        bool
	}{
		{"markers added", "@@ -1 +1,5 @@\n+<<<<<<< HEAD\n+a\n+=======\n+b\n+>>>>>>> side\n", true},
		{"a separator alone still counts", "@@ -1 +1,2 @@\n a\n+=======\n", true},
		{"resolved file", "@@ -1 +1,2 @@\n a\n+b\n", false},
		{"markers only in context or removed", "@@ -1,2 +1 @@\n <<<<<<< HEAD\n-=======\n", false},
		{"a longer run of equals is code", "@@ -1 +1 @@\n+========\n", false},
	}
	for _, tt := range tests {
		if got := hasConflictMarkers(tt.patch); got != tt.want {
			t.Errorf("%s: hasConflictMarkers = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestConflictsFirstKeepsGitOrder(t *testing.T) {
	t.Parallel()
	order := []string{"a.go", "b.go", "c.go", "d.go"}
	conflictsFirst(order, []string{"d.go", "b.go"})
	if want := []string{"b.go", "d.go", "a.go", "c.go"}; !slices.Equal(order, want) {
		t.Errorf("order = %q, want %q", order, want)
	}
}

func TestReviewOffersNoRevertMidMerge(t *testing.T) {
	t.Parallel()
	clean := &reviewPanel{}
	if !clean.reverts("a.go") {
		t.Error("a checkout with no merge offers no revert")
	}
	rebasing := &reviewPanel{git: backend.DiffStats{Conflicted: []string{"a.go"}}}
	if rebasing.reverts("a.go") || !rebasing.reverts("b.go") || !rebasing.conflicted("a.go") {
		t.Error("outside a merge, only the conflicted file should lose its revert")
	}
	merging := &reviewPanel{git: backend.DiffStats{Merging: "side", Conflicted: []string{"a.go"}}}
	if merging.reverts("b.go") {
		t.Error("a file the merge brought in clean offers a revert")
	}
	pr := &reviewPanel{pr: &prFiles{}, git: backend.DiffStats{Conflicted: []string{"a.go"}}}
	if pr.conflicted("a.go") {
		t.Error("a pull request's files read the checkout's conflicts")
	}
}

func TestCheckoutSlotNamesTheMerge(t *testing.T) {
	t.Parallel()
	in := footerInput{path: "/repo", isGit: true, git: backend.DiffStats{Branch: "master", Merging: "fix-post-title"}}
	s, ok := footerSlotOf("checkout", in)
	if want := "master · merging fix-post-title"; !ok || s.Text != want || s.Hint != want {
		t.Errorf("checkout slot = %q (hint %q), want %q", s.Text, s.Hint, want)
	}
	in.git.Merging = ""
	if s, _ := footerSlotOf("checkout", in); s.Text != "master" {
		t.Errorf("checkout slot with no merge = %q, want master", s.Text)
	}
}

func TestSameDiffSeesTheConflicts(t *testing.T) {
	t.Parallel()
	a := backend.DiffStats{Files: 1, Merging: "side", Conflicted: []string{"a.go"}}
	b := a
	b.Conflicted = nil
	if sameDiff(a, b) || !sameDiff(a, a) {
		t.Error("sameDiff misses a file leaving the conflict list")
	}
}
