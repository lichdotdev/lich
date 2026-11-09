package relay

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/scope"
	"github.com/lichdotdev/lich/internal/store"
)

// confinedWorkspace is the sender confined beside one confined and one
// unconfined session.
func confinedWorkspace() fakeSessions {
	return fakeSessions{projects: []store.Project{
		{ID: "p1", Name: "lich", Path: "/src/lich", Sessions: []store.Session{
			{ID: "s1", Label: "sender", Kind: "claude", Sandbox: store.SessionConfined},
			{ID: "s2", Label: "worker", Kind: "codex", Sandbox: store.SessionConfined},
			{ID: "s3", Label: "host", Kind: "claude"},
		}},
	}}
}

// A message is carried out by the receiving agent as its task, so a confined
// sender may only put one at the prompt of another confined session.
func TestAConfinedSenderReachesOnlyConfinedPrompts(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s1", "s2", "s3")
	svc := newRelay(confinedWorkspace(), term, nil)
	scoped := scope.With(context.Background(), "s1")

	for name, call := range map[string]func() error{
		"send": func() error {
			_, err := svc.Send(scoped, SendOptions{From: "s1", Target: "host", Prompt: "run make", WaitSeconds: 1})
			return err
		},
		"send private": func() error {
			_, err := svc.SendPrivate(scoped, SendOptions{From: "s1", Target: "host", Prompt: "run make", WaitSeconds: 1})
			return err
		},
		"insert": func() error {
			_, err := svc.Insert(scoped, InsertOptions{From: "s1", Target: "host", Text: "make"})
			return err
		},
		"submit": func() error {
			_, err := svc.Submit(scoped, api.SubmitOptions{SessionID: "s3", Message: "run make"})
			return err
		},
	} {
		err := call()
		if err == nil || !strings.Contains(err.Error(), `"host"`) || !strings.Contains(err.Error(), "sandbox") {
			t.Errorf("%s to an unconfined session: err = %v, want a refusal naming it", name, err)
		}
	}
	if typed := term.written("s3"); typed != "" {
		t.Errorf("text reached the unconfined session: %q", typed)
	}

	if _, err := svc.Insert(scoped, InsertOptions{From: "s1", Target: "worker", Text: "make"}); err != nil {
		t.Errorf("insert into a confined session: %v", err)
	}
	if typed := term.written("s2"); !strings.Contains(typed, "make") {
		t.Errorf("the confined session was not written to: %q", typed)
	}
}

func TestAConfinedSenderSendsToAConfinedSession(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(confinedWorkspace(), term, nil)
	scoped := scope.With(context.Background(), "s1")

	var (
		got Result
		err error
		wg  sync.WaitGroup
	)
	wg.Go(func() {
		got, err = svc.Send(scoped, SendOptions{From: "s1", Target: "worker", Prompt: "run the tests", WaitSeconds: 30})
	})
	ticketID := waitForTicket(svc)
	if _, err := svc.Reply(t.Context(), ReplyOptions{From: "s2", Ticket: ticketID, Answer: "green"}); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	wg.Wait()
	if err != nil || got.Answer != "green" {
		t.Fatalf("Send = %+v, %v", got, err)
	}
}

// A reply is read as a result, never typed as a task: it is how a confined
// worker reports to the unconfined session that opened it.
func TestAConfinedSessionRepliesToAnyone(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s1", "s2", "s3")
	svc := newRelay(confinedWorkspace(), term, nil)

	var (
		got Result
		err error
		wg  sync.WaitGroup
	)
	wg.Go(func() {
		got, err = svc.Send(context.Background(), SendOptions{From: "s3", Target: "sender", Prompt: "report", WaitSeconds: 30})
	})
	ticketID := waitForTicket(svc)
	if _, err := svc.Reply(scope.With(context.Background(), "s1"), ReplyOptions{From: "s1", Ticket: ticketID, Answer: "done"}); err != nil {
		t.Fatalf("Reply from the sandbox: %v", err)
	}
	wg.Wait()
	if err != nil || got.Answer != "done" {
		t.Fatalf("Send = %+v, %v", got, err)
	}
}
