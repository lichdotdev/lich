package relay

import (
	"context"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/internal/prompt"
)

// labelOfID is the label each session of workspace() is addressed by.
var labelOfID = map[string]string{"s1": "sender", "s2": "docs", "s4": "parked"}

// hop is one turn of fromID that runs tools and then messages toID, ending
// once the message is in: the sender does not wait for an answer.
func hop(t *testing.T, svc *Service, fromID, toID string, tools ...string) error {
	t.Helper()
	for _, tool := range tools {
		svc.ObserveTool(fromID, tool, "")
	}
	svc.ObserveTool(fromID, "mcp__lich__send_to_session", "")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := svc.Send(ctx, SendOptions{From: fromID, Target: labelOfID[toID], Project: "lich", Prompt: "over to you"})
	svc.Observe(fromID, stateDone)
	return err
}

func pingPongRelay() (*Service, *fakeTerminal) {
	term := newFakeTerminal("s1", "s2", "s4")
	return newRelay(workspace(), term, nil), term
}

func warned(term *fakeTerminal, id string) bool {
	return strings.Contains(term.written(id), prompt.For(prompt.English).PingPongWarning)
}

func TestAnIdleExchangeIsWarnedThenBroken(t *testing.T) {
	t.Parallel()
	svc, term := pingPongRelay()

	if err := hop(t, svc, "s1", "s2"); err != nil {
		t.Fatalf("hop 1: %v", err)
	}
	if warned(term, "s2") {
		t.Error("the first message of an exchange was warned")
	}
	if err := hop(t, svc, "s2", "s1"); err != nil {
		t.Fatalf("hop 2: %v", err)
	}
	if !warned(term, "s1") {
		t.Errorf("the second idle message carried no warning:\n%s", term.written("s1"))
	}
	if err := hop(t, svc, "s1", "s2"); err != nil {
		t.Fatalf("hop 3: %v", err)
	}
	if !warned(term, "s2") {
		t.Errorf("the third idle message carried no warning:\n%s", term.written("s2"))
	}

	before := len(term.writesTo("s1"))
	err := hop(t, svc, "s2", "s1")
	if err == nil || !strings.Contains(err.Error(), "not delivered") {
		t.Fatalf("hop 4: err = %v, want the send refused", err)
	}
	if after := len(term.writesTo("s1")); after != before {
		t.Errorf("a refused message was typed: %d writes, then %d", before, after)
	}
	if err := hop(t, svc, "s1", "s2"); err == nil {
		t.Error("the exchange was refused once and went on without anything changing")
	}
}

// The lesson clowder-ai learned the hard way: a review loop trades many
// messages, and every one of them follows real work.
func TestAReviewLoopThatWorksIsNeverBroken(t *testing.T) {
	t.Parallel()
	svc, term := pingPongRelay()

	for round := range 10 {
		if err := hop(t, svc, "s1", "s2", "Edit", "Bash"); err != nil {
			t.Fatalf("round %d, change: %v", round, err)
		}
		if err := hop(t, svc, "s2", "s1", "Read"); err != nil {
			t.Fatalf("round %d, review: %v", round, err)
		}
	}
	if warned(term, "s1") || warned(term, "s2") {
		t.Error("a loop doing work between messages was warned")
	}
}

func TestAThirdSessionResetsTheStreak(t *testing.T) {
	t.Parallel()
	svc, term := pingPongRelay()

	for _, h := range [][2]string{{"s1", "s2"}, {"s2", "s1"}, {"s1", "s2"}, {"s1", "s4"}, {"s2", "s1"}} {
		if err := hop(t, svc, h[0], h[1]); err != nil {
			t.Fatalf("%s → %s: %v", h[0], h[1], err)
		}
	}
	if warned(term, "s4") {
		t.Error("the third session's first message was warned")
	}
	// s1 was warned at the second hop already; the last one went to it too, so
	// look at the message it was typed last.
	writes := term.writesTo("s1")
	if last := writes[len(writes)-2]; strings.Contains(last, prompt.For(prompt.English).PingPongWarning) {
		t.Errorf("the streak outlived a third session joining:\n%s", last)
	}
}

func TestTheUserTypingResetsTheStreak(t *testing.T) {
	t.Parallel()
	svc, term := pingPongRelay()

	for _, h := range [][2]string{{"s1", "s2"}, {"s2", "s1"}, {"s1", "s2"}} {
		if err := hop(t, svc, h[0], h[1]); err != nil {
			t.Fatalf("%s → %s: %v", h[0], h[1], err)
		}
	}
	svc.UserTyped("s2")
	if err := hop(t, svc, "s2", "s1"); err != nil {
		t.Fatalf("the exchange was refused after the user stepped in: %v", err)
	}
	writes := term.writesTo("s1")
	if last := writes[len(writes)-2]; strings.Contains(last, prompt.For(prompt.English).PingPongWarning) {
		t.Errorf("the streak outlived the user typing:\n%s", last)
	}
}

// A provider that reports no tools (Crush) gives no evidence of an idle turn,
// so its messages are never counted.
func TestAProviderReportingNoToolsIsNeverBroken(t *testing.T) {
	t.Parallel()
	svc, term := pingPongRelay()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	for i := range 6 {
		from, to := "s1", "docs"
		if i%2 == 1 {
			from, to = "s2", "sender"
		}
		if _, err := svc.Send(ctx, SendOptions{From: from, Target: to, Project: "lich", Prompt: "over to you"}); err != nil {
			t.Fatalf("hop %d: %v", i+1, err)
		}
		svc.Observe(from, stateDone)
	}
	if warned(term, "s1") || warned(term, "s2") {
		t.Error("an exchange with no tool reports was warned")
	}
}

func TestRelayToolsAreRecognisedInEveryProvidersSpelling(t *testing.T) {
	t.Parallel()
	tests := []struct {
		tool, detail string
		relay        bool
	}{
		{"mcp__lich__send_to_session", "", true},
		{"mcp__lich_reply_to_session", "", true},
		{"lich_wait_for_answer", "", true},
		{"call_mcp_tool", "lich/list_sessions", true},
		{"call_mcp_tool", "github/create_pull_request", false},
		{"mcp__lich__open_session", "", false},
		{"Bash", "lich reply 1802059c done", false},
		{"Edit", "", false},
	}
	for _, tt := range tests {
		if got := isRelayTool(tt.tool, tt.detail); got != tt.relay {
			t.Errorf("isRelayTool(%q, %q) = %v, want %v", tt.tool, tt.detail, got, tt.relay)
		}
	}
}

// A coordinator delegating to one worker trades many messages and runs nothing
// but relay tools itself: the work is the worker's, and its report comes back
// through a reply, which is not a hop.
func TestACoordinatorDelegatingToAWorkerThatWorksIsNeverWarned(t *testing.T) {
	t.Parallel()
	svc, term := pingPongRelay()

	for task := range 5 {
		if err := hop(t, svc, "s1", "s2", "mcp__lich__wait_for_answer"); err != nil {
			t.Fatalf("task %d: %v", task+1, err)
		}
		svc.ObserveTool("s2", "Bash", "go test ./...")
		svc.Observe("s2", stateDone)
	}
	if warned(term, "s2") {
		t.Errorf("a worker that ran tools between tasks was warned:\n%s", term.written("s2"))
	}
}

// delegateTo is one task from a coordinator that runs only relay tools to the
// Crush session of workspace().
func delegateTo(t *testing.T, svc *Service) error {
	t.Helper()
	svc.ObserveTool("s1", "mcp__lich__send_to_session", "")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := svc.Send(ctx, SendOptions{From: "s1", Target: "api", Project: "revu", Prompt: "next task"})
	svc.Observe("s1", stateDone)
	return err
}

// Crush reports no tools, so a worker there can work without lich hearing of
// it: its silence is no evidence of an idle exchange.
func TestACoordinatorDelegatingToACrushWorkerIsNeverWarned(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s1", "s5")
	svc := newRelay(workspace(), term, nil)

	for task := range 5 {
		if err := delegateTo(t, svc); err != nil {
			t.Fatalf("task %d: %v", task+1, err)
		}
	}
	if warned(term, "s5") {
		t.Errorf("a worker that cannot report tools was warned:\n%s", term.written("s5"))
	}
}

// A provider without the plugin reports nothing either, Crush or not.
func TestACoordinatorDelegatingToAWorkerWithoutThePluginIsNeverWarned(t *testing.T) {
	t.Parallel()
	svc, term := pingPongRelay()
	svc.SetPlugins(fakePlugins{installed: false})

	for task := range 5 {
		if err := hop(t, svc, "s1", "s2"); err != nil {
			t.Fatalf("task %d: %v", task+1, err)
		}
	}
	if warned(term, "s2") {
		t.Errorf("a worker whose hooks are not installed was warned:\n%s", term.written("s2"))
	}
}
