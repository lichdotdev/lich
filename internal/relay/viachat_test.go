package relay

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// fakeChat stands in for a session's chat. It answers each prompt with the
// next scripted receipt; once the script runs out the chat is gone, and the
// session reads as not drawn as one. An acked or ended receipt arrives at once,
// the way a turn's start or the chat's exit does; delivered only once the wait
// on it ends.
type fakeChat struct {
	receipts []PromptReceipt
	err      error
	prompts  []string
}

func (f *fakeTerminal) ChatPrompt(id, text string) (HandedPrompt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	chat := f.takes[id]
	if chat == nil {
		return HandedPrompt{}, ErrNoChat
	}
	if chat.err != nil {
		return HandedPrompt{}, chat.err
	}
	if len(chat.prompts) >= len(chat.receipts) {
		return HandedPrompt{}, ErrNoChat
	}
	receipt := chat.receipts[len(chat.prompts)]
	chat.prompts = append(chat.prompts, text)
	return HandedPrompt{Await: func(ctx context.Context) PromptReceipt {
		if receipt.State == PromptDelivered {
			<-ctx.Done()
		}
		return receipt
	}}, nil
}

func (f *fakeTerminal) ChatOpened(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.chats[id] && !f.opening[id]
}

// openingChat makes a chat one still opening its conversation, or, with false,
// one that has opened it.
func (f *fakeTerminal) openingChat(id string, opening bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.chats == nil {
		f.chats = map[string]bool{}
	}
	f.chats[id] = true
	f.opening[id] = opening
}

// openChat draws a session as a chat that answers its prompts with receipts,
// in order.
func (f *fakeTerminal) openChat(id string, receipts ...PromptReceipt) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.chats == nil {
		f.chats = map[string]bool{}
	}
	f.chats[id] = true
	f.takes[id] = &fakeChat{receipts: receipts}
}

// failChat makes handing a prompt to that session's chat fail with err.
func (f *fakeTerminal) failChat(id string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.chats == nil {
		f.chats = map[string]bool{}
	}
	f.chats[id] = true
	f.takes[id] = &fakeChat{err: err}
}

// promptsTo is every prompt a session's chat was handed, in order.
func (f *fakeTerminal) promptsTo(id string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if chat := f.takes[id]; chat != nil {
		return append([]string(nil), chat.prompts...)
	}
	return nil
}

// awaitPrompts blocks until a session's chat was handed n prompts, and returns
// whether it was.
func awaitPrompts(term *fakeTerminal, id string, n int) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(term.promptsTo(id)) >= n {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}

var (
	acked  = PromptReceipt{State: PromptAcked}
	queued = PromptReceipt{State: PromptDelivered}
)

// viaChat is a relay whose sessions report state, with a receipt window short
// enough to run out inside a test.
func viaChat(term *fakeTerminal, events Events) *Service {
	svc := newRelay(workspace(), term, events)
	svc.receiptWindow = 40 * time.Millisecond
	svc.chatTurnWait = 40 * time.Millisecond
	svc.SetPlugins(fakePlugins{installed: true})
	return svc
}

func assertNotTyped(t *testing.T, term *fakeTerminal, id string) {
	t.Helper()
	if writes := term.writesTo(id); len(writes) != 0 {
		t.Errorf("%s was typed at: %q", id, writes)
	}
}

func TestATaskToAChatIsHandedToItNotTyped(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s1", "s2")
	term.openChat("s2", acked)
	events := &fakeEvents{}
	svc := viaChat(term, events)

	done := make(chan Result, 1)
	go func() {
		got, _ := svc.Send(context.Background(), SendOptions{From: "s1", Target: "docs", Prompt: "run the tests", WaitSeconds: 5})
		done <- got
	}()
	if !events.awaitMark("s2", DirectionIn) || !events.awaitMark("s1", DirectionOut) {
		t.Fatal("the cards were never marked")
	}
	ticketID := waitForTicket(svc)
	if _, err := svc.Reply(t.Context(), ReplyOptions{Ticket: ticketID, Answer: "all green"}); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if got := <-done; got.Status != StatusAnswered || got.Answer != "all green" {
		t.Fatalf("result = %+v, want answered", got)
	}
	prompts := term.promptsTo("s2")
	if len(prompts) != 1 || !strings.Contains(prompts[0], "run the tests") || !strings.Contains(prompts[0], ticketID) {
		t.Errorf("the chat was handed %q, want the composed task once", prompts)
	}
	assertNotTyped(t, term, "s2")
}

// A chat still opening its conversation would take a message mid-handshake,
// so a task waits for it to open and goes to the chat.
func TestATaskWaitsForAChatStillOpening(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s1", "s2")
	term.openingChat("s2", true)
	svc := viaChat(term, nil)

	done := make(chan Result, 1)
	go func() {
		got, _ := svc.Send(context.Background(), SendOptions{From: "s1", Target: "docs", Prompt: "run the tests", WaitSeconds: 5})
		done <- got
	}()
	time.Sleep(2 * readyPoll)
	assertNotTyped(t, term, "s2")

	term.openChat("s2", acked)
	term.openingChat("s2", false)
	if !awaitPrompts(term, "s2", 1) {
		t.Fatal("the task never reached the chat once it opened")
	}
	assertNotTyped(t, term, "s2")
	if _, err := svc.Reply(t.Context(), ReplyOptions{Ticket: waitForTicket(svc), Answer: "all green"}); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	<-done
}

// A busy target runs the prompt once its turn ends, which can be long after
// the window: the errand waits for its turn instead of being written off.
func TestATaskQueuedBehindABusyChatWaitsForItsTurn(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s1", "s2")
	term.openChat("s2", queued)
	svc := viaChat(term, nil)
	svc.Observe("s2", stateBusy)

	done := make(chan Result, 1)
	go func() {
		got, _ := svc.Send(context.Background(), SendOptions{From: "s1", Target: "docs", Prompt: "run the tests", WaitSeconds: 5})
		done <- got
	}()
	awaitDelivered(t, svc, 1)
	time.Sleep(3 * svc.receiptWindow)
	if open := openTickets(svc); len(open) != 1 {
		t.Fatalf("open tickets = %v, want the errand still waiting for its turn", open)
	}

	svc.Observe("s2", stateDone)
	svc.Observe("s2", stateBusy)
	svc.Observe("s2", stateDone)
	if got := <-done; got.Status != StatusUnanswered {
		t.Errorf("status = %q, want unanswered: its own turn ended without a reply", got.Status)
	}
	assertNotTyped(t, term, "s2")
}

func TestAChatTaskTheTargetStartedIsNotUnread(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s1", "s2")
	term.openChat("s2", acked)
	svc := viaChat(term, nil)
	go func() {
		time.Sleep(10 * time.Millisecond)
		svc.Observe("s2", stateBusy)
	}()

	got, err := svc.Send(context.Background(), SendOptions{From: "s1", Target: "docs", Prompt: "run the tests", WaitSeconds: 1})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got.Status != StatusPending {
		t.Errorf("status = %q, want the errand still open", got.Status)
	}
	assertNotTyped(t, term, "s2")
}

func TestChatPromptErrorIsTheSendsError(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s1", "s2")
	refused := errors.New("the transport is down")
	term.failChat("s2", refused)
	svc := viaChat(term, nil)

	_, err := svc.Send(context.Background(), SendOptions{From: "s1", Target: "docs", Prompt: "run the tests", WaitSeconds: 5})
	if !errors.Is(err, refused) {
		t.Fatalf("Send = %v, want the chat's error", err)
	}
	if open := openTickets(svc); len(open) != 0 {
		t.Errorf("open tickets = %v, want none", open)
	}
	assertNotTyped(t, term, "s2")
}

// A chat queues a message apart from the person's draft, so a sender typing in
// it is told at once instead of after its draft goes stale.
func TestANudgeReachesASenderThroughItsChat(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s1", "s2")
	term.openChat("s1", acked)
	term.typeAt("s1", true)
	svc := viaChat(term, nil)
	plant(svc, "t1", "s1", "s2", "docs")

	if _, err := svc.Reply(t.Context(), ReplyOptions{Ticket: "t1", Answer: "all green"}); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if !awaitPrompts(term, "s1", 1) {
		t.Fatal("the sender's chat was never handed the nudge")
	}
	if prompt := term.promptsTo("s1")[0]; !strings.Contains(prompt, "<lich-notification") {
		t.Errorf("nudge = %q, want the envelope", prompt)
	}
	assertNotTyped(t, term, "s1")
}

func TestAScheduledPromptGoesThroughTheChat(t *testing.T) {
	t.Parallel()
	writer := &scheduleWriter{}
	term := newFakeTerminal("s1")
	term.openChat("s1", acked)
	svc := newRelay(scheduled(1000, "run the release checklist", writer), term, nil)
	svc.now = at(1000)

	svc.deliverDue()

	if prompts := term.promptsTo("s1"); len(prompts) != 1 || !strings.Contains(prompts[0], "run the release checklist") {
		t.Fatalf("the chat was handed %q, want the scheduled prompt", prompts)
	}
	assertNotTyped(t, term, "s1")
}

func TestTheTicketNoticeGoesThroughTheChat(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s1", "s2", "s3")
	term.openChat("s2", acked, acked, acked)
	svc := viaChat(term, nil)

	for _, from := range []string{"s1", "s3"} {
		go func() {
			_, _ = svc.Send(context.Background(), SendOptions{From: from, Target: "docs", Prompt: "task from " + from, WaitSeconds: 5})
		}()
	}
	awaitDelivered(t, svc, 2)
	svc.Observe("s2", stateBusy)
	svc.Observe("s2", stateDone)

	if !awaitPrompts(term, "s2", 3) {
		t.Fatalf("the worker's chat was never asked to name the ticket: %q", term.promptsTo("s2"))
	}
	if notice := term.promptsTo("s2")[2]; !strings.Contains(notice, "has to name its ticket") {
		t.Errorf("third prompt = %q, want the notice", notice)
	}
	assertNotTyped(t, term, "s2")
}

// A nudge to a busy chat waits on the turn after the running one, and the
// report that ends the running turn must not wait on that.
func TestAStateReportDoesNotWaitOnTheNudgesTurn(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s1", "s2")
	term.openChat("s1", queued)
	svc := viaChat(term, nil)
	svc.chatTurnWait = 5 * time.Second
	svc.Observe("s1", stateBusy)
	plant(svc, "t1", "s1", "s2", "docs")
	if _, err := svc.Reply(t.Context(), ReplyOptions{Ticket: "t1", Answer: "all green"}); err != nil {
		t.Fatalf("Reply: %v", err)
	}

	returned := make(chan struct{})
	go func() {
		svc.Observe("s1", stateInterrupted)
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("the report waited on the nudge's turn")
	}
	if !awaitPrompts(term, "s1", 1) {
		t.Fatal("the sender's chat was never handed the nudge")
	}
}

// A chat never touches the person's draft, so a task does not wait for it.
func TestATaskReachesADraftingChat(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s1", "s2")
	term.openChat("s2", acked)
	term.typeAt("s2", true)
	svc := viaChat(term, nil)

	if _, err := svc.Send(context.Background(), SendOptions{From: "s1", Target: "docs", Prompt: "run the tests", WaitSeconds: 1}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if prompts := term.promptsTo("s2"); len(prompts) != 1 {
		t.Errorf("the chat was handed %d prompts, want the task", len(prompts))
	}
	assertNotTyped(t, term, "s2")
}

func TestAScheduledPromptReachesADraftingChat(t *testing.T) {
	t.Parallel()
	writer := &scheduleWriter{}
	term := newFakeTerminal("s1")
	term.openChat("s1", acked)
	term.typeAt("s1", true)
	svc := newRelay(scheduled(1000, "run the release checklist", writer), term, nil)
	svc.now = at(1000)

	svc.deliverDue()

	if prompts := term.promptsTo("s1"); len(prompts) != 1 {
		t.Fatalf("the chat was handed %q, want the scheduled prompt", prompts)
	}
	assertNotTyped(t, term, "s1")
}
