//go:build !windows

package chat

import (
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestCleanTitle(t *testing.T) {
	cases := map[string]string{
		"Fix login redirect":                 "Fix login redirect",
		"  \"Fix login redirect.\"\n\nextra": "Fix login redirect",
		"「ログイン修正」。":                          "ログイン修正",
		"":                                   "",
		strings.Repeat("a", 100):             strings.Repeat("a", titleRunes),
	}
	for in, want := range cases {
		if got := cleanTitle(in); got != want {
			t.Errorf("cleanTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func (r *rig) titles() []string {
	var out []string
	for _, e := range r.snapshot() {
		if s, ok := e.data.(api.SessionTitle); ok && e.name == api.SessionTitleEvent.Name {
			out = append(out, s.Label)
		}
	}
	return out
}

// TestClaudeNamesTheSessionOnce: the first turn's end runs Claude Code's print mode
// without the hooks' variables, the card takes the cleaned title, and the next turn
// asks nothing.
func TestClaudeNamesTheSessionOnce(t *testing.T) {
	r := newClaudeChat(t)
	r.prompt("hi")
	r.eventually("title", func() bool { return len(r.titles()) == 1 })
	if got := r.titles(); got[0] != "Fake title" {
		t.Fatalf("titles = %q, want the cleaned one", got)
	}
	if label := r.db.SessionLabel("s1"); label != "Fake title" {
		t.Errorf("label = %q", label)
	}
	r.prompt("again")
	r.settled()
	if n := strings.Count(r.fakeLog(), "--no-session-persistence"); n != 1 {
		t.Errorf("print mode ran %d times, want 1", n)
	}
}

// TestNoTitleAfterEnvelope: a turn lich's relay opened names nothing.
func TestNoTitleAfterEnvelope(t *testing.T) {
	r := newClaudeChat(t)
	r.take(testEnvelope)
	r.turnEnd(0)
	r.settled()
	if got := r.titles(); len(got) != 0 {
		t.Errorf("titles = %q, want none", got)
	}
}
