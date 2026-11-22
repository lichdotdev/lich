package terminal

import (
	"slices"
	"strings"

	"github.com/lichdotdev/lich/internal/providers"
)

// claudeTodoToolsVar gives a Claude Code session its task tools on every
// model: 2.1.330 offers them to the Claude 4 and older models alone (measured),
// and they are what the chat reads the agent's task list from
// (internal/chat/claude_plan.go).
const claudeTodoToolsVar = "CLAUDE_CODE_ENABLE_TODO_TOOLS"

// todoToolsEnv turns the task tools on for a Claude Code spawn, unless the
// person's own environment already says either way.
func todoToolsEnv(env []string, kind string) []string {
	if kind != providers.Claude || slices.ContainsFunc(env, func(kv string) bool { return strings.HasPrefix(kv, claudeTodoToolsVar+"=") }) {
		return env
	}
	return append(env, claudeTodoToolsVar+"=1")
}
