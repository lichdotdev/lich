package relay

import (
	"context"
	"testing"
)

// Chat answers with the sessions the fake draws as a chat.
func (f *fakeTerminal) Chat(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.chats[id]
}

// A worker that answered by its ticket while its own worker still runs is not
// closed under that worker: the report it is owed would reach no one.
func TestAWorkerWithAnErrandOfItsOwnOpenIsNotFinished(t *testing.T) {
	t.Parallel()
	svc, finished := finishing(t)
	plantSubagent(svc, "t1", "s1", "s2", "docs")
	plantSubagent(svc, "t2", "s2", "s3", "api")

	svc.Observe("s2", stateBusy)
	if _, err := svc.Reply(t.Context(), ReplyOptions{From: "s2", Ticket: "t1", Answer: "the report"}); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	svc.Observe("s2", stateDone)
	expectNoneFinished(t, finished)
}

// A worker that answers by its ticket and runs a subagent of its own in the
// background ends its turn waiting for that subagent: the report reaching its
// prompt is what resumes it. That turn ending is not the worker finishing
// without an answer, whether the report is still on its way or already waits
// in its inbox for the prompt to free up.
func TestATurnEndingWhileTheWorkerAwaitsItsOwnErrandKeepsItsErrandOpen(t *testing.T) {
	t.Parallel()
	for name, reportFirst := range map[string]bool{"report still running": false, "report waiting": true} {
		t.Run(name, func(t *testing.T) {
			term := newFakeTerminal("s1", "s2", "s3")
			svc := newRelay(workspace(), term, nil)
			plantSubagent(svc, "t1", "s1", "s2", "docs")
			plantSubagent(svc, "t2", "s2", "s3", "api")

			svc.Observe("s2", stateBusy)
			if reportFirst {
				if _, err := svc.Reply(t.Context(), ReplyOptions{From: "s3", Ticket: "t2", Answer: "the api is done"}); err != nil {
					t.Fatalf("Reply: %v", err)
				}
			}
			svc.Observe("s2", stateDone)
			expectOpen(t, svc, "t1")

			if !reportFirst {
				if _, err := svc.Reply(t.Context(), ReplyOptions{From: "s3", Ticket: "t2", Answer: "the api is done"}); err != nil {
					t.Fatalf("Reply: %v", err)
				}
			}
			if !awaitWritten(term, "s2", "api") {
				t.Fatal("the worker was never told its own worker finished")
			}
			svc.Observe("s2", stateBusy)
			if _, err := svc.Reply(t.Context(), ReplyOptions{From: "s2", Ticket: "t1", Answer: "docs and api are done"}); err != nil {
				t.Fatalf("Reply: %v", err)
			}
			expectAnswer(t, svc, "t1", "docs and api are done")
		})
	}
}

// expectOpen fails unless the errand is still open: no answer reached it.
func expectOpen(t *testing.T, svc *Service, id string) {
	t.Helper()
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.tickets[id] == nil {
		t.Fatalf("errand %s was closed", id)
	}
}

func expectAnswer(t *testing.T, svc *Service, id, want string) {
	t.Helper()
	got, err := svc.Wait(context.Background(), WaitOptions{Ticket: id, WaitSeconds: 1})
	if err != nil || got.Status != StatusAnswered || got.Answer != want {
		t.Fatalf("Wait(%s) = %+v, %v, want answered %q", id, got, err, want)
	}
}
