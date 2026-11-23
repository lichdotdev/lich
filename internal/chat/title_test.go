//go:build !windows

package chat

import (
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
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

// TestTitleOnOtherProviders: oh-my-pi is named by its own print mode; opencode's
// session is deleted once its JSON events are read; Kiro CLI is out, and its
// chat is asked nothing.
func TestTitleOnOtherProviders(t *testing.T) {
	for _, tc := range []struct {
		kind string
		want []string
	}{
		{providers.OMP, []string{"Fake title"}},
		{providers.OpenCode, []string{"Fake title"}},
		{providers.Kiro, nil},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			r := newRig(t, tc.kind)
			if err := r.setView(api.ViewChat); err != nil {
				t.Fatalf("SetView: %v", err)
			}
			r.prompt("hi")
			if tc.want == nil {
				r.settled()
				if got := r.titles(); len(got) != 0 {
					t.Fatalf("titles = %q, want none", got)
				}
				return
			}
			r.eventually("title", func() bool { return len(r.titles()) == 1 })
			if got := r.titles(); got[0] != tc.want[0] {
				t.Errorf("titles = %q, want %q", got, tc.want)
			}
			if tc.kind == providers.OpenCode {
				r.eventually("the session deleted", func() bool { return strings.Contains(r.fakeLog(), "session delete ses_fake") })
			}
		})
	}
}
