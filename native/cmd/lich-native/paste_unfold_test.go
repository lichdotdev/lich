package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/native/gioterm"
)

const unfoldRows = 40

func numberedLines(count int) string {
	lines := make([]string, count)
	for i := range lines {
		lines[i] = "line " + string(rune('a'+i%26))
	}
	return strings.Join(lines, "\n")
}

func TestUnfoldPasteForClaudeCode(t *testing.T) {
	cases := []struct {
		name, text string
		rows       int
		folds      bool
	}{
		{"above 800 characters", strings.Repeat("x", 801), unfoldRows, true},
		{"at 800 characters", strings.Repeat("x", 800), unfoldRows, false},
		{"above two newlines", numberedLines(4), unfoldRows, true},
		{"at two newlines", numberedLines(3), unfoldRows, false},
		{"CRLF counted once", "a\r\nb\r\nc", unfoldRows, false},
		{"CR counted as newline", "a\rb\rc\rd", unfoldRows, true},
		{"tab counted as four spaces", strings.Repeat("\t", 201), unfoldRows, true},
		{"short terminal tightens the newline limit", "a\nb", 10, true},
		{"the limit eases back", "a\nb", 11, false},
		{"at the size Claude Code still unfolds", strings.Repeat("x", 100_000), unfoldRows, true},
		{"past the size Claude Code unfolds", strings.Repeat("x", 100_001), unfoldRows, false},
	}
	for _, c := range cases {
		got := unfoldPaste("claude", c.text, "P", c.rows)
		if want := map[bool]string{true: "P", false: ""}[c.folds]; got != want {
			t.Errorf("%s: got %q, want %q", c.name, got, want)
		}
	}
}

// The edges below are the ones measured against a real Kiro CLI 2.21.0.
func TestUnfoldPasteForKiroCLI(t *testing.T) {
	cases := []struct {
		name, text string
		folds      bool
	}{
		{"above 10 lines", numberedLines(11), true},
		{"at 10 lines", numberedLines(10), false},
		{"a trailing newline opens another line", numberedLines(10) + "\n", true},
		{"CRLF counted once", strings.ReplaceAll(numberedLines(10), "\n", "\r\n"), false},
		{"above 500 UTF-16 units", strings.Repeat("x", 501), true},
		{"at 500 UTF-16 units", strings.Repeat("x", 500), false},
		{"an emoji is two units", strings.Repeat("😀", 251), true},
		{"250 emoji are 500 units", strings.Repeat("😀", 250), false},
	}
	for _, c := range cases {
		got := unfoldPaste("kiro", c.text, "P", unfoldRows)
		if want := map[bool]string{true: "\t", false: ""}[c.folds]; got != want {
			t.Errorf("%s: got %q, want %q", c.name, got, want)
		}
	}
	if got := unfoldPaste("kiro", numberedLines(4), "P", 5); got != "" {
		t.Errorf("Kiro's rule took the terminal's height: %q", got)
	}
}

func TestUnfoldPasteLeavesOtherProvidersAlone(t *testing.T) {
	for _, kind := range []string{"shell", "codex", "opencode", "cursor"} {
		if got := unfoldPaste(kind, numberedLines(40), "P", unfoldRows); got != "" {
			t.Errorf("%s: got %q", kind, got)
		}
	}
}

// injectedWrite is what the next terminal.Write typed into the session.
func injectedWrite(t *testing.T, b *gateBackend) string {
	t.Helper()
	for {
		select {
		case call := <-b.calls:
			body, found := strings.CutPrefix(call, "terminal.Write ")
			if !found {
				continue
			}
			var args []string
			if err := json.Unmarshal([]byte(body), &args); err != nil {
				t.Fatal(err)
			}
			return args[1]
		case <-time.After(5 * time.Second):
			t.Fatal("no terminal.Write")
		}
	}
}

func claudeTerminals(t *testing.T, b *gateBackend) *terminals {
	t.Helper()
	ts := &terminals{client: clientOf(b.srv), byID: map[string]*termView{}}
	vt, err := gioterm.New(80, unfoldRows, 10, 20, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vt.Close)
	ts.byID["s"] = &termView{id: "s", kind: "claude", vt: vt}
	return ts
}

// A batch of review comments is long enough for Claude Code to fold, and
// with Unfold long pastes on it lands pasted twice, which unfolds the chip.
func TestBatchOfCommentsUnfoldsInClaudeCode(t *testing.T) {
	b := newGateBackend(t)
	b.answer["store.GetSetting"] = "true"
	ts := claudeTerminals(t, b)
	batch := composeReviewComments([]reviewComment{
		{path: "a.go", lines: "1", txt: "one"},
		{path: "b.go", lines: "2", txt: "two"},
		{path: "c.go", lines: "3", txt: "three"},
	}, "")
	if !ts.injector("s")(batch) {
		t.Fatal("inject refused")
	}
	if got := injectedWrite(t, b); got != batch+batch {
		t.Errorf("wrote %q, want the batch twice", got)
	}
}

func TestBatchOfCommentsStaysFoldedWithTheSwitchOff(t *testing.T) {
	b := newGateBackend(t)
	b.answer["store.GetSetting"] = ""
	ts := claudeTerminals(t, b)
	batch := composeReviewComments([]reviewComment{
		{path: "a.go", lines: "1", txt: "one"},
		{path: "b.go", lines: "2", txt: "two"},
		{path: "c.go", lines: "3", txt: "three"},
	}, "")
	ts.injector("s")(batch)
	if got := injectedWrite(t, b); got != batch {
		t.Errorf("wrote %q, want the batch once", got)
	}
}

// Dropping enough files writes a paste Claude Code would fold, and it lands
// unfolded the same as any other.
func TestDropOfManyFilesUnfoldsInClaudeCode(t *testing.T) {
	u, b := newGateWindow(t)
	b.answer["store.GetSetting"] = "true"
	u.terms = claudeTerminals(t, b)
	dir := t.TempDir()
	var paths []string
	for i := range 12 {
		path := filepath.Join(dir, strings.Repeat("a-file-with-a-long-name-", 3)+strconv.Itoa(i))
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	u.dropFiles(dropTarget{sessionID: "s", checkout: dir}, paths)
	paste := composeDroppedPaths(paths, false, nil)
	if got := injectedWrite(t, b); got != paste+paste {
		t.Errorf("wrote %q, want the drop twice", got)
	}
}
