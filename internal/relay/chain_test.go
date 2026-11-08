package relay

import (
	"context"
	"strings"
	"testing"
)

// relayOn is a message from fromID to toID while fromID still works on what it
// was sent, which is what carries a chain along.
func relayOn(t *testing.T, svc *Service, fromID, toID string) error {
	t.Helper()
	svc.ObserveTool(fromID, "mcp__lich__send_to_session", "")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := svc.Send(ctx, SendOptions{From: fromID, Target: labelOfID[toID], Project: "lich", Prompt: "pass it on"})
	return err
}

// ticketsTo lists the open tickets a session owes an answer to.
func ticketsTo(svc *Service, id string) map[string]*ticket {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	owed := map[string]*ticket{}
	for ticketID, t := range svc.tickets {
		if t.targetID == id {
			owed[ticketID] = t
		}
	}
	return owed
}

var threeSessionLoop = [][2]string{{"s1", "s2"}, {"s2", "s4"}, {"s4", "s1"}, {"s1", "s2"}, {"s2", "s4"}}

func TestALoopThroughThreeSessionsIsRefusedAtTheChainLimit(t *testing.T) {
	t.Parallel()
	svc, term := pingPongRelay()

	for i, h := range threeSessionLoop {
		if err := relayOn(t, svc, h[0], h[1]); err != nil {
			t.Fatalf("message at depth %d, %s → %s: %v", i, h[0], h[1], err)
		}
	}
	before := len(term.writesTo("s1"))
	err := relayOn(t, svc, "s4", "s1")
	if err == nil || !strings.Contains(err.Error(), "5 relayed messages deep") {
		t.Fatalf("the message at depth 5: err = %v, want it refused", err)
	}
	if after := len(term.writesTo("s1")); after != before {
		t.Errorf("a refused message was typed: %d writes, then %d", before, after)
	}
}

func TestATurnOwingNoTaskStartsTheChainOver(t *testing.T) {
	t.Parallel()
	svc, _ := pingPongRelay()

	for _, h := range threeSessionLoop {
		if err := relayOn(t, svc, h[0], h[1]); err != nil {
			t.Fatalf("%s → %s: %v", h[0], h[1], err)
		}
	}
	for ticketID := range ticketsTo(svc, "s4") {
		if _, err := svc.Reply(t.Context(), ReplyOptions{From: "s4", Ticket: ticketID, Answer: "done"}); err != nil {
			t.Fatalf("Reply: %v", err)
		}
	}
	before := ticketsTo(svc, "s1")
	if err := relayOn(t, svc, "s4", "s1"); err != nil {
		t.Fatalf("a turn owing nothing was refused: %v", err)
	}
	for ticketID, tk := range ticketsTo(svc, "s1") {
		if _, old := before[ticketID]; !old && tk.depth != 0 {
			t.Errorf("a message from a turn owing nothing has depth %d, want 0", tk.depth)
		}
	}
}
