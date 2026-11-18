package chat

import (
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// TestACPResumeReplaysTheConversation: session/load reproduces the history
// into the chat without reaching the window, and the turns after it stream.
func TestACPResumeReplaysTheConversation(t *testing.T) {
	r := newRig(t, providers.OMP)
	_ = r.db.SetProviderSession("s1", "omp-conv")
	r.writeOMPSession("omp-conv")
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	if !strings.Contains(r.fakeLog(), "argv acp") || !strings.Contains(r.fakeLog(), `"session/load"`) {
		t.Errorf("fake log = %q, want acp loading the conversation", r.fakeLog())
	}
	r.mustItem(api.ItemUser, "remember caju")
	if it := r.mustItem(api.ItemAgent, "ok"); it.Turn != 1 {
		t.Errorf("replayed answer turn = %d, want 1", it.Turn)
	}
	if len(r.snapshot()) == 0 {
		t.Fatal("no events at all")
	}
	for _, e := range r.snapshot() {
		if _, ok := e.data.(api.ChatItemUpdate); ok {
			t.Errorf("the replay reached the window: %+v", e.data)
		}
	}

	r.prompt("hi")
	if turn := r.turnEnd(0); turn.StopReason != stopEndTurn {
		t.Errorf("turn ended %q", turn.StopReason)
	}
	r.mustItem(api.ItemThought, "hmm")
	if it := r.mustItem(api.ItemAgent, "Oi!"); it.Text != "Oi!" || it.Turn != 2 {
		t.Errorf("answer = %+v, want one item in turn 2", it)
	}
	users := 0
	for _, it := range r.history() {
		if it.Kind == api.ItemUser && it.Turn == 2 {
			users++
		}
	}
	if users != 1 {
		t.Errorf("turn 2 has %d user items, want 1: the echo must not repeat the message", users)
	}
	if conv, _ := r.db.ProviderSession("s1"); conv != "omp-conv" {
		t.Errorf("bound conversation = %q", conv)
	}
}

// TestACPToolsPermissionsAndErrors walks a tool update into one item, answers
// a permission, cancels a turn and one waiting on a permission, turns a
// JSON-RPC error into a notice and refuses the client methods lich does not
// serve.
func TestACPToolsPermissionsAndErrors(t *testing.T) {
	r := newRig(t, providers.OMP)
	_ = r.db.SetSessionModel("s1", "cheap-model")
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	if conv, _ := r.db.ProviderSession("s1"); conv != "ses-new" {
		t.Errorf("bound conversation = %q, want the one session/new opened", conv)
	}
	if !strings.Contains(r.fakeLog(), `"configId":"model","sessionId":"ses-new","value":"cheap-model"`) {
		t.Errorf("the birth model was not set: %q", r.fakeLog())
	}

	r.prompt("tool")
	r.turnEnd(0)
	var tool api.ChatItem
	for _, it := range r.history() {
		if it.ID == "tool-t1" {
			tool = it
		}
	}
	if tool.Tool == nil || tool.Tool.Name != "write" || tool.Tool.State != api.ToolDone || tool.Tool.Output != "wrote" ||
		tool.Tool.Title != "Editing a.txt" || !strings.Contains(tool.Tool.Diff, "+y") {
		t.Errorf("tool item = %+v", tool.Tool)
	}

	r.prompt("perm")
	perm := r.waitPermission()
	if perm.Permission.Options[0].Label != "Allow once" || perm.Permission.Tool.Title != "/outside" {
		t.Errorf("permission = %+v", perm.Permission)
	}
	if _, err := r.svc.Answer(t.Context(), api.ChatAnswerOptions{ID: "s1", ItemID: perm.ID, OptionID: "once"}); err != nil {
		t.Fatalf("Answer: %v", err)
	}
	r.turnEnd(1)
	r.mustItem(api.ItemAgent, "selected:once")

	r.prompt("slow")
	r.mustItem(api.ItemAgent, "counting")
	_, _ = r.svc.Cancel(t.Context(), api.SessionIDOptions{ID: "s1"})
	if turn := r.turnEnd(2); turn.StopReason != stopCancelled {
		t.Errorf("cancelled turn ended %q", turn.StopReason)
	}

	r.prompt("permslow")
	r.waitPermission()
	_, _ = r.svc.Cancel(t.Context(), api.SessionIDOptions{ID: "s1"})
	r.turnEnd(3)
	r.mustItem(api.ItemAgent, "after:cancelled")

	r.prompt("err")
	if turn := r.turnEnd(4); turn.StopReason != stopError {
		t.Errorf("failed turn ended %q", turn.StopReason)
	}
	r.mustItem(api.ItemNotice, "Model access is disabled")

	r.prompt("fs")
	r.turnEnd(5)
	r.mustItem(api.ItemAgent, "-32601")

	r.prompt("die")
	r.turnEnd(6)
	if !r.sent(api.ItemNotice, "acp boom") {
		t.Error("the process's death was not said with its stderr")
	}
}

func (r *rig) waitPermission() api.ChatItem {
	r.t.Helper()
	var perm api.ChatItem
	r.eventually("permission", func() bool {
		for _, it := range r.history() {
			if it.Kind == api.ItemPermission && it.Permission.Chosen == "" && it.Permission.Tool.State != api.ToolFailed {
				perm = it
				return true
			}
		}
		return false
	})
	return perm
}

// TestACPConversationsThatDidNotComeBack: every way a resume can fail is said,
// and the chat goes on in a new conversation.
func TestACPConversationsThatDidNotComeBack(t *testing.T) {
	cases := []struct {
		name, conv, notice string
		onDisk             bool
		noLoad             bool
	}{
		{"the load fails", "gone", "session not found", true, false},
		{"the load brings nothing back", "omp-empty", "came back empty", true, false},
		{"the agent no longer has it", "pruned", "no longer has it", false, false},
		{"the agent cannot load", "omp-conv", "cannot reopen", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.noLoad {
				t.Setenv("LICH_CHAT_FAKE_NOLOAD", "1")
			}
			r := newRig(t, providers.OMP)
			_ = r.db.SetProviderSession("s1", tc.conv)
			if tc.onDisk {
				r.writeOMPSession(tc.conv)
			}
			if err := r.setView(api.ViewChat); err != nil {
				t.Fatalf("SetView: %v", err)
			}
			r.mustItem(api.ItemNotice, tc.notice)
			conv, _ := r.db.ProviderSession("s1")
			if want := map[bool]string{true: "omp-empty", false: "ses-new"}[tc.conv == "omp-empty"]; conv != want {
				t.Errorf("bound conversation = %q, want %q", conv, want)
			}
		})
	}
}
