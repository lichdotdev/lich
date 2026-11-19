package chat

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lichdotdev/lich/api"
)

// outputLimit bounds a tool item's Output: the item is a short reading of the
// result, and a file the agent read in full would otherwise ride every update.
const outputLimit = 4000

// acpToolCall is a tool call as ACP reports it, and as each server bends it:
// Antigravity adds the tool's own name, Kiro its own under _meta.
type acpToolCall struct {
	ID       string          `json:"toolCallId"`
	Title    string          `json:"title"`
	ToolKind string          `json:"kind"`
	Status   string          `json:"status"`
	Name     string          `json:"name"`
	Body     json.RawMessage `json:"content"`
	Meta     struct {
		Kiro struct {
			Tool string `json:"toolName"`
		} `json:"kiro"`
	} `json:"_meta"`
}

// acpStates maps ACP's tool statuses onto the chat's.
var acpStates = map[string]string{
	"pending":     api.ToolPending,
	"in_progress": api.ToolRunning,
	"completed":   api.ToolDone,
	"failed":      api.ToolFailed,
}

// call is prev with what this report adds: an update repeats only the fields
// that changed.
func (t acpToolCall) call(prev api.ChatToolCall) api.ChatToolCall {
	next := prev
	for _, name := range []string{t.Meta.Kiro.Tool, t.Name, t.ToolKind} {
		if name != "" {
			next.Name = name
			break
		}
	}
	if t.Title != "" {
		next.Title = t.Title
	}
	if state, ok := acpStates[t.Status]; ok {
		next.State = state
	}
	if next.State == "" {
		next.State = api.ToolPending
	}
	var blocks []struct {
		Type    string     `json:"type"`
		Content acpContent `json:"content"`
		Path    string     `json:"path"`
		Old     *string    `json:"oldText"`
		New     string     `json:"newText"`
	}
	_ = json.Unmarshal(t.Body, &blocks)
	var texts, diffs []string
	for _, b := range blocks {
		switch b.Type {
		case "content":
			if b.Content.Type == "text" {
				texts = append(texts, b.Content.Text)
			}
		case "diff":
			old := ""
			if b.Old != nil {
				old = *b.Old
			}
			diffs = append(diffs, unifiedDiff(b.Path, old, b.New))
		}
	}
	if len(texts) > 0 {
		next.Output = shorten(strings.Join(texts, "\n"))
	}
	if len(diffs) > 0 {
		next.Diff = strings.Join(diffs, "")
	}
	return next
}

// shorten cuts text to outputLimit bytes, on a rune boundary.
func shorten(text string) string {
	if len(text) <= outputLimit {
		return text
	}
	cut := outputLimit
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut] + "…"
}

// unifiedDiff is old against new as one hunk spanning both.
//
// shortcut: every line is shown removed and added rather than only the ones
// that changed; compute a real diff when the window draws long edits.
func unifiedDiff(path, old, next string) string {
	if old == next {
		return ""
	}
	before, after := diffLines(old), diffLines(next)
	from, to := "a/"+path, "b/"+path
	if old == "" {
		from = "/dev/null"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "--- %s\n+++ %s\n@@ -%s +%s @@\n", from, to, hunkRange(len(before)), hunkRange(len(after)))
	for _, line := range before {
		b.WriteString("-" + line + "\n")
	}
	for _, line := range after {
		b.WriteString("+" + line + "\n")
	}
	return b.String()
}

func diffLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// hunkRange is a hunk's start,count in unified diff's spelling: an empty side
// starts at 0.
func hunkRange(n int) string {
	if n == 0 {
		return "0,0"
	}
	return fmt.Sprintf("1,%d", n)
}

// stamp reads a transcript line's timestamp as unix milliseconds, 0 for none.
func stamp(ts string) int64 {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return 0
	}
	return t.UnixMilli()
}
