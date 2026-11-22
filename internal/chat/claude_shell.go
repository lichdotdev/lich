package chat

import (
	"encoding/json"
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// A shell command, the TUI's `!` or stream-json's bash_command, is written as
// two user lines of tagged text: the command, then its output or why it did
// not run, XML-escaped. Live, both come as replay rows; the TUI's land in the
// transcript, bash_command's do not (measured on 2.1.330).
var (
	bashInput  = regexp.MustCompile(`(?s)^\s*<bash-input>(.*?)</bash-input>\s*$`)
	bashOutput = regexp.MustCompile(`(?s)^\s*(<bash-stdout>(.*?)</bash-stdout>)?<bash-stderr>(.*?)</bash-stderr>(?:<bash-exit-code>(-?\d+)</bash-exit-code>)?\s*$`)
)

// claudeShellTool is the tool a shell command is drawn as, the agent's own
// Bash.
const claudeShellTool = "Bash"

func (d *claudeDriver) runsShell() bool { return true }

// claudeShellToModel bounds the output a shell command hands the model: Claude
// Code's Bash tool gives it 30000 characters inline by default
// (BASH_MAX_OUTPUT_LENGTH, 2.1.330), where bash_command's row runs past
// 260 KB.
const claudeShellToModel = 30000

// shell runs the command beside the conversation, which the agent never sees:
// the TUI's `!` adds it to the conversation, stream-json does that only on a
// cloud worker (2.1.330). Its output then reaches the agent as shellEnded
// tells.
func (d *claudeDriver) shell(command string) error {
	d.mu.Lock()
	d.interrupted, d.shellPending = false, true
	d.mu.Unlock()
	return d.c.out.send(map[string]any{"type": "bash_command", "command": command})
}

// claudeShellLine reads a shell command's line into the conversation and
// reports whether text was one: the command as the person's message, read
// back only from the transcript, as lich already wrote it live; its output
// as a call of the shell tool under it, which ends the command's turn live.
func claudeShellLine(c *chatSession, text string, history bool) bool {
	if m := bashInput.FindStringSubmatch(text); m != nil {
		if history {
			c.conv.userMessage(shellMark+html.UnescapeString(m[1]), nil)
		}
		return true
	}
	m := bashOutput.FindStringSubmatch(text)
	if m == nil {
		return false
	}
	turn, typed := c.conv.turnText()
	command := strings.TrimPrefix(typed, shellMark)
	input, _ := json.Marshal(map[string]string{"command": command})
	tool := claudeTool(claudeShellTool, input)
	// Only the blank lines go: a first line's indent lines up the columns
	// under it, as git status --short's " M" does.
	tool.Output = shorten(strings.Trim(html.UnescapeString(m[2])+"\n"+html.UnescapeString(m[3]), "\n"))
	// A command that did not run has no stdout; the TUI's carry no code.
	tool.State = api.ToolDone
	if code, _ := strconv.Atoi(m[4]); code != 0 || m[1] == "" {
		tool.State = api.ToolFailed
	}
	c.conv.put(api.ChatItem{ID: "shell-" + strconv.Itoa(turn), Kind: api.ItemTool, Tool: &tool}, true)
	if d, ok := c.drv.(*claudeDriver); ok && !history {
		d.shellEnded(command, m)
	}
	return true
}

// shellEnded follows the command lich ran, once, as the TUI's `!` does
// (processBashCommand, 2.1.330): a command that ran, failed included, goes
// to the agent as the person's message of the TUI's two lines, its input and
// its output, and the agent's answer ends the turn. A command the person
// stopped (Claude Code then prints [interrupted] with code 130) ends it
// cancelled, and one Claude Code refused, with no stdout, ends it there.
func (d *claudeDriver) shellEnded(command string, out []string) {
	d.mu.Lock()
	pending, interrupted := d.shellPending, d.interrupted
	d.shellPending, d.interrupted = false, false
	d.mu.Unlock()
	switch {
	case !pending:
	case interrupted:
		d.c.endTurn(stopCancelled)
	case out[1] == "":
		d.c.endTurn(stopEndTurn)
	default:
		lines := []map[string]any{
			{"type": "text", "text": "<bash-input>" + html.EscapeString(command) + "</bash-input>"},
			{"type": "text", "text": "<bash-stdout>" + shortenTo(out[2], claudeShellToModel) + "</bash-stdout><bash-stderr>" + shortenTo(out[3], claudeShellToModel) + "</bash-stderr>"},
		}
		if err := d.c.out.send(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": lines}}); err != nil {
			d.c.conv.notice(err.Error())
			d.c.endTurn(stopError)
		}
	}
}
