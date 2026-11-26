package spawn

import (
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/internal/providers"
)

// A fork branches the caller's conversation on its first start, as its
// terminal in the caller's checkout, on every provider that can branch one; the others are
// refused before any session exists.
func TestOpenForksTheCallersConversation(t *testing.T) {
	for _, tc := range []struct {
		kind  string
		forks bool
	}{
		{providers.Claude, true}, {providers.Codex, true}, {providers.OpenCode, true},
		{providers.Antigravity, false}, {providers.OMP, false}, {providers.Crush, false},
		{providers.Cursor, false}, {providers.Kiro, false},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			svc, sessions, _, term, _ := newService(t)
			sessions.projects[0].Sessions[0].Kind = tc.kind
			sessions.projects[0].Sessions[0].ProviderSessionID = "conv-1"
			sessions.projects[0].Sessions[0].Path = "/wt/auth-fix"

			opened, err := svc.Open(t.Context(), OpenOptions{From: "s1", Fork: true})
			if !tc.forks {
				if err == nil || !strings.Contains(err.Error(), "cannot fork a conversation") || len(sessions.rows) != 0 {
					t.Fatalf("err = %v with rows %+v, want the fork refused before a row", err, sessions.rows)
				}
				return
			}
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			if !slices.Equal(sessions.forked, []string{opened.ID}) {
				t.Errorf("forked rows %q, want the successor's", sessions.forked)
			}
			if len(term.spawns) != 1 || term.spawns[0].resume != "conv-1" || !term.spawns[0].fork || term.spawns[0].cwd != "/wt/auth-fix" {
				t.Errorf("spawns %+v, want one branching conv-1", term.spawns)
			}
		})
	}
}

// A fork with nothing it can branch, or told to change what a fork cannot,
// is refused before a session exists.
func TestOpenRefusesAForkItCannotMake(t *testing.T) {
	for _, tc := range []struct {
		name, conversation string
		gone               bool
		opts               OpenOptions
		want               string
	}{
		{"no conversation yet", "", false, OpenOptions{From: "s1", Fork: true}, "has no conversation"},
		{"a pruned conversation", "conv-1", true, OpenOptions{From: "s1", Fork: true}, "has no conversation"},
		{"a model", "conv-1", false, OpenOptions{From: "s1", Fork: true, Model: "o3"}, "leave kind, model, effort and ultracode out"},
		{"another provider", "conv-1", false, OpenOptions{From: "s1", Fork: true, Kind: "claude"}, "leave kind, model, effort and ultracode out"},
		{"no caller", "conv-1", false, OpenOptions{From: "nobody", Fork: true}, "run it from inside the session to fork"},
		{"another project", "conv-1", false, OpenOptions{From: "s1", Fork: true, Project: "/src/other"}, "in your own project"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, sessions, _, term, _ := newService(t)
			sessions.projects[0].Sessions[0].ProviderSessionID = tc.conversation
			term.gone = map[string]bool{"conv-1": tc.gone}

			_, err := svc.Open(t.Context(), tc.opts)
			if err == nil || !strings.Contains(err.Error(), tc.want) || len(sessions.rows) != 0 {
				t.Fatalf("err = %v with rows %+v, want one saying %q and no row", err, sessions.rows, tc.want)
			}
		})
	}
}
