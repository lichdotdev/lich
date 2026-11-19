//go:build !windows

package chat

import (
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/relay"
)

// A task sent to a session drawn as a chat reaches its agent: the relay
// hands it to the chat, which sends it as the turn's message.
func TestARelayedTaskReachesTheAgentOfAChat(t *testing.T) {
	for _, kind := range []string{providers.Claude, providers.OpenCode, providers.Codex} {
		t.Run(kind, func(t *testing.T) {
			r := newRig(t, kind)
			if err := r.setView(api.ViewChat); err != nil {
				t.Fatalf("SetView(chat): %v", err)
			}
			rl := relay.New(r.db, r.term, nil)
			r.term.SetSessionState(rl.Observe)
			res, err := rl.Send(t.Context(), relay.SendOptions{Target: "S", Prompt: "relayed-task-7", WaitSeconds: 1})
			if err != nil {
				t.Fatalf("Send: %v", err)
			}
			if res.Ticket == "" {
				t.Fatalf("Send gave no ticket: %+v", res)
			}
			r.eventually("the agent to read the task", func() bool { return strings.Contains(r.fakeLog(), "relayed-task-7") })
		})
	}
}

// Tasks sent while a turn runs wait for it, each in a turn of its own and in
// the order they came.
func TestRelayedTasksWaitForTheTurnRunningInOrder(t *testing.T) {
	r := newRig(t, providers.OpenCode)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView(chat): %v", err)
	}
	rl := relay.New(r.db, r.term, nil)
	r.term.SetSessionState(rl.Observe)
	r.prompt("slow")
	r.eventually("the slow turn", func() bool { return strings.Contains(r.fakeLog(), `"slow"`) })
	for _, task := range []string{"queued-task-a", "queued-task-b"} {
		if _, err := rl.Send(t.Context(), relay.SendOptions{Target: "S", Prompt: task, WaitSeconds: 1}); err != nil {
			t.Fatalf("Send(%s): %v", task, err)
		}
	}
	if strings.Contains(r.fakeLog(), "queued-task-a") {
		t.Fatal("a task reached the agent in the middle of its turn")
	}
	if _, err := r.svc.Cancel(t.Context(), api.SessionIDOptions{ID: "s1"}); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	r.eventually("both tasks", func() bool {
		log := r.fakeLog()
		return strings.Contains(log, "queued-task-a") && strings.Contains(log, "queued-task-b")
	})
	log := r.fakeLog()
	if strings.Index(log, "queued-task-a") > strings.Index(log, "queued-task-b") {
		t.Error("the tasks reached the agent out of order")
	}
}

// The window's handoffs wait for the session to be ready and submit their
// message; a chat is ready once its conversation is open, and takes it.
func TestASubmittedHandoffReachesAChat(t *testing.T) {
	r := newRig(t, providers.Claude)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView(chat): %v", err)
	}
	if ready, _ := r.term.Ready(t.Context(), api.SessionIDOptions{ID: "s1"}); !ready {
		t.Fatal("an open chat does not read as ready")
	}
	rl := relay.New(r.db, r.term, nil)
	if _, err := rl.Submit(t.Context(), api.SubmitOptions{SessionID: "s1", Message: "handed-off-3"}); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	r.eventually("the agent to read the handoff", func() bool { return strings.Contains(r.fakeLog(), "handed-off-3") })
}

// A session born drawn as a chat opens straight into it, and a task sent the
// moment Open returns reaches its agent.
func TestABornChatTakesItsFirstTask(t *testing.T) {
	r := newRig(t, providers.Claude)
	r.db.SetChatKinds(func(kind string) bool { return kind == providers.Claude })
	if _, err := r.db.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "s2", Label: "Born", Kind: providers.Claude, NextSeq: 3}); err != nil {
		t.Fatal(err)
	}
	if !r.svc.Drawn("s2") {
		t.Fatal("the new session was not born drawn as a chat")
	}
	t.Cleanup(func() { r.svc.End("s2") })
	if err := r.svc.Open(t.Context(), "s2"); err != nil {
		t.Fatalf("Open: %v", err)
	}
	rl := relay.New(r.db, r.term, nil)
	if _, err := rl.Send(t.Context(), relay.SendOptions{Target: "Born", Prompt: "first-task-9", WaitSeconds: 1}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	r.eventually("the agent to read its first task", func() bool { return strings.Contains(r.fakeLog(), "first-task-9") })
}
