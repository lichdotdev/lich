//go:build !windows

package chat

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

// fakeKiroCommandsEnv has the fake ACP agent list its commands the way Kiro
// CLI does instead of ACP's update.
const fakeKiroCommandsEnv = "LICH_CHAT_FAKE_KIRO_COMMANDS"

// fakeClaudeCommands is initialize's list in its measured shape (2.1.329):
// a skill, a command with arguments, and one only the TUI runs.
var fakeClaudeCommands = []any{
	map[string]any{"name": "review", "description": "Review a pull request", "argumentHint": "[pr]"},
	map[string]any{"name": "deploy", "description": "Ship it", "argumentHint": ""},
	map[string]any{"name": "doctor", "description": "Check the install", "argumentHint": "", "builtin": true},
}

// acpCommands lists the fake agent's commands once its session is open.
func (f *fake) acpCommands() {
	if os.Getenv(fakeKiroCommandsEnv) != "" {
		f.send(map[string]any{"jsonrpc": "2.0", "method": kiroCommands, "params": map[string]any{"sessionId": "s", "commands": []any{
			map[string]any{"name": "/usage", "description": "Show billing and credits", "meta": map[string]any{"inputType": "panel", "hint": ""}},
			map[string]any{"name": "/quit", "description": "Quit the application", "meta": map[string]any{"local": true}},
		}}})
		return
	}
	f.update(map[string]any{"sessionUpdate": "available_commands_update", "availableCommands": []any{
		map[string]any{"name": "compact", "description": "Compact the conversation"},
		map[string]any{"name": "switch", "description": "Switch model for this session only", "input": map[string]any{"hint": "[model]"}},
	}})
}

func (r *rig) commands() []api.ChatCommand {
	r.t.Helper()
	cmds, err := r.svc.Commands(r.t.Context(), api.SessionIDOptions{ID: "s1"})
	if err != nil {
		r.t.Fatalf("Commands: %v", err)
	}
	return cmds
}

// waitCommands waits for the chat's commands to read want.
func (r *rig) waitCommands(want []api.ChatCommand) {
	r.t.Helper()
	r.eventually("the commands "+strings.Join(commandTexts(want), " "), func() bool { return slices.Equal(r.commands(), want) })
}

func commandTexts(cmds []api.ChatCommand) []string {
	out := make([]string, len(cmds))
	for i, c := range cmds {
		out[i] = c.Text
	}
	return out
}

func newChat(t *testing.T, kind string) *rig {
	t.Helper()
	r := newRig(t, kind)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	return r
}

// Claude Code lists its commands at initialize and, once the first message
// is in, says in system/init which are skills and which only its TUI runs.
func TestClaudeCommandsComeFromInitializeAndInit(t *testing.T) {
	r := newChat(t, providers.Claude)
	r.waitCommands([]api.ChatCommand{
		{Text: "/review", Description: "Review a pull request", Hint: "[pr]"},
		{Text: "/deploy", Description: "Ship it"},
		{Text: "/doctor", Description: "Check the install"},
	})
	r.prompt("hi")
	r.waitCommands([]api.ChatCommand{
		{Text: "/review", Description: "Review a pull request", Hint: "[pr]"},
		{Text: "/deploy", Description: "Ship it", Skill: true},
	})
}

// opencode, oh-my-pi and Antigravity list theirs in ACP's
// available_commands_update, the name without its slash.
func TestACPCommandsComeFromTheUpdate(t *testing.T) {
	r := newChat(t, providers.OpenCode)
	r.waitCommands([]api.ChatCommand{
		{Text: "/compact", Description: "Compact the conversation"},
		{Text: "/switch", Description: "Switch model for this session only", Hint: "[model]"},
	})
}

// Kiro CLI lists its own in a notification of its own, slashes included; a
// command it marks local runs only in its TUI.
func TestKiroCommandsComeFromItsNotification(t *testing.T) {
	t.Setenv(fakeKiroCommandsEnv, "1")
	r := newChat(t, providers.Kiro)
	r.waitCommands([]api.ChatCommand{{Text: "/usage", Description: "Show billing and credits"}})
}

// Codex lists skills alone, typed as $name; a message naming one hands
// turn/start the skill too, as app-server's schema takes it.
func TestCodexSkillsAreItsCommands(t *testing.T) {
	r := newChat(t, providers.Codex)
	r.waitCommands([]api.ChatCommand{{Text: "$deploy", Description: "Ship it", Skill: true}})
	r.prompt("hi $deploy and $off")
	r.eventually("a turn/start", func() bool { return len(r.turnStarts()) > 0 })
	start := r.turnStarts()[0]
	if !strings.Contains(start, `"type":"skill"`) || !strings.Contains(start, `"/skills/deploy/SKILL.md"`) || strings.Contains(start, "/skills/off/") {
		t.Errorf("turn/start %s, want the deploy skill alone beside the text", start)
	}
}
