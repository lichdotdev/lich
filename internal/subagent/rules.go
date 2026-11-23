package subagent

import (
	"fmt"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/relay"
)

// claudeCardType is the one Claude Code subagent type that becomes a card;
// every other type is a read-only exploration that runs inside the session,
// as the mod's agent-cards had it. A call naming none runs as this one.
const claudeCardType = "general-purpose"

// nativeTypes is the exploration type each provider keeps native
// (docs/hooks/subagent-cards.md, Types kept native). Kiro CLI and Crush have
// none.
var nativeTypes = map[string]string{
	providers.Codex:       "explorer",
	providers.OpenCode:    "explore",
	providers.OMP:         "scout",
	providers.Antigravity: "research",
	providers.Cursor:      "explore",
}

// keepsNative is whether a call runs natively because one of its tasks is a
// type kind keeps native. A veto is the whole call's, so one such task keeps
// the whole call native, as a prompt over the limit does.
func keepsNative(kind string, tasks []api.SubagentTask) bool {
	for _, t := range tasks {
		if kind == providers.Claude {
			if t.Type != "" && t.Type != claudeCardType {
				return true
			}
			continue
		}
		if native, ok := nativeTypes[kind]; ok && strings.EqualFold(t.Type, native) {
			return true
		}
	}
	return false
}

// overLimit is whether a task is longer than send_to_session takes: the
// native subagent runs the whole call rather than a card half of it.
func overLimit(tasks []api.SubagentTask) bool {
	for _, t := range tasks {
		if len(t.Prompt) > relay.PromptLimit {
			return true
		}
	}
	return false
}

// openedReason is what the model reads about the sessions a deny opened.
func (s *Service) openedReason(opened []api.SubagentOpened) string {
	text := prompt.For(s.lang())
	if len(opened) == 1 {
		return fmt.Sprintf(text.SubagentOpened, opened[0].Label, opened[0].Ticket)
	}
	names := make([]string, len(opened))
	for i, o := range opened {
		names[i] = fmt.Sprintf(text.SubagentOpenedSession, o.Label, o.Ticket)
	}
	return fmt.Sprintf(text.SubagentOpenedMany, strings.Join(names, ", "))
}
