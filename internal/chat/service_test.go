package chat

import (
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// TestConversationHoldsAnItemToItsGap: a burst of changes to one item reaches
// the window once now and once with its last text, and a final change or a
// flush never waits.
func TestConversationHoldsAnItemToItsGap(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var sent []api.ChatItem
	c := newConversation(func(it api.ChatItem) {
		mu.Lock()
		sent = append(sent, it)
		mu.Unlock()
	})
	now := time.Unix(100, 0)
	c.now = func() time.Time { return now }
	for _, part := range []string{"a", "b", "c", "d"} {
		c.update("x", api.ItemAgent, false, func(it *api.ChatItem) { it.Text += part })
	}
	mu.Lock()
	if len(sent) != 1 || sent[0].Text != "a" {
		t.Errorf("sent during the burst = %+v, want the first change alone", sent)
	}
	mu.Unlock()
	c.flush()
	mu.Lock()
	if len(sent) != 2 || sent[1].Text != "abcd" {
		t.Errorf("sent after the flush = %+v, want the whole text", sent)
	}
	mu.Unlock()

	c.update("y", api.ItemAgent, false, func(it *api.ChatItem) { it.Text = "1" })
	c.update("y", api.ItemAgent, false, func(it *api.ChatItem) { it.Text = "12" })
	deadline := time.Now().Add(2 * time.Second)
	for {
		mu.Lock()
		n := len(sent)
		last := sent[n-1]
		mu.Unlock()
		if n == 4 && last.Text == "12" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the held change never left: %+v", sent)
		}
		time.Sleep(5 * time.Millisecond)
	}
	c.put(api.ChatItem{ID: "y", Kind: api.ItemAgent, Text: "123"}, true)
	mu.Lock()
	defer mu.Unlock()
	if got := sent[len(sent)-1]; got.Text != "123" || got.At != sent[2].At {
		t.Errorf("final change = %+v, want it sent now keeping its start", got)
	}
}

func TestToolHelpers(t *testing.T) {
	t.Parallel()
	if got := unifiedDiff("a.go", "", "one\ntwo\n"); got != "--- /dev/null\n+++ b/a.go\n@@ -0,0 +1,2 @@\n+one\n+two\n" {
		t.Errorf("new file diff = %q", got)
	}
	if got := unifiedDiff("a.go", "same", "same"); got != "" {
		t.Errorf("unchanged diff = %q, want none", got)
	}
	long := strings.Repeat("é", outputLimit)
	if got := shorten(long); len(got) > outputLimit+len("…") || !strings.HasSuffix(got, "…") {
		t.Errorf("shorten kept %d bytes", len(got))
	}
	if got := shorten("short"); got != "short" {
		t.Errorf("shorten(short) = %q", got)
	}
	if stamp("2026-11-18T10:00:00Z") != time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC).UnixMilli() || stamp("") != 0 {
		t.Error("stamp misread a timestamp")
	}
	if got := exitNotice(""); got != "The agent's process exited." {
		t.Errorf("exitNotice = %q", got)
	}
	if got := lastLines("1\n2\n3", 2); got != "2\n3" {
		t.Errorf("lastLines = %q", got)
	}
}

// TestSetViewRefuses every switch the session cannot make.
func TestSetViewRefuses(t *testing.T) {
	r := newRig(t, providers.OMP)
	if kinds, _ := r.svc.Kinds(t.Context(), api.None{}); !slices.Contains(kinds, providers.OMP) || slices.Contains(kinds, providers.Codex) {
		t.Errorf("Kinds = %v", kinds)
	}
	if err := r.setView("sideways"); err == nil {
		t.Error("an unknown view was accepted")
	}
	if _, err := r.svc.SetView(t.Context(), api.ChatViewOptions{ID: "missing", View: api.ViewChat}); err == nil {
		t.Error("a missing session switched")
	}
	if _, err := r.svc.History(t.Context(), api.SessionIDOptions{ID: "s1"}); err == nil {
		t.Error("History of a terminal session succeeded")
	}
	if _, err := r.svc.Answer(t.Context(), api.ChatAnswerOptions{ID: "s1"}); err == nil {
		t.Error("Answer with no chat succeeded")
	}
	r.term.ReportChatState("s1", stateBusy, "", "")
	if err := r.setView(api.ViewChat); err == nil {
		t.Error("a switch during the terminal's turn was accepted")
	}
	r.term.ReportChatState("s1", stateDone, "", "")

	release, _ := r.svc.claim("s1")
	if err := r.setView(api.ViewChat); err == nil {
		t.Error("a switch during another switch was accepted")
	}
	release()

	_, _ = r.db.AddSession(t.Context(), api.AddSessionOptions{ProjectID: "p1", SessionID: "codex", Label: "C", Kind: providers.Codex, NextSeq: 3})
	if _, err := r.svc.SetView(t.Context(), api.ChatViewOptions{ID: "codex", View: api.ViewChat}); err == nil {
		t.Error("a Codex session became a chat")
	}

	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	if err := r.setView(api.ViewChat); err != nil {
		t.Errorf("switching a chat to a chat = %v, want nil", err)
	}
	r.svc.End("s1")
	if r.svc.running("s1") != nil {
		t.Error("End left the chat running")
	}
	r.svc.End("s1")
}

// TestAutoApproveAndBirthEffort: a checkout that skips permissions answers
// yes itself where the ACP server has no flag for it, and the effort the
// session was born with is set over the protocol.
func TestAutoApproveAndBirthEffort(t *testing.T) {
	r := newRig(t, providers.OMP)
	_, _ = r.db.SetSetting(t.Context(), api.SetSettingOptions{Key: "provider.omp.skip-permissions", Value: "true"})
	_ = r.db.SetSessionEffort("s1", "low")
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	if !strings.Contains(r.fakeLog(), `"configId":"thinking","sessionId":"ses-new","value":"low"`) {
		t.Errorf("the birth effort was not set: %q", r.fakeLog())
	}
	r.prompt("perm")
	r.turnEnd(0)
	r.mustItem(api.ItemAgent, "selected:once")
	for _, it := range r.history() {
		if it.Kind == api.ItemPermission {
			t.Error("an auto-approved question became an item")
		}
	}
	if opt, ok := allowOption([]api.ChatPermitOption{{ID: "r", Kind: api.PermitRejectOnce}}); ok {
		t.Errorf("allowOption picked %+v from a reject-only question", opt)
	}
}

// TestShutdownStopsEveryChat ends every process on the way out.
func TestShutdownStopsEveryChat(t *testing.T) {
	r := newRig(t, providers.OMP)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	c := r.svc.running("s1")
	r.svc.Shutdown()
	select {
	case <-c.proc.Done():
	default:
		t.Fatal("Shutdown returned with a chat running")
	}
}
