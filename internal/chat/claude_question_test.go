//go:build !windows

package chat

import (
	"reflect"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// waitingQuestion waits for the question item the agent is waiting on.
func (r *rig) waitingQuestion() api.ChatItem {
	r.t.Helper()
	var got api.ChatItem
	// The item can show before the question waits: Claude Code streams the
	// tool_use that asks before the request lich answers it on.
	r.eventually("a waiting question", func() bool {
		for _, it := range r.history() {
			if it.Kind == api.ItemQuestion && it.Question != nil && it.Question.Reply == nil && r.asking(it.ID) {
				got = it
				return true
			}
		}
		return false
	})
	return got
}

// asking reports whether the session waits on question item id.
func (r *rig) asking(id string) bool {
	c := r.svc.running("s1")
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.questions[id]
	return ok
}

func (r *rig) reply(itemID string, reply api.ChatQReplied) error {
	_, err := r.svc.Reply(r.t.Context(), api.ChatReplyOptions{ID: "s1", ItemID: itemID, Reply: reply})
	return err
}

// TestClaudeQuestionAnswered: AskUserQuestion becomes a question item, not a
// tool or a permission, the card waits on it, and the answers go back in the
// tool's input the way the Agent SDK writes them.
func TestClaudeQuestionAnswered(t *testing.T) {
	r := newRig(t, providers.Claude)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.prompt("question")
	q := r.waitingQuestion()
	asks := q.Question.Questions
	if q.ID != "toolu_q" || len(asks) != 3 || asks[0].Header != "Fruit" || asks[0].Multi || !asks[1].Multi ||
		asks[0].Options[1].Preview != "pear()" || asks[0].Options[0].Description != "red" || len(asks[2].Options) != 0 {
		t.Fatalf("question = %+v", q)
	}
	for _, it := range r.history() {
		if it.Kind == api.ItemTool || it.Kind == api.ItemPermission {
			t.Errorf("AskUserQuestion also became a %s item", it.Kind)
		}
	}
	if st := r.term.Status("s1"); st.State != stateWaiting {
		t.Errorf("card state while asking = %q, want waiting", st.State)
	}
	if err := r.reply(q.ID, api.ChatQReplied{Answers: []api.ChatAskAnswer{{Picked: []string{"Pear"}}}}); err == nil {
		t.Error("a reply missing two answers was accepted")
	}
	answers := []api.ChatAskAnswer{{Picked: []string{"Pear"}}, {Picked: []string{"Red", "Blue"}, Other: "teal"}, {Other: " a mango "}}
	if err := r.reply(q.ID, api.ChatQReplied{Answers: answers}); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	r.turnEnd(0)
	r.mustItem(api.ItemAgent, `answers:{"Anything else?":"a mango","Which colors?":"Red, Blue, teal","Which fruit?":"Pear"}`)
	got, _ := r.svc.running("s1").conv.get(q.ID)
	if got.Question.Reply == nil || !reflect.DeepEqual(got.Question.Reply.Answers, answers) {
		t.Errorf("reply recorded = %+v, want the person's", got.Question.Reply)
	}
	if err := r.reply(q.ID, api.ChatQReplied{Answers: answers}); err == nil {
		t.Error("a question was answered twice")
	}
	if _, err := r.svc.Reply(t.Context(), api.ChatReplyOptions{ID: "nobody", ItemID: q.ID}); err == nil {
		t.Error("a reply to a session with no chat was accepted")
	}
}

// TestClaudeQuestionDeclined: Esc and a cancel answer none, and the agent is
// told so through the prompt catalog.
func TestClaudeQuestionDeclined(t *testing.T) {
	r := newRig(t, providers.Claude)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.prompt("question")
	q := r.waitingQuestion()
	if err := r.reply(q.ID, api.ChatQReplied{Declined: true}); err != nil {
		t.Fatalf("Reply(declined): %v", err)
	}
	r.turnEnd(0)
	r.mustItem(api.ItemAgent, "declined")
	if !strings.Contains(r.fakeLog(), "The person did not answer these questions.") {
		t.Error("the deny did not tell the agent nobody answered")
	}
	if got, _ := r.svc.running("s1").conv.get(q.ID); got.Question.Reply == nil || !got.Question.Reply.Declined {
		t.Errorf("declined question = %+v", got.Question)
	}

	r.prompt("question")
	r.eventually("the second question", func() bool {
		c := r.svc.running("s1")
		c.mu.Lock()
		defer c.mu.Unlock()
		return len(c.questions) == 1
	})
	if _, err := r.svc.Cancel(t.Context(), api.SessionIDOptions{ID: "s1"}); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	r.turnEnd(1)
	if strings.Count(r.fakeLog(), "The person did not answer these questions.") != 2 {
		t.Error("a cancel did not decline the question it stopped")
	}
}

// TestClaudeQuestionThenMessage: a message written while the agent waits
// declines its questions and reaches it in the same turn.
func TestClaudeQuestionThenMessage(t *testing.T) {
	r := newRig(t, providers.Claude)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.prompt("question-then-write")
	q := r.waitingQuestion()
	r.prompt("cheese instead")
	r.mustItem(api.ItemAgent, "then:cheese instead")
	if turn := r.turnEnd(0); turn.StopReason != stopEndTurn {
		t.Errorf("turn ended %q", turn.StopReason)
	}
	log := r.fakeLog()
	deny, msg := strings.Index(log, "The person did not answer"), strings.Index(log, `"content":"cheese instead"`)
	if deny < 0 || msg < deny {
		t.Errorf("the message did not follow the decline:\n%s", log)
	}
	r.mustItem(api.ItemUser, "cheese instead")
	if got, _ := r.svc.running("s1").conv.get(q.ID); got.Question.Reply == nil || !got.Question.Reply.Declined {
		t.Errorf("question after the message = %+v", got.Question)
	}
	if got := r.term.Status("s1").State; got != stateDone {
		t.Errorf("card state = %q, want done", got)
	}
}

// TestClaudeQuestionHistory reads questions back from the transcript: the
// answers the tool echoed, a deny, and one the process never got to.
func TestClaudeQuestionHistory(t *testing.T) {
	r := newRig(t, providers.Claude)
	_ = r.db.SetProviderSession("s1", "conv-q")
	ask := `{"questions":[{"question":"Which colors?","header":"Colors","multiSelect":true,"options":[{"label":"Red"},{"label":"Red, dark"},{"label":"Blue"}]}]}`
	use := func(id string) string {
		return `{"type":"assistant","message":{"id":"` + id + `","content":[{"type":"tool_use","id":"` + id + `","name":"AskUserQuestion","input":` + ask + `}]}}`
	}
	r.writeClaudeTranscript("conv-q",
		use("q1"),
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"q1","content":"ok"}]},"toolUseResult":{"answers":{"Which colors?":"Red, Blue, teal"}}}`,
		use("q2"),
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"q2","is_error":true,"content":"no"}]}}`,
		use("q3"),
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"q4","content":"x"}]},"toolUseResult":{"answers":{"Which colors?":"Red, dark"}}}`,
	)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	byID := map[string]api.ChatItem{}
	for _, it := range r.history() {
		byID[it.ID] = it
	}
	if got := byID["q1"].Question.Reply; got == nil || !reflect.DeepEqual(got.Answers, []api.ChatAskAnswer{{Picked: []string{"Red", "Blue"}, Other: "teal"}}) {
		t.Errorf("q1 reply = %+v", got)
	}
	if got := byID["q2"].Question.Reply; got == nil || !got.Declined {
		t.Errorf("q2 reply = %+v, want declined", got)
	}
	if got := byID["q3"].Question.Reply; got == nil || !got.Declined {
		t.Errorf("q3 reply = %+v, want declined: nothing is left to answer it", got)
	}
}

// TestClaudeReplied: a whole answer that is a label wins over splitting it.
func TestClaudeReplied(t *testing.T) {
	asks := []api.ChatAsk{{Text: "c", Options: []api.ChatOffer{{Label: "Red, dark"}, {Label: "Blue"}}}, {Text: "free"}}
	got := claudeReplied(asks, map[string]string{"c": "Red, dark", "free": "anything, really"})
	want := []api.ChatAskAnswer{{Picked: []string{"Red, dark"}}, {Other: "anything, really"}}
	if !reflect.DeepEqual(got.Answers, want) {
		t.Errorf("claudeReplied = %+v, want %+v", got.Answers, want)
	}
}

// TestARelayedMessageWaitsBehindAQuestion: what the relay hands a chat goes
// in as the next user message, never into the turn running
// (docs/hooks/notification.md, Delivery), so a question the agent waits on is
// left for the person and the message follows once that turn ends.
func TestARelayedMessageWaitsBehindAQuestion(t *testing.T) {
	r := newRig(t, providers.Claude)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.prompt("question")
	q := r.waitingQuestion()
	if _, err := r.svc.Take("s1", "relayed note"); err != nil {
		t.Fatalf("Take: %v", err)
	}
	if got, _ := r.svc.running("s1").conv.get(q.ID); got.Question.Reply != nil {
		t.Fatalf("the relayed message answered the person's question: %+v", got.Question.Reply)
	}
	if strings.Contains(r.fakeLog(), "relayed note") {
		t.Fatal("the relayed message went into the running turn")
	}
	answers := []api.ChatAskAnswer{{Picked: []string{"Pear"}}, {Picked: []string{"Red"}}, {Other: "a mango"}}
	if err := r.reply(q.ID, api.ChatQReplied{Answers: answers}); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	r.turnEnd(0)
	r.eventually("the relayed message after the turn", func() bool { return strings.Contains(r.fakeLog(), `"content":"relayed note"`) })
}

// LastAnswer is the final message of the chat's last turn, which a worker's
// errand is answered by.
func TestLastAnswerIsTheLastTurnsFinalMessage(t *testing.T) {
	r := newRig(t, providers.Claude)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.prompt("question")
	q := r.waitingQuestion()
	if err := r.reply(q.ID, api.ChatQReplied{Declined: true}); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	r.turnEnd(0)
	if got := r.svc.LastAnswer("s1"); got != "declined" {
		t.Errorf("LastAnswer = %q, want the turn's final message", got)
	}
	if got := r.svc.LastAnswer("nobody"); got != "" {
		t.Errorf("LastAnswer of no chat = %q", got)
	}
}
