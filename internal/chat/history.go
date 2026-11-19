package chat

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"os"
	"strings"

	"github.com/lichdotdev/lich/internal/terminal"
)

// readClaudeHistory reads conversation id back from Claude Code's transcript
// into the chat. Claude Code's stream-json does not replay a resumed
// conversation the way an ACP load does, and the transcript holds the same
// message lines the stream writes, in the same shape.
func readClaudeHistory(c *chatSession, id string) {
	path, ok := terminal.ClaudeTranscript(id)
	if !ok {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		slog.Warn("chat: read transcript", "session", c.id, "err", err)
		return
	}
	defer f.Close()
	c.conv.setQuiet(true)
	defer c.conv.setQuiet(false)
	r := bufio.NewReader(f)
	for {
		raw, err := r.ReadBytes('\n')
		if raw := strings.TrimSpace(string(raw)); raw != "" {
			var line claudeLine
			if json.Unmarshal([]byte(raw), &line) == nil {
				readClaudeMessage(c, line, true)
			}
		}
		if err != nil {
			c.declineUnanswered()
			return
		}
	}
}
