package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/backend"
)

func TestGitMergeItem(t *testing.T) {
	t.Parallel()
	clean := backend.DiffStats{Branch: "master"}
	ready := mergePreview{done: true, Commits: 2}
	none := mergePreview{done: true}
	cases := []struct {
		name    string
		row     gitTreeRow
		cur     backend.DiffStats
		preview mergePreview
		offered bool
		blocked gitBlocked
	}{
		{"a local branch", gitTreeRow{}, clean, ready, true, gitFree},
		{"a remote branch", gitTreeRow{remote: true}, clean, ready, true, gitFree},
		{"a branch held by another worktree", gitTreeRow{worktree: "/wt"}, clean, ready, true, gitFree},
		{"still reading the preview", gitTreeRow{}, clean, mergePreview{}, true, gitFree},
		{"a preview that failed", gitTreeRow{}, clean, mergePreview{done: true, failed: true}, true, gitFree},
		{"nothing to bring", gitTreeRow{}, clean, none, true, gitMerged},
		{"uncommitted changes here", gitTreeRow{}, backend.DiffStats{Files: 1}, ready, true, gitDirtyHere},
		{"a merge already running", gitTreeRow{}, backend.DiffStats{Files: 1, Merging: "x"}, ready, true, gitMergeRunning},
		{"the branch HEAD is on", gitTreeRow{head: true}, clean, ready, false, gitFree},
		{"a tag", gitTreeRow{tag: true}, clean, ready, false, gitFree},
	}
	for _, c := range cases {
		it, ok := gitMergeItem(c.row, c.cur, c.preview)
		if ok != c.offered {
			t.Errorf("%s: offered = %v, want %v", c.name, ok, c.offered)
			continue
		}
		if ok && (it.act != gitActMerge || it.blocked != c.blocked) {
			t.Errorf("%s: item = %+v, want merge blocked %v", c.name, it, c.blocked)
		}
	}
}
