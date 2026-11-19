//go:build !windows

package chat

import (
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// answers are the agent items of the chat, oldest first.
func (r *rig) answers() []api.ChatItem {
	var out []api.ChatItem
	for _, it := range r.history() {
		if it.Kind == api.ItemAgent {
			out = append(out, it)
		}
	}
	return out
}

// TestClaudeAnswersKeepTheModelThatWroteThem: an item carries the model and
// effort in force when it was written, and a later change of either leaves it
// as it was.
func TestClaudeAnswersKeepTheModelThatWroteThem(t *testing.T) {
	r := newRig(t, providers.Claude)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.prompt("hi")
	r.turnEnd(0)
	r.mustSet(claudeModelOption, "haiku")
	r.mustSet(claudeEffortOption, "medium")
	r.prompt("hi")
	r.turnEnd(1)

	got := r.answers()
	if len(got) != 2 {
		t.Fatalf("%d answers, want 2", len(got))
	}
	if first := got[0]; first.Model != "Opus 5.5" || first.Effort != "high" {
		t.Errorf("first answer is %q at %q, want Opus 5.5 at high (not the alias the default is called)", first.Model, first.Effort)
	}
	if second := got[1]; second.Model != "Haiku 5.5" || second.Effort != "medium" {
		t.Errorf("second answer is %q at %q, want Haiku 5.5 at medium", second.Model, second.Effort)
	}
}

// A model that does not take the effort the process still holds is not given
// one.
func TestClaudeAnswerOfAModelThatTakesNoEffortHasNone(t *testing.T) {
	r := newRig(t, providers.Claude)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.mustSet(claudeModelOption, "old")
	r.prompt("hi")
	r.turnEnd(0)
	if a := r.answers(); len(a) != 1 || a[0].Model != "Old" || a[0].Effort != "" {
		t.Errorf("answers = %+v, want the model Old and no effort", a)
	}
}

// TestClaudeHistoryNamesTheModelOfEachMessage: the transcript says which model
// wrote each message and not at which effort; the ids it holds are named once
// the chat knows its models, and a message of Claude Code's own has none.
func TestClaudeHistoryNamesTheModelOfEachMessage(t *testing.T) {
	r := newRig(t, providers.Claude)
	_ = r.db.SetProviderSession("s1", "conv-1")
	r.writeClaudeTranscript("conv-1",
		`{"type":"user","message":{"content":"q1"}}`,
		`{"type":"assistant","message":{"id":"h1","model":"claude-haiku-5-5","content":[{"type":"text","text":"by haiku"}]}}`,
		`{"type":"user","message":{"content":"q2"}}`,
		`{"type":"assistant","message":{"id":"h2","model":"claude-opus-5-5","content":[{"type":"text","text":"by opus"}]}}`,
		`{"type":"assistant","message":{"id":"h3","model":"<synthetic>","content":[{"type":"text","text":"by the cli"}]}}`,
		`{"type":"assistant","message":{"id":"h4","content":[{"type":"text","text":"by nobody told"}]}}`,
	)
	if _, err := r.term.Start(t.Context(), api.StartOptions{ID: "s1", ProjectID: "p1", Cwd: r.home, Kind: providers.Claude, Resume: "conv-1", Cols: 80, Rows: 24}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	for text, want := range map[string]string{"by haiku": "Haiku 5.5", "by opus": "Opus 5.5", "by the cli": "", "by nobody told": ""} {
		it := r.mustItem(api.ItemAgent, text)
		if it.Model != want || it.Effort != "" {
			t.Errorf("%q: model %q effort %q, want %q and no effort", text, it.Model, it.Effort, want)
		}
	}
}

// TestACPAnswersKeepTheChoiceInForceWhenTheyBegan: an ACP item takes the
// current model and effort by the server's own names, and a later change
// leaves it as it was.
func TestACPAnswersKeepTheChoiceInForceWhenTheyBegan(t *testing.T) {
	r := newRig(t, providers.OMP)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.prompt("hi")
	r.turnEnd(0)
	r.mustSet("model", "m2")
	r.prompt("hi")
	r.turnEnd(1)

	got := r.answers()
	if len(got) != 2 {
		t.Fatalf("%d answers, want 2", len(got))
	}
	if got[0].Model != "M1" || got[0].Effort != "high" {
		t.Errorf("first answer is %q at %q, want M1 at high", got[0].Model, got[0].Effort)
	}
	if got[1].Model != "M2" || got[1].Effort != "" {
		t.Errorf("second answer is %q at %q, want M2 and no effort (m2 has none)", got[1].Model, got[1].Effort)
	}
}
