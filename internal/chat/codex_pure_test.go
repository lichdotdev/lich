package chat

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lichdotdev/lich/api"
)

func TestCodexPatchReadsEachChangeKind(t *testing.T) {
	got := codexPatch([]codexChange{
		{Path: "a.txt", Kind: struct {
			Type string `json:"type"`
		}{"update"}, Diff: "@@ -1 +1 @@\n-x\n+y\n"},
		{Path: "b.txt", Kind: struct {
			Type string `json:"type"`
		}{"add"}, Diff: "hi\n"},
		{Path: "c.txt", Kind: struct {
			Type string `json:"type"`
		}{"delete"}, Diff: "bye\n"},
	})
	want := "--- a/a.txt\n+++ b/a.txt\n@@ -1 +1 @@\n-x\n+y\n" +
		"--- /dev/null\n+++ b/b.txt\n@@ -0,0 +1,1 @@\n+hi\n" +
		"--- a/c.txt\n+++ /dev/null\n@@ -1,1 +0,0 @@\n-bye\n"
	if got.Diff != want || got.Title != "a.txt, b.txt, c.txt" || got.Name != codexPatchTool {
		t.Errorf("patch = %+v\nwant diff %q", got, want)
	}
}

func TestCodexModeOfNamesEverySandbox(t *testing.T) {
	for in, want := range map[string]string{
		"readOnly": codexReadOnly, "workspace-write": codexWorkspace, "dangerFullAccess": codexFullAccess, "externalSandbox": "",
	} {
		if got := codexModeOf(codexSandboxP{Type: in}); got != want {
			t.Errorf("codexModeOf(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestCodexOptionsKeepAModelTheAccountLacks: a config.toml model that
// model/list leaves out stays the current one, with no effort to offer.
func TestCodexOptionsKeepAModelTheAccountLacks(t *testing.T) {
	opts := codexOptions(codexRunning{models: []codexModel{{ID: "gpt-a", Efforts: []codexEffort{{Effort: "low"}}}}, model: "gpt-astra", mode: codexReadOnly})
	if len(opts) != 2 || opts[0].Current != "gpt-astra" || opts[1].Category != api.OptionMode {
		t.Errorf("options = %+v", opts)
	}
}

func TestCodexAnswersDeclineAnswersNone(t *testing.T) {
	qs := []codexUserQuestion{{ID: "a"}}
	if got := codexAnswers(qs, api.ChatQReplied{Declined: true}); len(got) != 0 {
		t.Errorf("declined answers = %v", got)
	}
}

func TestCodexTurnProducersReadTheRollout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "r.jsonl")
	lines := `{"type":"session_meta","payload":{"id":"x"}}` + "\n" +
		`{"type":"response_item","payload":{"type":"message","text":"turn_context"}}` + "\n" +
		`{"type":"turn_context","payload":{"turn_id":"t1","model":"gpt-b","effort":"low"}}`
	if err := os.WriteFile(path, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	got := codexTurnProducers(path)
	if len(got) != 1 || got["t1"] != (codexProducer{Model: "gpt-b", Effort: "low"}) {
		t.Errorf("producers = %+v", got)
	}
	if got := codexTurnProducers(filepath.Join(t.TempDir(), "missing")); len(got) != 0 {
		t.Errorf("a missing rollout read %+v", got)
	}
}

// TestCodexItemsTheChatDraws: the person's message, a plan and a compaction
// are not drawn as items of their own; a tool item carries its kind's name.
func TestCodexItemsTheChatDraws(t *testing.T) {
	for _, kind := range []string{"userMessage", "plan", "contextCompaction", "hookPrompt"} {
		if _, ok := (codexItem{Type: kind, ID: "x"}).chatItem(true); ok {
			t.Errorf("%s was drawn", kind)
		}
	}
	for kind, name := range map[string]string{"mcpToolCall": "mcp__srv__run", "webSearch": codexSearchTool, "imageView": codexImageTool, "dynamicToolCall": "run"} {
		ci, ok := (codexItem{Type: kind, ID: "x", Server: "srv", Tool: "run", Status: "failed"}).chatItem(false)
		if !ok || ci.Tool.Name != name || ci.Tool.State != api.ToolFailed {
			t.Errorf("%s = %+v", kind, ci.Tool)
		}
	}
	mcp, _ := (codexItem{Type: "mcpToolCall", ID: "m", Args: []byte(`{"path":"/a"}`), Result: []byte(`{"content":[{"type":"text","text":"read"}]}`)}).chatItem(true)
	if mcp.Tool.Title != "/a" || mcp.Tool.Output != "read" || mcp.Tool.State != api.ToolDone {
		t.Errorf("mcp tool = %+v", mcp.Tool)
	}
}

func TestCodexPatchPathsAreRelativeToTheCheckout(t *testing.T) {
	it := codexItem{Type: "fileChange", ID: "f", Changes: []codexChange{{Path: "/w/src/a.go"}, {Path: "/elsewhere/b.go"}}}
	got := it.under("/w")
	if got.Changes[0].Path != "src/a.go" || got.Changes[1].Path != "/elsewhere/b.go" || it.Changes[0].Path != "/w/src/a.go" {
		t.Errorf("paths = %+v, original %+v", got.Changes, it.Changes)
	}
}
