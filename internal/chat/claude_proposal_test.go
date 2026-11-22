//go:build !windows

package chat

import (
	"reflect"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
	"github.com/lichdotdev/lich/internal/providers"
)

// TestClaudeProposalApproved: ExitPlanMode becomes a question item holding the
// plan, not a tool row and a permission, and a mode picked allows it with the
// input left empty and switches the session to that mode, as the TUI does.
func TestClaudeProposalApproved(t *testing.T) {
	r := newRig(t, providers.Claude)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.prompt("exit-plan")
	q := r.waitingQuestion()
	if q.ID != "toolu_p" || q.Question.Proposal == nil || q.Question.Proposal.Plan != "# Plan\n\n1. Create hello.txt" {
		t.Fatalf("proposal = %+v", q.Question)
	}
	if got := q.Question.Questions; len(got) != 1 || !reflect.DeepEqual(got[0].Options, []api.ChatOffer{{Label: "acceptEdits"}, {Label: "default"}}) {
		t.Fatalf("asks = %+v", got)
	}
	for _, it := range r.history() {
		if it.Kind == api.ItemTool || it.Kind == api.ItemPermission {
			t.Errorf("ExitPlanMode also became a %s item", it.Kind)
		}
	}
	if st := r.term.Status("s1"); st.State != stateWaiting {
		t.Errorf("card state while asking = %q, want waiting", st.State)
	}
	if err := r.reply(q.ID, api.ChatQReplied{Answers: []api.ChatAskAnswer{{Picked: []string{"acceptEdits"}}}}); err != nil {
		t.Fatalf("Reply: %v", err)
	}
	r.turnEnd(0)
	r.mustItem(api.ItemAgent, "approved:{}")
	if !r.logged(`"updatedPermissions":[{"destination":"session","mode":"acceptEdits","type":"setMode"}]`) {
		t.Errorf("the allow did not switch the mode:\n%s", r.fakeLog())
	}
	r.current(api.OptionMode, "acceptEdits")
	if got, _ := r.svc.running("s1").conv.get(q.ID); got.Question.Reply == nil || got.Question.Reply.Answers[0].Picked[0] != "acceptEdits" {
		t.Errorf("reply recorded = %+v", got.Question.Reply)
	}
}

// TestClaudeProposalKeepPlanning: the person's words and no answer both deny
// it, with the words when there are some, and never with an empty message.
func TestClaudeProposalKeepPlanning(t *testing.T) {
	text := prompt.For(prompt.English)
	for _, tc := range []struct {
		name  string
		reply api.ChatQReplied
		want  string
	}{
		{"words", api.ChatQReplied{Answers: []api.ChatAskAnswer{{Other: " add a test "}}}, strings.Replace(text.ChatPlanFeedback, "%s", "add a test", 1)},
		{"declined", api.ChatQReplied{Declined: true}, text.ChatPlanDeclined},
		{"a mode it does not offer", api.ChatQReplied{Answers: []api.ChatAskAnswer{{Picked: []string{"bypassPermissions"}}}}, text.ChatPlanDeclined},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newRig(t, providers.Claude)
			if err := r.setView(api.ViewChat); err != nil {
				t.Fatalf("SetView: %v", err)
			}
			r.prompt("exit-plan")
			q := r.waitingQuestion()
			if err := r.reply(q.ID, tc.reply); err != nil {
				t.Fatalf("Reply: %v", err)
			}
			r.turnEnd(0)
			r.mustItem(api.ItemAgent, "kept:"+tc.want)
			if r.logged("setMode") {
				t.Error("a deny switched the mode")
			}
		})
	}
}

// TestClaudeProposalHistory reads plans back from the transcript: approved,
// denied, and one the process never got to answer.
func TestClaudeProposalHistory(t *testing.T) {
	r := newRig(t, providers.Claude)
	_ = r.db.SetProviderSession("s1", "conv-p")
	use := func(id string) string {
		return `{"type":"assistant","message":{"id":"` + id + `","content":[{"type":"tool_use","id":"` + id + `","name":"ExitPlanMode","input":{"plan":"# Plan ` + id + `","planFilePath":"/p.md"}}]}}`
	}
	r.writeClaudeTranscript("conv-p",
		use("p1"),
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"p1","content":"User has approved your plan."}]},"toolUseResult":{"plan":"# Plan p1"}}`,
		use("p2"),
		`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"p2","is_error":true,"content":"no"}]}}`,
		use("p3"),
	)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	byID := map[string]api.ChatItem{}
	for _, it := range r.history() {
		byID[it.ID] = it
	}
	if got := byID["p1"].Question; got == nil || got.Proposal == nil || got.Proposal.Plan != "# Plan p1" || got.Reply == nil || got.Reply.Declined {
		t.Errorf("p1 = %+v, want the plan approved", got)
	}
	if got := byID["p2"].Question.Reply; got == nil || !got.Declined {
		t.Errorf("p2 reply = %+v, want declined", got)
	}
	if got := byID["p3"].Question.Reply; got == nil || !got.Declined {
		t.Errorf("p3 reply = %+v, want declined: nothing is left to answer it", got)
	}
}
