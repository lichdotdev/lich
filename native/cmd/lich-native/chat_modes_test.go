package main

import (
	"testing"

	"github.com/lichdotdev/lich/native/backend"
)

func TestEveryModeIsNamedInTheWindowsWords(t *testing.T) {
	cases := []struct {
		kind, id, label, want string
	}{
		{"claude", "default", "", "Manual"},
		{"claude", "acceptEdits", "", "Accept edits"},
		{"claude", "dontAsk", "", "Pre-approved only"},
		{"claude", "bypassPermissions", "", "YOLO"},
		{"codex", ":read-only", "", "Read only"},
		{"antigravity", "default", "Default", "Manual"},
		{"antigravity", "accept-edits", "Accept edits", "Accept edits"},
		{"omp", "default", "Default", "Default"},
		{"cursor", "ask", "Ask", "Ask"},
		{"opencode", "build", "build", "Build"},
		{"kiro", "kiro_planner", "kiro_planner", "Planner"},
		{"opencode", "cavecrew-builder", "cavecrew-builder", "cavecrew-builder"},
		{"kiro", "lich", "lich", "lich"},
		{"opencode", "ask", "ask", "ask"},
	}
	for _, c := range cases {
		if got := modeLabel(c.kind, backend.ChatChoice{ID: c.id, Label: c.label}); got != c.want {
			t.Errorf("%s %s is %q, want %q", c.kind, c.id, got, c.want)
		}
	}
}
