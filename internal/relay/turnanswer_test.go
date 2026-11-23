package relay

import (
	"context"
	"strings"
	"testing"
	"time"
)

// FinalMessage answers with what say recorded; a session never said into is
// one lich cannot read.
func (f *fakeTerminal) FinalMessage(id string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	text, ok := f.finals[id]
	return text, ok
}

// say records the final message of id's last turn.
func (f *fakeTerminal) say(id, text string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.finals == nil {
		f.finals = map[string]string{}
	}
	f.finals[id] = text
}

// sendAsync sends a task from s1 to docs (s2) and hands its result back on the
// channel once the call returns.
func sendAsync(svc *Service) <-chan Result {
	out := make(chan Result, 1)
	go func() {
		r, _ := svc.Send(context.Background(), SendOptions{From: "s1", Target: "docs", Prompt: "write the docs", WaitSeconds: 5})
		out <- r
	}()
	return out
}

func awaitResult(t *testing.T, results <-chan Result) Result {
	t.Helper()
	select {
	case r := <-results:
		return r
	case <-time.After(5 * time.Second):
		t.Fatal("the send never returned")
	}
	return Result{}
}

// A worker drawn as a chat answers its task by finishing its turn: the turn's
// final message is the answer, with nothing to call.
func TestAChatWorkerAnswersByItsLastMessage(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s1")
	term.chats = map[string]bool{"s2": true}
	term.openChat("s2", acked)
	term.say("s2", "an older answer")
	svc := viaChat(term, nil)

	results := sendAsync(svc)
	if !awaitPrompts(term, "s2", 1) {
		t.Fatal("the chat was never handed the task")
	}
	if task := term.promptsTo("s2")[0]; !strings.Contains(task, `answer="last-message"`) || strings.Contains(task, "<summary>") {
		t.Errorf("task = %q, want it answered by the last message", task)
	}
	svc.Observe("s2", stateBusy)
	term.say("s2", "The docs are written; see docs/cli.md.")
	svc.Observe("s2", stateDone)

	if r := awaitResult(t, results); r.Status != StatusAnswered || r.Answer != "The docs are written; see docs/cli.md." {
		t.Errorf("result = %+v, want the turn's final message", r)
	}
}

// A worker in a terminal whose provider reports its turns and whose transcript
// lich reads answers at its done report.
func TestATerminalWorkerAnswersAtDone(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s1", "s2")
	term.say("s2", "")
	svc := newRelay(workspace(), term, nil)
	svc.SetPlugins(fakePlugins{installed: true})

	results := sendAsync(svc)
	if !awaitWritten(term, "s2", `answer="last-message"`) {
		t.Fatalf("typed at s2 = %q, want a task answered by the last message", term.written("s2"))
	}
	svc.Observe("s2", stateBusy)
	term.say("s2", "done: 3 files changed")
	svc.Observe("s2", stateDone)

	if r := awaitResult(t, results); r.Status != StatusAnswered || r.Answer != "done: 3 files changed" {
		t.Errorf("result = %+v, want the turn's final message", r)
	}
}

// Where lich cannot read the final message the task is answered by its ticket.
func TestAWorkerLichCannotReadIsAnsweredByItsTicket(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)
	svc.SetPlugins(fakePlugins{installed: true})

	sendAsync(svc)
	if !awaitWritten(term, "s2", `answer="reply"`) {
		t.Fatalf("typed at s2 = %q, want a task answered by its ticket", term.written("s2"))
	}
}

// The first answer wins: a reply_to_session before the turn ends is the
// answer, and the turn's end does not answer a second time.
func TestAReplyBeforeTheTurnEndsIsTheAnswer(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s1", "s2")
	term.say("s2", "")
	svc := newRelay(workspace(), term, nil)
	svc.SetPlugins(fakePlugins{installed: true})

	results := sendAsync(svc)
	ticket := waitForTicket(svc)
	svc.Observe("s2", stateBusy)
	if _, err := svc.Reply(context.Background(), ReplyOptions{From: "s2", Ticket: ticket, Answer: "replied by ticket"}); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	term.say("s2", "the final message")
	svc.Observe("s2", stateDone)

	if r := awaitResult(t, results); r.Status != StatusAnswered || r.Answer != "replied by ticket" {
		t.Errorf("result = %+v, want the reply that came first", r)
	}
	time.Sleep(20 * time.Millisecond)
	if got, err := svc.Wait(context.Background(), WaitOptions{Ticket: ticket}); err == nil && got.Answer == "the final message" {
		t.Errorf("the turn's end answered a second time: %+v", got)
	}
}

// A turn that wrote nothing new stays unanswered, as it always has.
func TestABlankFinalTurnStaysUnanswered(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s1", "s2")
	term.say("s2", "the previous turn's words")
	svc := newRelay(workspace(), term, nil)
	svc.SetPlugins(fakePlugins{installed: true})
	svc.finalSettle = 0

	results := sendAsync(svc)
	waitForTicket(svc)
	svc.Observe("s2", stateBusy)
	svc.Observe("s2", stateDone)

	if r := awaitResult(t, results); r.Status != StatusUnanswered {
		t.Errorf("result = %+v, want unanswered", r)
	}
}

// A worker that delegated in turn does not answer "I handed it off": while an
// errand of its own is out, its turn's end answers nothing, and the turn its
// outcome starts is the one that answers.
func TestAWorkerWithAnErrandOutDoesNotAnswerEarly(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s1", "s2", "s3")
	term.say("s2", "")
	svc := newRelay(workspace(), term, nil)
	svc.SetPlugins(fakePlugins{installed: true})
	svc.nudgeDelay = time.Millisecond

	results := sendAsync(svc)
	waitForTicket(svc)
	svc.Observe("s2", stateBusy)
	plant(svc, "own", "s2", "s3", "api")
	term.say("s2", "I handed it to api.")
	svc.Observe("s2", stateDone)
	time.Sleep(20 * time.Millisecond)
	select {
	case r := <-results:
		t.Fatalf("answered while its own errand was out: %+v", r)
	default:
	}

	if _, err := svc.Reply(context.Background(), ReplyOptions{From: "s3", Ticket: "own", Answer: "api is done"}); err != nil {
		t.Fatalf("Reply own: %v", err)
	}
	if !awaitWritten(term, "s2", `kind="worker-finished"`) {
		t.Fatalf("the outcome never reached the worker: %q", term.written("s2"))
	}
	svc.Observe("s2", stateBusy)
	term.say("s2", "Done, api fixed it.")
	svc.Observe("s2", stateDone)
	if r := awaitResult(t, results); r.Status != StatusAnswered || r.Answer != "Done, api fixed it." {
		t.Errorf("result = %+v, want the answer of the turn the outcome started", r)
	}
}

// A done that lands before the transcript holds the last words still answers
// with them once they are written.
func TestALateTranscriptStillAnswers(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s1", "s2")
	term.say("s2", "")
	svc := newRelay(workspace(), term, nil)
	svc.SetPlugins(fakePlugins{installed: true})

	results := sendAsync(svc)
	waitForTicket(svc)
	svc.Observe("s2", stateBusy)
	svc.Observe("s2", stateDone)
	time.Sleep(3 * finalPoll)
	term.say("s2", "written after the hook")

	if r := awaitResult(t, results); r.Status != StatusAnswered || r.Answer != "written after the hook" {
		t.Errorf("result = %+v, want the words written after the done", r)
	}
}

// HandSubagent opens the errand and comes back at once with its ticket, the
// task handed over or held for the worker's prompt; the outcome reaches the
// asker later, as a subagent's report does.
func TestHandSubagentWaitsForNothing(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)
	start := time.Now()
	res, err := svc.HandSubagent(context.Background(), SendOptions{From: "s1", Target: "docs", Prompt: "write the docs"})
	if err != nil || res.Ticket == "" || res.Status != StatusPending || time.Since(start) > time.Second {
		t.Fatalf("HandSubagent = %+v, %v after %s", res, err, time.Since(start))
	}
	svc.mu.Lock()
	tk := svc.tickets[res.Ticket]
	svc.mu.Unlock()
	if tk == nil || !tk.subagent {
		t.Fatalf("ticket %+v, want an open subagent errand", tk)
	}
	if _, err := svc.HandSubagent(context.Background(), SendOptions{Target: "docs", Prompt: "x"}); err == nil {
		t.Error("a subagent errand from no session was accepted")
	}
}

// ErrandOpen is true while an errand waits on its worker and false once it
// was answered.
func TestErrandOpenFollowsTheAnswer(t *testing.T) {
	t.Parallel()
	term := newFakeTerminal("s1", "s2")
	svc := newRelay(workspace(), term, nil)
	res, err := svc.HandSubagent(context.Background(), SendOptions{From: "s1", Target: "docs", Prompt: "write the docs"})
	if err != nil || !svc.ErrandOpen(res.Ticket) {
		t.Fatalf("a fresh errand reads closed: %+v %v", res, err)
	}
	if _, err := svc.Reply(context.Background(), ReplyOptions{From: "s2", Ticket: res.Ticket, Answer: "done"}); err != nil {
		t.Fatal(err)
	}
	if svc.ErrandOpen(res.Ticket) {
		t.Error("an answered errand still reads open")
	}
}
