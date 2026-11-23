//go:build !windows

package chat

import (
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// compactions are the chat's compaction items, in order.
func (r *rig) compactions() []api.ChatItem {
	var out []api.ChatItem
	for _, it := range r.history() {
		if it.Kind == api.ItemCompaction && it.Compaction != nil {
			out = append(out, it)
		}
	}
	return out
}

// kinds is the chat's items as kind:text, the compactions as kind:state.
func (r *rig) kinds() []string {
	var out []string
	for _, it := range r.history() {
		switch {
		case it.Kind == api.ItemCompaction && it.Compaction != nil:
			out = append(out, it.Kind+":"+it.Compaction.State)
		case it.Kind == api.ItemPlan:
		default:
			out = append(out, it.Kind+":"+it.Text)
		}
	}
	return out
}

func (r *rig) wantKinds(want ...string) {
	r.t.Helper()
	if got := r.kinds(); !slices.Equal(got, want) {
		r.t.Fatalf("the chat holds\n  %q\nwant\n  %q", got, want)
	}
}

func (r *rig) onlyCompaction() api.ChatCompaction {
	r.t.Helper()
	cs := r.compactions()
	if len(cs) != 1 {
		r.t.Fatalf("the chat holds %d compactions, want 1: %q", len(cs), r.kinds())
	}
	return *cs[0].Compaction
}

// The shape of a resumed /compact in Claude Code's transcript (2.1.330, an
// anonymised copy of a real one): the boundary, the summary as a user line
// for the transcript alone, the command's caveat, the command and what it
// printed.
var claudeCompactedTranscript = []string{
	`{"type":"user","message":{"content":"fix the parser"},"timestamp":"2026-11-22T10:00:00Z"}`,
	`{"type":"assistant","message":{"id":"a1","content":[{"type":"text","text":"fixed"}]},"timestamp":"2026-11-22T10:00:05Z"}`,
	`{"type":"system","subtype":"compact_boundary","content":"Conversation compacted","isMeta":false,"level":"info","compactMetadata":{"trigger":"manual","preTokens":35043,"postTokens":13792,"durationMs":9155}}`,
	`{"type":"user","isCompactSummary":true,"isVisibleInTranscriptOnly":true,"message":{"role":"user","content":"` + fakeSummary + `"}}`,
	`{"type":"user","isMeta":true,"message":{"content":"<local-command-caveat>The command below was run directly in Claude Code.</local-command-caveat>"}}`,
	`{"type":"user","message":{"content":"<command-name>/compact</command-name>\n            <command-message>compact</command-message>\n            <command-args></command-args>"}}`,
	`{"type":"user","message":{"content":"<local-command-stdout>Compacted PreCompact [hook-a] completed successfully: {}\nPreCompact [hook-b] completed successfully</local-command-stdout>"}}`,
	`{"type":"user","message":{"content":"and now?"}}`,
	`{"type":"assistant","message":{"id":"a2","content":[{"type":"text","text":"now this"}]}}`,
	`{"type":"system","subtype":"compact_boundary","compactMetadata":{"trigger":"auto","preTokens":196000,"postTokens":11000}}`,
	`{"type":"user","isCompactSummary":true,"message":{"content":[{"type":"text","text":"second summary"}]}}`,
	`{"type":"assistant","message":{"id":"a3","content":[{"type":"text","text":"carried on"}]}}`,
}

// TestAResumedCompactionIsAMarkerNotAMessage: a conversation read back from
// a transcript that compacted drew the summary as the person's message and
// the command's output as a notice. It is a compaction, after the person's
// /compact, with its tokens, summary and hook output; the automatic one has
// no /compact before it.
func TestAResumedCompactionIsAMarkerNotAMessage(t *testing.T) {
	r := newRig(t, providers.Claude)
	_ = r.db.SetProviderSession("s1", "conv-1")
	r.writeClaudeTranscript("conv-1", claudeCompactedTranscript...)
	if _, err := r.term.Start(t.Context(), api.StartOptions{ID: "s1", ProjectID: "p1", Cwd: r.home, Kind: providers.Claude, Resume: "conv-1", Cols: 80, Rows: 24}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.wantKinds(
		"user:fix the parser", "agent:fixed",
		"user:/compact", "compaction:done",
		"user:and now?", "agent:now this",
		"compaction:done", "agent:carried on",
	)
	cs := r.compactions()
	manual, auto := *cs[0].Compaction, *cs[1].Compaction
	if manual.Auto || manual.Before != 35043 || manual.After != 13792 {
		t.Errorf("the /compact reads auto %v, %d → %d tokens, want manual, 35043 → 13792", manual.Auto, manual.Before, manual.After)
	}
	if manual.Summary != fakeSummary {
		t.Errorf("its summary is %q, want the transcript's", manual.Summary)
	}
	if want := "PreCompact [hook-a] completed successfully: {}\nPreCompact [hook-b] completed successfully"; manual.Hooks != want {
		t.Errorf("its hooks printed %q, want %q", manual.Hooks, want)
	}
	if !auto.Auto || auto.Before != 196000 || auto.After != 11000 || auto.Summary != "second summary" || auto.Hooks != "" {
		t.Errorf("the automatic one reads %+v", auto)
	}
}

// TestAResumedCompactWithArgumentsKeepsThem: the command read back says what
// the person typed after /compact.
func TestAResumedCompactWithArgumentsKeepsThem(t *testing.T) {
	r := newRig(t, providers.Claude)
	_ = r.db.SetProviderSession("s1", "conv-1")
	r.writeClaudeTranscript("conv-1",
		`{"type":"system","subtype":"compact_boundary","compactMetadata":{"trigger":"manual","preTokens":5000}}`,
		`{"type":"user","message":{"content":"<command-name>/compact</command-name><command-message>compact</command-message><command-args>keep the tests</command-args>"}}`,
		`{"type":"user","message":{"content":"<command-name>/model</command-name><command-args>haiku</command-args>"}}`,
	)
	if _, err := r.term.Start(t.Context(), api.StartOptions{ID: "s1", ProjectID: "p1", Cwd: r.home, Kind: providers.Claude, Resume: "conv-1", Cols: 80, Rows: 24}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.wantKinds("user:/compact keep the tests", "compaction:done", "notice:/model haiku")
}

// TestClaudeCompactsLive: the person's /compact is one compaction while it
// runs, whatever Claude Code repeats, and ends with its tokens, summary and
// hook output, the command's own output folded in.
func TestClaudeCompactsLive(t *testing.T) {
	r := newChat(t, providers.Claude)
	r.prompt("/compact")
	if end := r.turnEnd(0); end.StopReason != stopEndTurn {
		t.Fatalf("the turn ended %q", end.StopReason)
	}
	r.wantKinds("user:/compact", "compaction:done")
	cc := r.onlyCompaction()
	if cc.Auto || cc.Before != 182000 || cc.After != 9000 || cc.Summary != fakeSummary {
		t.Errorf("the compaction reads %+v", cc)
	}
	if want := "PreCompact [hook-a] completed successfully: {}\nPreCompact [hook-b] completed successfully"; cc.Hooks != want {
		t.Errorf("its hooks printed %q, want %q", cc.Hooks, want)
	}
	if !slices.ContainsFunc(r.snapshot(), func(e event) bool {
		u, ok := e.data.(api.ChatItemUpdate)
		return ok && u.Item.Compaction != nil && u.Item.Compaction.State == api.CompactionRunning
	}) {
		t.Error("the window never heard the compaction running")
	}
}

// TestClaudeCompactionFailures: the reason is Claude Code's compact_error,
// its command's error where it gave none, and the turn's end where it said
// nothing at all; no notice repeats it.
func TestClaudeCompactionFailures(t *testing.T) {
	for _, tc := range []struct{ typed, reason string }{
		{"/compact fail", "Not enough messages to compact."},
		{"/compact quiet", "Error: Not enough messages to compact."},
		{"/compact silent", ""},
	} {
		t.Run(tc.typed, func(t *testing.T) {
			r := newChat(t, providers.Claude)
			// Claude Code lists /compact at initialize; the fake's list is
			// what the commands tests read.
			r.eventually("the commands", func() bool { return len(r.commands()) > 0 })
			r.svc.running("s1").setCommands([]api.ChatCommand{{Text: "/compact"}})
			r.prompt(tc.typed)
			r.turnEnd(0)
			r.wantKinds("user:"+tc.typed, "compaction:failed")
			if cc := r.onlyCompaction(); cc.Error != tc.reason {
				t.Errorf("the compaction failed for %q, want %q", cc.Error, tc.reason)
			}
		})
	}
}

// TestClaudeCompactsOnItsOwnMidTurn: an automatic compaction is placed where
// it happened, and the turn carries on after it.
func TestClaudeCompactsOnItsOwnMidTurn(t *testing.T) {
	r := newChat(t, providers.Claude)
	r.prompt("auto")
	r.turnEnd(0)
	r.wantKinds("user:auto", "agent:working", "compaction:done", "agent:carried on")
	if cc := r.onlyCompaction(); !cc.Auto || cc.Before != 196000 || cc.After != 11000 {
		t.Errorf("the compaction reads %+v", cc)
	}
}

// TestCodexCompacts: the person's /compact goes to thread/compact/start, not
// to the model, and its tokens are the thread's before and after; Codex's
// own compaction mid-turn is automatic; one read back is drawn.
func TestCodexCompacts(t *testing.T) {
	r := newChat(t, providers.Codex)
	r.prompt("big")
	r.turnEnd(0)
	r.prompt("/compact")
	r.turnEnd(1)
	if strings.Contains(r.fakeLog(), `"text":"/compact"`) {
		t.Error("/compact went to the model as a message")
	}
	r.prompt("autocompact")
	r.turnEnd(2)
	r.wantKinds("user:big", "agent:lots", "user:/compact", "compaction:done",
		"user:autocompact", "agent:working", "compaction:done", "agent:carried on")
	cs := r.compactions()
	if typed := *cs[0].Compaction; typed.Auto || typed.Before != 182000 || typed.After != 9000 {
		t.Errorf("the /compact reads %+v", typed)
	}
	if !cs[1].Compaction.Auto {
		t.Error("Codex's own compaction reads as typed")
	}
}

// TestCodexCompactionReadBack: a compaction in a turn the person opened is
// Codex's own; one in a turn with no message of theirs was their /compact.
func TestCodexCompactionReadBack(t *testing.T) {
	r := newChat(t, providers.Codex)
	d := r.svc.running("s1").drv.(*codexDriver)
	r.svc.running("s1").conv.setQuiet(true)
	d.replayTurn(codexTurn{ID: "t1", Items: []codexItem{{Type: "userMessage", ID: "u1", Content: []byte(`[{"type":"text","text":"go"}]`)}, {Type: codexCompactItem, ID: "k1"}}}, codexProducer{})
	d.replayTurn(codexTurn{ID: "t2", Items: []codexItem{{Type: codexCompactItem, ID: "k2"}}}, codexProducer{})
	r.svc.running("s1").conv.setQuiet(false)
	r.wantKinds("user:go", "compaction:done", "user:/compact", "compaction:done")
	if cs := r.compactions(); !cs[0].Compaction.Auto || cs[1].Compaction.Auto {
		t.Errorf("read back as auto %v and %v, want true and false", cs[0].Compaction.Auto, cs[1].Compaction.Auto)
	}
}

// TestACPAdvertisesCompaction: lich tells an ACP agent it draws compactions,
// the only way the spec lets one send them.
func TestACPAdvertisesCompaction(t *testing.T) {
	r := newChat(t, providers.OpenCode)
	r.prompt("hi")
	r.turnEnd(0)
	if !strings.Contains(r.fakeLog(), `"session":{"compaction":{}}`) {
		t.Errorf("initialize did not advertise session.compaction:\n%s", r.fakeLog())
	}
}

// TestACPTypedCompactIsACompactionTurn: an agent that says nothing of its
// compaction (opencode) still compacts on its listed /compact: the turn is
// the compaction, and usage_update gives its tokens.
func TestACPTypedCompactIsACompactionTurn(t *testing.T) {
	r := newChat(t, providers.OpenCode)
	r.waitCommands([]api.ChatCommand{
		{Text: "/compact", Description: "Compact the conversation"},
		{Text: "/switch", Description: "Switch model for this session only", Hint: "[model]"},
	})
	r.prompt("big")
	r.turnEnd(0)
	r.prompt("/compact")
	r.turnEnd(1)
	r.wantKinds("user:big", "agent:lots", "user:/compact", "compaction:done", "agent:Compaction complete.")
	if cc := r.onlyCompaction(); cc.Auto || cc.Before != 182000 || cc.After != 9000 {
		t.Errorf("the compaction reads %+v", cc)
	}
}

// TestACPCompactionUpdates: compaction_update and its summary chunks, by id;
// a failure carries its error, and a status lich does not know changes
// nothing.
func TestACPCompactionUpdates(t *testing.T) {
	r := newChat(t, providers.OpenCode)
	r.prompt("spec")
	r.turnEnd(0)
	r.wantKinds("user:spec", "agent:working", "compaction:done", "compaction:failed", "compaction:done")
	cs := r.compactions()
	if c := *cs[0].Compaction; !c.Auto || c.Summary != "Summary so far" {
		t.Errorf("the first compaction reads %+v", c)
	}
	if c := *cs[1].Compaction; c.Error != "the model refused" || c.Summary != "" {
		t.Errorf("the failed one reads %+v", c)
	}
}

// TestKiroCompactionStatus: Kiro's own notice opens and ends a compaction,
// its summary and error read; an unknown status is left alone.
func TestKiroCompactionStatus(t *testing.T) {
	r := newChat(t, providers.OpenCode)
	r.prompt("kiro")
	r.turnEnd(0)
	r.wantKinds("user:kiro", "compaction:done", "compaction:failed")
	cs := r.compactions()
	if c := *cs[0].Compaction; c.Summary != "kiro summary" || !c.Auto {
		t.Errorf("the first compaction reads %+v", c)
	}
	if c := *cs[1].Compaction; c.Error != "context overflow" {
		t.Errorf("the failed one reads %+v", c)
	}
}

// TestCompactStateByTurnEnd: a compaction its provider left running ends as
// its turn did, a cancelled one drawn nowhere.
func TestCompactStateByTurnEnd(t *testing.T) {
	for reason, want := range map[string]string{
		stopEndTurn:   api.CompactionDone,
		stopCancelled: api.CompactionCancelled,
		stopError:     api.CompactionFailed,
	} {
		if got := compactState(reason); got != want {
			t.Errorf("compactState(%q) = %q, want %q", reason, got, want)
		}
	}
}

// TestACompactionsAfterIsTheFirstCountUnderItsBefore: a count no smaller than
// the conversation it folded is not what it left.
func TestACompactionsAfterIsTheFirstCountUnderItsBefore(t *testing.T) {
	c := &chatSession{conv: newConversation(func(api.ChatItem) {})}
	c.reading(1000)
	c.conv.compactStart(false, c.counted())
	c.reading(1200)
	c.conv.compactClose(api.CompactionDone)
	c.reading(300)
	c.reading(200)
	got, _ := c.conv.compactionItem(compactionID(1))
	if got.Before != 1000 || got.After != 300 {
		t.Errorf("the compaction reads %d → %d, want 1000 → 300", got.Before, got.After)
	}
}

// TestContentText reads a summary in each shape ACP and Kiro give it.
func TestContentText(t *testing.T) {
	for raw, want := range map[string]string{
		`"plain"`:                      "plain",
		`{"type":"text","text":"one"}`: "one",
		`[{"type":"text","text":"a"},{"type":"image"},{"type":"text","text":"b"}]`: "a\nb",
		`null`: "",
		`[]`:   "",
	} {
		if got := contentText([]byte(raw)); got != want {
			t.Errorf("contentText(%s) = %q, want %q", raw, got, want)
		}
	}
	if got := reasonText([]byte(`{"message":"why"}`)); got != "why" {
		t.Errorf("reasonText of an object = %q", got)
	}
}
