package chat

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// Claude Code's compaction, as 2.1.330 writes it: live, system/status
// "compacting" (repeated while it runs) and then a status carrying
// compact_result and compact_error, a system/compact_boundary with the
// tokens, the summary as a user line marked as one, and the /compact
// command's output, "Compacted " and what its hooks printed. The transcript
// holds the boundary, the summary, then the command and its output.
const (
	claudeCompacting     = "compacting"
	claudeCompactSuccess = "success"
	claudeCompactFailed  = "failed"
	claudeCompactBound   = "compact_boundary"
	claudeCompactAuto    = "auto"
	claudeCompactCommand = "/compact"
	// claudeCompacted opens the command's output: the TUI's own line.
	claudeCompacted = "Compacted"
)

// claudeCompactMeta is a boundary's metadata: snake_case live, camelCase in
// the transcript.
type claudeCompactMeta struct {
	Trigger   string `json:"trigger"`
	Pre       int    `json:"pre_tokens"`
	Post      int    `json:"post_tokens"`
	SavedPre  int    `json:"preTokens"`
	SavedPost int    `json:"postTokens"`
}

var (
	commandStderr = regexp.MustCompile(`(?s)^\s*<local-command-stderr>(.*?)</local-command-stderr>\s*$`)
	// claudeSummaryHint is the TUI's "(ctrl+o to see full summary)" line,
	// which the output carries where a terminal could open it.
	claudeSummaryHint = regexp.MustCompile(`(?m)^\(.*to see full summary\)$`)
)

// compactStatus reads a system/status line: compacting opens the compaction,
// a result ends it.
func (d *claudeDriver) compactStatus(line claudeLine) {
	switch {
	case line.Status == claudeCompacting:
		d.c.conv.compactStart(false, 0)
	case line.CompactResult == claudeCompactSuccess:
		d.c.conv.compaction(func(cc *api.ChatCompaction) { cc.State = api.CompactionDone })
	case line.CompactResult == claudeCompactFailed:
		d.c.conv.compaction(func(cc *api.ChatCompaction) {
			cc.State = api.CompactionFailed
			if line.CompactError != "" {
				cc.Error = line.CompactError
			}
		})
	}
}

// readClaudeBoundary reads a compact_boundary, which Claude Code writes only
// once the conversation is compacted. Read back, it opens a compaction of its
// own, after the person's /compact where they typed one: the transcript writes
// the command after it.
func readClaudeBoundary(c *chatSession, line claudeLine, history bool) {
	meta := line.CompactMeta
	if meta == nil {
		meta = line.SavedCompactMeta
	}
	if meta == nil {
		meta = &claudeCompactMeta{}
	}
	fill := func(cc *api.ChatCompaction) {
		cc.Auto = meta.Trigger == claudeCompactAuto
		cc.Before, cc.After = max(meta.Pre, meta.SavedPre), max(meta.Post, meta.SavedPost)
		if cc.State == api.CompactionRunning {
			cc.State = api.CompactionDone
		}
	}
	if !history {
		c.conv.compaction(fill)
		return
	}
	if meta.Trigger != claudeCompactAuto {
		c.conv.userMessage(claudeCompactCommand, nil)
	}
	c.conv.compactOpen(fill)
}

// claudeCompactSummary reads the summary a compaction left as a user line,
// which is not something the person wrote, and reports whether line was one.
func claudeCompactSummary(c *chatSession, line claudeLine) bool {
	if !line.CompactSummary && !line.LiveCompactSummary {
		return false
	}
	text := blockText(line.Message.Content)
	c.conv.compaction(func(cc *api.ChatCompaction) { cc.Summary = strings.TrimSpace(text) })
	return true
}

// claudeCompactLine folds the /compact command's own lines into the open
// compaction, and reports whether text was one: the command read back, which
// the compaction already drew with the person's message, its output, which
// says what the hooks printed, and its error, which says why it failed.
func claudeCompactLine(c *chatSession, text string) bool {
	if _, open := c.conv.openCompaction(); !open {
		return false
	}
	if cmd, ok := parseCommand(text); ok && (cmd == claudeCompactCommand || strings.HasPrefix(cmd, claudeCompactCommand+" ")) {
		turn, typed := c.conv.turnText()
		if typed != cmd {
			c.conv.update("user-"+strconv.Itoa(turn), api.ItemUser, true, func(it *api.ChatItem) { it.Text = cmd })
		}
		return true
	}
	if out, ok := parseStdout(text); ok && strings.HasPrefix(out, claudeCompacted) {
		hooks := strings.TrimSpace(claudeSummaryHint.ReplaceAllString(strings.TrimPrefix(out, claudeCompacted), ""))
		c.conv.compaction(func(cc *api.ChatCompaction) { cc.Hooks = hooks })
		return true
	}
	if m := commandStderr.FindStringSubmatch(text); m != nil {
		reason := strings.TrimSpace(m[1])
		c.conv.compaction(func(cc *api.ChatCompaction) {
			if cc.State != api.CompactionDone && cc.Error == "" {
				cc.State, cc.Error = api.CompactionFailed, reason
			}
		})
		return true
	}
	return false
}

// compactResult settles the compaction a turn's result finds still running:
// Claude Code wrote neither its result nor its boundary, so it did not
// compact, and the result's text is why.
func (d *claudeDriver) compactResult(text string) {
	if cc, ok := d.c.conv.openCompaction(); ok && cc.State == api.CompactionRunning {
		d.c.conv.compaction(func(cc *api.ChatCompaction) { cc.State, cc.Error = api.CompactionFailed, text })
	}
}
