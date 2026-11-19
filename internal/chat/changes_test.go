package chat

import (
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/project"
)

func lines(head string, files map[string]project.FileLines) *project.Lines {
	return &project.Lines{Head: head, Files: files}
}

func TestTurnChanges(t *testing.T) {
	t.Parallel()
	was := lines("h1", map[string]project.FileLines{
		"dirty.go": {Added: 5, Removed: 1},
		"gone.go":  {Added: 2},
		"same.go":  {Added: 3},
	})
	tests := []struct {
		name          string
		before, after *project.Lines
		want          *api.ChatTurnChanges
	}{
		{"nothing moved", was, was, &api.ChatTurnChanges{}},
		{"a file grew and one is new", was, lines("h1", map[string]project.FileLines{
			"dirty.go": {Added: 8, Removed: 1}, "gone.go": {Added: 2}, "same.go": {Added: 3}, "new.go": {Added: 4},
		}), &api.ChatTurnChanges{Files: 2, Added: 7}},
		{"a file went back to HEAD", was, lines("h1", map[string]project.FileLines{
			"dirty.go": {Added: 5, Removed: 1}, "same.go": {Added: 3},
		}), &api.ChatTurnChanges{Files: 1}},
		{"lines were removed", was, lines("h1", map[string]project.FileLines{
			"dirty.go": {Added: 5, Removed: 4}, "gone.go": {Added: 2}, "same.go": {Added: 3},
		}), &api.ChatTurnChanges{Files: 1, Removed: 3}},
		{"a binary changed state", lines("h1", nil), lines("h1", map[string]project.FileLines{"img.png": {Binary: true}}), &api.ChatTurnChanges{Files: 1}},
		{"the head moved", was, lines("h2", nil), nil},
		{"no reading at the start", nil, was, nil},
		{"no reading at the end", was, nil, nil},
	}
	for _, tc := range tests {
		got := turnChanges(tc.before, tc.after)
		switch {
		case got == nil && tc.want == nil:
		case got == nil || tc.want == nil || *got != *tc.want:
			t.Errorf("%s: turnChanges = %+v, want %+v", tc.name, got, tc.want)
		}
	}
}
