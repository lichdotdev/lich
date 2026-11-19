//go:build !windows

package chat

import (
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// TestClaudeSwitchRoundTrip: a terminal switched to a chat stops first,
// resumes its conversation in stream-json with its history read from the
// transcript, and switched back resumes the same conversation in a terminal.
func TestClaudeSwitchRoundTrip(t *testing.T) {
	r := newRig(t, providers.Claude)
	_ = r.db.SetProviderSession("s1", "conv-1")
	r.writeClaudeTranscript("conv-1",
		`{"type":"user","message":{"content":"earlier question"},"timestamp":"2026-11-18T10:00:00Z"}`,
		`{"type":"assistant","message":{"id":"h1","content":[{"type":"text","text":"earlier answer"}]}}`,
		`{"type":"assistant","isSidechain":true,"message":{"id":"h2","content":[{"type":"text","text":"side"}]}}`,
		`{"type":"user","isMeta":true,"message":{"content":"meta"}}`,
		`{"type":"user","message":{"content":[{"type":"text","text":"[Request interrupted by user]"}]}}`,
		`not json`,
	)
	if _, err := r.term.Start(t.Context(), api.StartOptions{ID: "s1", ProjectID: "p1", Cwd: r.home, Kind: providers.Claude, Resume: "conv-1", Cols: 80, Rows: 24}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView(chat): %v", err)
	}
	if live, _ := r.term.Live(t.Context(), api.SessionIDOptions{ID: "s1"}); live {
		t.Error("the terminal still runs beside the chat")
	}
	if got := r.db.SessionView("s1"); got != api.ViewChat {
		t.Errorf("stored view = %q, want chat", got)
	}
	if _, ok := r.item(api.ItemUser, "earlier question"); !ok {
		t.Error("history lacks the transcript's user message")
	}
	if _, ok := r.item(api.ItemAgent, "earlier answer"); !ok {
		t.Error("history lacks the transcript's answer")
	}
	if _, ok := r.item(api.ItemUser, "interrupted"); ok {
		t.Error("the interrupt marker became a message")
	}
	if _, ok := r.item(api.ItemAgent, "side"); ok {
		t.Error("history holds a sidechain line")
	}
	if n := r.itemSends("h1/0"); n != 0 {
		t.Errorf("the replayed history reached the window %d times, want 0", n)
	}
	if !strings.Contains(r.fakeLog(), "argv -p --input-format stream-json") || !strings.Contains(r.fakeLog(), "--resume conv-1") {
		t.Errorf("chat argv = %q, want stream-json resuming conv-1", r.fakeLog())
	}

	r.prompt("hi")
	if turn := r.turnEnd(0); turn.StopReason != stopEndTurn {
		t.Errorf("turn ended %q, want end_turn", turn.StopReason)
	}
	if it := r.mustItem(api.ItemAgent, "Oi!"); it.Text != "Oi!" || it.Turn != 2 {
		t.Errorf("answer = %+v, want the whole text in turn 2", it)
	}
	for _, it := range r.history() {
		if it.Kind == api.ItemThought && it.Text == "" {
			t.Error("a redacted thinking block became an empty thought")
		}
	}
	if got := r.term.Status("s1").State; got != stateDone {
		t.Errorf("card state = %q, want done", got)
	}

	if err := r.setView(api.ViewTerminal); err != nil {
		t.Fatalf("SetView(terminal): %v", err)
	}
	if live, _ := r.term.Live(t.Context(), api.SessionIDOptions{ID: "s1"}); !live {
		t.Error("the terminal did not come back")
	}
	if r.svc.running("s1") != nil {
		t.Error("the chat still runs beside the terminal")
	}
	if got := r.db.SessionView("s1"); got != api.ViewTerminal {
		t.Errorf("stored view = %q, want terminal", got)
	}
	// The first terminal is stopped before its fake can log; the chat and the
	// terminal after it both resume conv-1.
	r.eventually("the terminal's resume", func() bool { return strings.Contains(r.fakeLog(), "argv --resume conv-1") })
	var views []string
	for _, e := range r.snapshot() {
		if v, ok := e.data.(api.SessionView); ok {
			views = append(views, v.View)
		}
	}
	if strings.Join(views, ",") != "chat,terminal" {
		t.Errorf("view events = %v, want chat then terminal", views)
	}
}

// TestClaudeToolsAndPermissions reads a tool turn into items, keeps a
// subagent's own messages out, and answers can_use_tool with each option.
func TestClaudeToolsAndPermissions(t *testing.T) {
	r := newRig(t, providers.Claude)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.prompt("tool")
	r.turnEnd(0)
	if _, ok := r.item(api.ItemThought, "reading"); !ok {
		t.Error("no thought item")
	}
	items := r.history()
	var edit, sub *api.ChatItem
	for i := range items {
		switch items[i].ID {
		case "toolu_1":
			edit = &items[i]
		case "toolu_sub":
			sub = &items[i]
		}
		if items[i].Text == "subagent" {
			t.Error("a subagent's message became an item")
		}
	}
	if edit == nil || edit.Tool.State != api.ToolDone || edit.Tool.Output != "edited" || edit.Tool.Title != "a.txt" ||
		!strings.Contains(edit.Tool.Diff, "-x\n+y\n") {
		t.Errorf("edit item = %+v %+v", edit, edit.Tool)
	}
	if sub == nil || sub.Tool.State != api.ToolFailed || sub.Tool.Output != "nope" {
		t.Errorf("failed tool item = %+v", sub)
	}
	if r.db.SessionView("s1") != api.ViewChat {
		t.Error("view not stored")
	}
	if conv, _ := r.db.ProviderSession("s1"); conv != "claude-new" {
		t.Errorf("bound conversation = %q, want the one system/init named", conv)
	}

	for i, tc := range []struct{ option, behavior string }{{api.PermitAllowAlways, "allow"}, {api.PermitRejectOnce, "deny"}} {
		r.prompt("perm")
		var perm api.ChatItem
		r.eventually("permission", func() bool {
			for _, it := range r.history() {
				if it.Kind == api.ItemPermission && it.Permission.Chosen == "" {
					perm = it
					return true
				}
			}
			return false
		})
		if len(perm.Permission.Options) != 3 || perm.Permission.Tool.Name != "Write" {
			t.Fatalf("permission = %+v", perm.Permission)
		}
		if got := r.term.Status("s1").State; got != stateWaiting {
			t.Errorf("card state while asking = %q, want waiting", got)
		}
		if _, err := r.svc.Answer(t.Context(), api.ChatAnswerOptions{ID: "s1", ItemID: perm.ID, OptionID: "nope"}); err == nil {
			t.Error("an unknown option was accepted")
		}
		if _, err := r.svc.Answer(t.Context(), api.ChatAnswerOptions{ID: "s1", ItemID: perm.ID, OptionID: tc.option}); err != nil {
			t.Fatalf("Answer: %v", err)
		}
		r.turnEnd(1 + i)
		r.mustItem(api.ItemAgent, tc.behavior)
		if _, err := r.svc.Answer(t.Context(), api.ChatAnswerOptions{ID: "s1", ItemID: perm.ID, OptionID: tc.option}); err == nil {
			t.Error("a permission was answered twice")
		}
	}
	if !strings.Contains(r.fakeLog(), `"updatedPermissions":[{"type":"setMode"}]`) {
		t.Error("allow_always did not hand back the suggestions")
	}
	if !strings.Contains(r.fakeLog(), "The person declined this tool call.") {
		t.Error("the deny did not tell the agent why")
	}
}

// TestClaudeCancelErrorsAndExit: an interrupt ends the turn cancelled, an error
// result becomes a notice, a request lich does not serve is refused, and a
// process that dies under a turn says why and comes back on the next message.
func TestClaudeCancelErrorsAndExit(t *testing.T) {
	r := newRig(t, providers.Claude)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	if _, err := r.svc.Cancel(t.Context(), api.SessionIDOptions{ID: "s1"}); err == nil {
		t.Error("Cancel with no turn succeeded")
	}
	r.prompt("slow")
	r.mustItem(api.ItemAgent, "counting")
	if _, err := r.svc.Prompt(t.Context(), api.ChatPromptOptions{ID: "s1", Text: "again"}); err == nil {
		t.Error("a second message during a turn was accepted")
	}
	if _, err := r.svc.SetView(t.Context(), api.ChatViewOptions{ID: "s1", View: api.ViewTerminal}); err == nil {
		t.Error("a switch during a turn was accepted")
	}
	if _, err := r.svc.Cancel(t.Context(), api.SessionIDOptions{ID: "s1"}); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if turn := r.turnEnd(0); turn.StopReason != stopCancelled {
		t.Errorf("cancelled turn ended %q", turn.StopReason)
	}

	r.prompt("fail")
	if turn := r.turnEnd(1); turn.StopReason != stopError {
		t.Errorf("failed turn ended %q", turn.StopReason)
	}
	r.mustItem(api.ItemNotice, "API Error: 401")

	r.prompt("ask")
	r.turnEnd(2)
	if !strings.Contains(r.fakeLog(), "lich does not serve hook_callback") {
		t.Error("an unserved control request was not refused")
	}

	r.prompt("die")
	if turn := r.turnEnd(3); turn.StopReason != stopError {
		t.Errorf("a dead process ended the turn %q", turn.StopReason)
	}
	if got := r.term.Status("s1").State; got != stateDone {
		t.Errorf("card state after the process died = %q, want done", got)
	}
	if !r.sent(api.ItemNotice, "boom") {
		t.Error("the process's death was not said with its stderr")
	}
	r.eventually("the dead chat to go", func() bool { return r.svc.running("s1") == nil })

	r.prompt("hi")
	r.turnEnd(4)
	r.mustItem(api.ItemAgent, "Oi!")
	if _, ok := r.item(api.ItemNotice, "boom"); ok {
		t.Error("the restarted chat kept the dead one's items")
	}
}

// TestClaudeConversationThatDidNotComeBack: a resume Claude Code answers with
// another conversation is said in a notice, and the new one is bound.
func TestClaudeConversationThatDidNotComeBack(t *testing.T) {
	r := newRig(t, providers.Claude)
	_ = r.db.SetProviderSession("s1", "drifts")
	r.writeClaudeTranscript("drifts", `{"type":"user","message":{"content":[{"type":"text","text":"listed"}]}}`)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.mustItem(api.ItemUser, "listed")
	r.prompt("hi")
	r.turnEnd(0)
	r.mustItem(api.ItemNotice, "did not come back")
	if conv, _ := r.db.ProviderSession("s1"); conv != "claude-other" {
		t.Errorf("bound conversation = %q", conv)
	}
}

// A Claude Code tool call ends at its result's time, in a history too.
func TestClaudeToolEndsAtItsResult(t *testing.T) {
	r := newRig(t, providers.Claude)
	_ = r.db.SetProviderSession("s1", "conv-t")
	r.writeClaudeTranscript("conv-t",
		`{"type":"assistant","timestamp":"2026-11-19T10:00:00Z","message":{"id":"m1","content":[{"type":"tool_use","id":"b1","name":"Bash","input":{"command":"ls"}}]}}`,
		`{"type":"user","timestamp":"2026-11-19T10:00:02.5Z","message":{"content":[{"type":"tool_result","tool_use_id":"b1","content":"ok"}]}}`,
	)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	for _, it := range r.history() {
		if it.ID != "b1" {
			continue
		}
		if want := time.Date(2026, 11, 19, 10, 0, 2, 5e8, time.UTC).UnixMilli(); it.Tool.Ended != want {
			t.Errorf("ended at %d, want %d", it.Tool.Ended, want)
		}
		return
	}
	t.Fatal("no b1 in the history")
}
