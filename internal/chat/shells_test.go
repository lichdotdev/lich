//go:build !windows

package chat

import (
	"slices"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// shells is the last shells event the window was sent, waiting for one that
// cond holds.
func (r *rig) shells(what string, cond func([]api.ChatShell) bool) []api.ChatShell {
	r.t.Helper()
	var got []api.ChatShell
	r.eventually(what, func() bool {
		for _, e := range slices.Backward(r.snapshot()) {
			if u, ok := e.data.(api.ChatShellsUpdate); ok && e.name == api.ChatShellsEvent.Name {
				got = u.Shells
				return cond(got)
			}
		}
		return false
	})
	return got
}

func TestClaudeBackgroundShellIsListedReadAndStopped(t *testing.T) {
	r := newChat(t, providers.Claude)
	r.prompt("shells")
	r.turnEnd(0)
	got := r.shells("a running shell", func(s []api.ChatShell) bool { return len(s) == 1 })
	if sh := got[0]; sh.ID != "bsh1" || sh.Command != "npm run dev" || sh.Description != "Start the dev server" || sh.State != api.ShellRunning || sh.Started == 0 || sh.Ended != 0 {
		t.Fatalf("shell = %+v", sh)
	}
	listed, err := r.svc.Shells(t.Context(), api.SessionIDOptions{ID: "s1"})
	if err != nil || len(listed) != 1 || listed[0].ID != "bsh1" {
		t.Fatalf("Shells = %+v, %v; the foreground command and the subagent are not shells", listed, err)
	}
	out, err := r.svc.ShellOutput(t.Context(), api.ChatShellOptions{ID: "s1", Shell: "bsh1"})
	if want := "ready on :5173\nbuild 100%\n"; err != nil || out != want {
		t.Errorf("ShellOutput = %q, %v; want %q", out, err, want)
	}
	if _, err := r.svc.StopShell(t.Context(), api.ChatShellOptions{ID: "s1", Shell: "bsh1"}); err != nil {
		t.Fatalf("StopShell: %v", err)
	}
	got = r.shells("the shell stopped", func(s []api.ChatShell) bool { return len(s) == 1 && s[0].State == api.ShellStopped })
	if got[0].Ended != 1795318375995 {
		t.Errorf("ended = %d, want the patch's end_time", got[0].Ended)
	}
	if _, err := r.svc.StopShell(t.Context(), api.ChatShellOptions{ID: "s1", Shell: "bsh1"}); err == nil {
		t.Error("a shell that ended was stopped again")
	}
	if _, err := r.svc.ShellOutput(t.Context(), api.ChatShellOptions{ID: "s1", Shell: "bfg1"}); err == nil {
		t.Error("a foreground command's output was read as a shell's")
	}
	if err := r.setView(api.ViewTerminal); err != nil {
		t.Fatalf("to terminal: %v", err)
	}
	r.shells("the shells gone with the process", func(s []api.ChatShell) bool { return len(s) == 0 })
}

func TestACPChatHasNoShells(t *testing.T) {
	r := newChat(t, providers.OMP)
	got, err := r.svc.Shells(t.Context(), api.SessionIDOptions{ID: "s1"})
	if err != nil || len(got) != 0 {
		t.Errorf("Shells = %+v, %v; want none, no error", got, err)
	}
	if _, err := r.svc.StopShell(t.Context(), api.ChatShellOptions{ID: "s1", Shell: "x"}); err != errNoShells {
		t.Errorf("StopShell = %v, want %v", err, errNoShells)
	}
}
