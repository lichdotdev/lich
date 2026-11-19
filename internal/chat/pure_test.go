package chat

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
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

// A tool call ends when it is first seen ended live; a later report keeps that
// time, and one replayed from a history carries none.
func TestConversationStampsWhenAToolCallEnded(t *testing.T) {
	t.Parallel()
	c := newConversation(func(api.ChatItem) {})
	now := time.Unix(100, 0)
	c.now = func() time.Time { return now }
	call := func(state string) api.ChatItem {
		return api.ChatItem{ID: "k1", Kind: api.ItemTool, Tool: &api.ChatToolCall{Name: "Bash", State: state}}
	}
	ended := func(id string) int64 {
		it, _ := c.get(id)
		return it.Tool.Ended
	}
	c.put(call(api.ToolRunning), true)
	if got := ended("k1"); got != 0 {
		t.Errorf("a running call ended at %d", got)
	}
	now = now.Add(2 * time.Second)
	c.put(call(api.ToolDone), true)
	if got, want := ended("k1"), now.UnixMilli(); got != want {
		t.Errorf("ended at %d, want %d", got, want)
	}
	stamped := now.UnixMilli()
	now = now.Add(time.Second)
	c.put(call(api.ToolDone), true)
	if got := ended("k1"); got != stamped {
		t.Errorf("a second report moved the end to %d, want %d", got, stamped)
	}
	c.setQuiet(true)
	c.put(api.ChatItem{ID: "k2", Kind: api.ItemTool, Tool: &api.ChatToolCall{Name: "Read", State: api.ToolDone}}, true)
	if got := ended("k2"); got != 0 {
		t.Errorf("a replayed call ended at %d, want 0: nobody said when", got)
	}
}
