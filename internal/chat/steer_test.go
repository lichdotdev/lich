//go:build !windows

package chat

import (
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

func (r *rig) steer(text string) {
	r.t.Helper()
	if _, err := r.svc.Steer(r.t.Context(), api.ChatPromptOptions{ID: "s1", Text: text}); err != nil {
		r.t.Fatalf("Steer(%q): %v", text, err)
	}
}

// A message steered into Claude Code's turn reaches the agent in that turn.
func TestClaudeSteerReachesTheTurnRunning(t *testing.T) {
	r := newRig(t, providers.Claude)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.prompt("slow")
	r.mustItem(api.ItemAgent, "counting")
	r.steer("also the offsets")
	r.mustItem(api.ItemUser, "also the offsets")
	r.eventually("the steered message at the agent", func() bool { return strings.Contains(r.fakeLog(), "also the offsets") })
	r.turnEnd(0)
	if n := r.turnsBegun(); n != 1 {
		t.Errorf("%d turns began, want the one the message was steered into", n)
	}
}

// turnsBegun counts the turns lich reported running.
func (r *rig) turnsBegun() int {
	n := 0
	for _, e := range r.snapshot() {
		if turn, ok := e.data.(api.ChatTurn); ok && e.name == api.ChatTurnEvent.Name && turn.Running {
			n++
		}
	}
	return n
}

// Codex takes a steered message as turn/steer on the turn running.
func TestCodexSteerSteersTheTurn(t *testing.T) {
	r := newRig(t, providers.Codex)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.prompt("slow")
	r.mustItem(api.ItemAgent, "counting")
	r.steer("also the offsets")
	r.mustItem(api.ItemUser, "also the offsets")
	r.eventually("turn/steer with the message", func() bool {
		log := r.fakeLog()
		return strings.Contains(log, "turn/steer") && strings.Contains(log, "also the offsets")
	})
	if _, err := r.svc.Cancel(t.Context(), api.SessionIDOptions{ID: "s1"}); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	r.turnEnd(0)
}

// ACP takes no message during a turn: a steered one stops the turn and goes
// in a turn of its own once it ended.
func TestACPSteerStopsTheTurnAndSendsAfter(t *testing.T) {
	r := newRig(t, providers.OMP)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.prompt("slow")
	r.mustItem(api.ItemAgent, "counting")
	r.steer("hi")
	if turn := r.turnEnd(0); turn.StopReason != stopCancelled {
		t.Errorf("the running turn ended %q, want cancelled", turn.StopReason)
	}
	if turn := r.turnEnd(1); turn.StopReason != stopEndTurn {
		t.Errorf("the steered message's turn ended %q", turn.StopReason)
	}
	r.mustItem(api.ItemUser, "hi")
}

// With no turn running, a steered message is a prompt.
func TestSteerWithNoTurnIsAPrompt(t *testing.T) {
	r := newRig(t, providers.Claude)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.steer("hi")
	r.turnEnd(0)
	r.mustItem(api.ItemUser, "hi")
}
