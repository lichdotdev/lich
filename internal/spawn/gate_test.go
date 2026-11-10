package spawn

import (
	"errors"
	"testing"
)

// A session the gate refuses must leave nothing behind: asked before the
// worktree is created, not only at the insert.
func TestARefusedSessionOpensNothing(t *testing.T) {
	refusal := errors.New("lich 0.10.0 is a required security update: run `yay -S lich-bin`")
	opens := map[string]func(*Service) error{
		"Open": func(svc *Service) error {
			_, err := svc.Open(t.Context(), OpenOptions{From: "s1"})
			return err
		},
		"Open on a new worktree": func(svc *Service) error {
			_, err := svc.Open(t.Context(), OpenOptions{From: "s1", Worktree: "auth-fix"})
			return err
		},
		"Open from outside a session": func(svc *Service) error {
			_, err := svc.Open(t.Context(), OpenOptions{Project: "revu", Kind: "claude"})
			return err
		},
		"OpenSubagent": func(svc *Service) error {
			_, err := svc.OpenSubagent(t.Context(), OpenSubagentOptions{From: "s1", Worktree: "auth-fix"})
			return err
		},
		"Handoff": func(svc *Service) error {
			_, err := svc.Handoff(t.Context(), HandoffOptions{From: "s1"})
			return err
		},
	}
	for name, open := range opens {
		t.Run(name, func(t *testing.T) {
			svc, sessions, worktrees, term, events := newService(t)
			sessions.refusal = refusal

			err := open(svc)

			if !errors.Is(err, refusal) {
				t.Fatalf("err = %v, want the gate's own refusal, which names how to update", err)
			}
			if len(worktrees.created) != 0 || len(sessions.rows) != 0 || len(term.spawns) != 0 || len(events.events) != 0 {
				t.Errorf("a refused session left a worktree %v, a row %v, a terminal %v or a card %v behind",
					worktrees.created, sessions.rows, term.spawns, events.events)
			}
		})
	}
}

func TestAShellOpensWhileAgentSessionsAreRefused(t *testing.T) {
	svc, sessions, _, term, _ := newService(t)
	sessions.refusal = errors.New("required")

	if _, err := svc.Open(t.Context(), OpenOptions{From: "s1", Kind: "shell"}); err != nil {
		t.Fatalf("Open(shell) = %v, want it open: the update is installed from one", err)
	}
	if len(term.spawns) != 1 || term.spawns[0].kind != "shell" {
		t.Fatalf("spawns = %+v, want the shell", term.spawns)
	}
}
