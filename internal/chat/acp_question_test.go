//go:build !windows

package chat

import (
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// TestACPElicitationIsAQuestion: a form elicitation becomes a question item,
// its other properties folded into the asks they belong to, and the reply
// goes back as the form's content by each choice's const.
func TestACPElicitationIsAQuestion(t *testing.T) {
	r := newRig(t, providers.Antigravity)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	if !strings.Contains(r.fakeLog(), `"elicitation":{"form":{}}`) {
		t.Error("initialize did not declare form elicitation")
	}
	r.prompt("question")
	q := r.waitingQuestion()
	asks := q.Question.Questions
	if len(asks) != 3 || asks[0].Text != "Which fruit?" || asks[0].Header != "Fruit" || asks[1].Header != "" || asks[0].Multi || len(asks[0].Options) != 2 ||
		asks[0].Options[1].Label != "Pear" || !asks[1].Multi || len(asks[2].Options) != 0 {
		t.Fatalf("question = %+v", asks)
	}
	if got := r.term.Status("s1").State; got != stateWaiting {
		t.Errorf("card state = %q, want waiting", got)
	}
	answers := []api.ChatAskAnswer{{Picked: []string{"Pear"}}, {Picked: []string{"Red", "Blue"}, Other: "teal"}, {Other: "a mango"}}
	if err := r.reply(q.ID, api.ChatQReplied{Answers: answers}); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	r.turnEnd(0)
	r.mustItem(api.ItemAgent, `elicit:{"action":"accept","content":{"q0":"2","q1":["r","b"],"q1_other":"teal","q2":"a mango"}}`)

	r.prompt("question")
	q = r.waitingQuestion()
	if err := r.reply(q.ID, api.ChatQReplied{Declined: true}); err != nil {
		t.Fatalf("Reply(declined): %v", err)
	}
	r.turnEnd(1)
	r.mustItem(api.ItemAgent, `elicit:{"action":"decline"}`)

	r.prompt("question-url")
	r.turnEnd(2)
	if strings.Count(r.fakeLog(), `"action":"decline"`) != 2 {
		t.Error("an elicitation that is not a form was not declined")
	}
}

// TestACPMessagePastAQuestion: ACP takes no message during a turn, so one
// written while the agent waits declines its questions, stops the turn and
// is sent once it ended.
func TestACPMessagePastAQuestion(t *testing.T) {
	r := newRig(t, providers.OMP)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.prompt("question-then-write")
	r.waitingQuestion()
	r.prompt("hi")
	if turn := r.turnEnd(0); turn.StopReason != stopCancelled {
		t.Errorf("the waiting turn ended %q, want cancelled", turn.StopReason)
	}
	if turn := r.turnEnd(1); turn.StopReason != stopEndTurn {
		t.Errorf("the message's turn ended %q", turn.StopReason)
	}
	r.mustItem(api.ItemAgent, `elicit:{"action":"decline"}`)
	r.mustItem(api.ItemAgent, "Oi!")
	users := 0
	for _, it := range r.history() {
		if it.Kind == api.ItemUser && it.Text == "hi" {
			users++
		}
	}
	if users != 1 {
		t.Errorf("the message shows %d times, want once", users)
	}
}
