package main

import (
	"testing"

	"gioui.org/io/key"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
	"github.com/lichdotdev/lich/native/i18n"
)

// askProposal is the one ask of Claude Code's plan approval, its options the
// modes to start in by id.
var askProposal = api.ChatAsk{Options: []api.ChatOffer{{Label: "acceptEdits"}, {Label: "default"}}}

// proposalPlan is the plan the agent asks to start on.
const proposalPlan = "## Add a greeting\n\n1. Create `hello.txt` holding `hi`.\n2. Print it back to check it.\n\nNo other file changes."

// proposalFrames is session s1 waiting on a plan item, reply its answer (nil
// while it waits).
func proposalFrames(t *testing.T, reply *api.ChatQReplied) (*window, *frameRig, *chatBackend) {
	t.Helper()
	u, r, f := chatFrames(t, api.ViewChat)
	u.model.chat.history("s1", []backend.ChatItem{
		{ID: "u1", Kind: api.ItemUser, Turn: 1, Text: "plan it"},
		{ID: "p1", Kind: api.ItemQuestion, Turn: 1, Question: &backend.ChatQuestion{
			Questions: []api.ChatAsk{askProposal}, Reply: reply, Proposal: &api.ChatProposal{Plan: proposalPlan},
		}},
	})
	if reply == nil {
		u.model.chat.turn(api.ChatTurn{SessionID: "s1", Running: true})
	}
	r.settle(t)
	return u, r, f
}

// drewPlan is whether the chat laid out item p1's plan as Markdown, which
// the semantic tree does not carry.
func drewPlan(u *window) bool {
	_, ok := u.chats.pane("s1").md["p1"]
	return ok
}

func TestAProposalShowsThePlanAndClaudeCodesChoices(t *testing.T) {
	u, r, f := proposalFrames(t, nil)
	if !drewPlan(u) {
		t.Error("the plan was not drawn")
	}
	for _, s := range []string{i18n.T("chat.proposal.header"), i18n.T("chat.proposal.ask"), i18n.T("chat.proposal.acceptEdits"),
		i18n.T("chat.proposal.manual"), i18n.T("chat.proposal.change")} {
		if !r.shows(s) {
			t.Errorf("the plan card lacks %q:\n%s", s, r.drawn())
		}
	}
	if r.shows("acceptEdits") || r.shows(i18n.T("chat.question.write")) {
		t.Errorf("the card shows a mode's id or a question's field:\n%s", r.drawn())
	}
	writeShot(t, r, "proposal-waiting.png")
	r.click(t, r.at(t, i18n.T("chat.proposal.manual")))
	wantReply(t, f, `"itemId":"p1"`, `"answers":[{"picked":["default"]}]`)
}

func TestAProposalAnswersByKey(t *testing.T) {
	for name, tc := range map[string]struct {
		keys []key.Name
		want string
	}{
		"Enter approves with edits accepted": {[]key.Name{key.NameReturn}, `"picked":["acceptEdits"]`},
		"2 approves each edit":               {[]key.Name{"2"}, `"picked":["default"]`},
		"Esc keeps it planning":              {[]key.Name{key.NameEscape}, `"declined":true`},
	} {
		t.Run(name, func(t *testing.T) {
			_, r, f := proposalFrames(t, nil)
			r.pressKeys(tc.keys...)
			wantReply(t, f, tc.want)
		})
	}
}

func TestAProposalTakesWhatToChange(t *testing.T) {
	_, r, f := proposalFrames(t, nil)
	r.press("3")
	r.router.Queue(key.EditEvent{Text: "add a test"})
	r.frame()
	r.press(key.NameReturn)
	wantReply(t, f, `"other":"add a test"`)
}

func TestAnAnsweredProposalKeepsThePlan(t *testing.T) {
	for name, tc := range map[string]struct {
		reply api.ChatQReplied
		want  string
	}{
		"a mode":          {api.ChatQReplied{Answers: []api.ChatAskAnswer{{Picked: []string{"acceptEdits"}}}}, i18n.T("chat.proposal.acceptEdits")},
		"words":           {api.ChatQReplied{Answers: []api.ChatAskAnswer{{Other: "add a test"}}}, i18n.T("chat.proposal.keep") + ", " + i18n.T("chat.question.own", i18n.Args{"text": "add a test"})},
		"declined":        {api.ChatQReplied{Declined: true}, i18n.T("chat.proposal.keep")},
		"read back as ok": {api.ChatQReplied{Answers: []api.ChatAskAnswer{{}}}, i18n.T("chat.proposal.approved")},
	} {
		t.Run(name, func(t *testing.T) {
			u, r, _ := proposalFrames(t, &tc.reply)
			if !drewPlan(u) {
				t.Error("the answered plan was not drawn")
			}
			for _, s := range []string{i18n.T("chat.question.label", i18n.Args{"header": i18n.T("chat.proposal.header")}), tc.want} {
				if !r.shows(s) {
					t.Errorf("the answered plan lacks %q:\n%s", s, r.drawn())
				}
			}
			if name == "words" {
				writeShot(t, r, "proposal-answered.png")
			}
			if r.shows(i18n.T("chat.proposal.ask")) {
				t.Error("an answered plan still asks")
			}
		})
	}
}
