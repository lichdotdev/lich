package relay

import (
	"context"
	"strings"
	"testing"
	"time"
)

// A compaction may fold a relayed task's ticket out of the worker's
// conversation, and with two errands open a ticketless answer is refused. The
// report closing the compaction types the open errands back in, once.
func TestACompactionWithErrandsOpenRemindsTheWorkerOnce(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s1", "s2", "s3")
	svc := newRelay(workspace(), term, &fakeEvents{})

	go func() {
		_, _ = svc.Send(context.Background(), SendOptions{
			From: "s1", Target: "docs", Prompt: "run the tests and report the failures", WaitSeconds: 30,
		})
	}()
	awaitSubmitted(t, svc, 1)
	go func() {
		_, _ = svc.Send(context.Background(), SendOptions{From: "s3", Target: "docs", Prompt: "build the docs", WaitSeconds: 30})
	}()
	awaitSubmitted(t, svc, 2)
	delivery := len(term.writesTo("s2"))
	tickets := openTickets(svc)

	svc.Observe("s2", "busy")
	svc.Observe("s2", "compacting")
	svc.Observe("s2", "busy")

	if !awaitWrites(term, "s2", delivery+2) {
		t.Fatal("the worker was never reminded of its errands after the compaction")
	}
	reminder := strings.Join(term.writesTo("s2")[delivery:], "")
	for _, want := range []string{
		"[lich]", "was just compacted", "run the tests and report the failures", "build the docs",
	} {
		if !strings.Contains(reminder, want) {
			t.Errorf("the reminder is missing %q:\n%s", want, reminder)
		}
	}
	for _, id := range tickets {
		if !strings.Contains(reminder, "lich reply "+id) {
			t.Errorf("the reminder does not say how to answer ticket %q:\n%s", id, reminder)
		}
	}

	svc.Observe("s2", "busy")
	svc.Observe("s2", "busy")
	time.Sleep(50 * time.Millisecond)
	if got := len(term.writesTo("s2")); got != delivery+2 {
		t.Errorf("writes = %d, want %d: one compaction is reminded once", got, delivery+2)
	}
}

func TestACompactionWithNothingOwedTypesNothing(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, &fakeEvents{})

	svc.Observe("s2", "done")
	svc.Observe("s2", "compacting")
	svc.Observe("s2", "done")

	time.Sleep(50 * time.Millisecond)
	if got := len(term.writesTo("s2")); got != 0 {
		t.Errorf("writes = %d, want none: the session owes no answer", got)
	}
}

// A report with no compaction before it is an ordinary turn report, whatever
// is open.
func TestErrandsOpenWithoutACompactionAreNotReminded(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, &fakeEvents{})

	go func() {
		_, _ = svc.Send(context.Background(), SendOptions{From: "s1", Target: "docs", Prompt: "run the tests", WaitSeconds: 30})
	}()
	awaitSubmitted(t, svc, 1)
	delivery := len(term.writesTo("s2"))

	svc.Observe("s2", "busy")
	svc.Observe("s2", "busy")

	time.Sleep(50 * time.Millisecond)
	if got := len(term.writesTo("s2")); got != delivery {
		t.Errorf("writes = %d, want %d: nothing was compacted", got, delivery)
	}
}
