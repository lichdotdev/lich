package chat

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
)

// claudeExitPlanTool is Claude Code's tool for asking to start on the plan it
// wrote in plan mode. It reaches lich as a can_use_tool whose input holds the
// plan and its file, as its tool_use does (measured on 2.1.330).
const claudeExitPlanTool = "ExitPlanMode"

// claudeProposalModes are the modes Claude Code's TUI offers to start the work
// in, in its order: "Yes, auto-accept edits" and "Yes, manually approve edits"
// (2.1.330, the approval that keeps the context). The ones it offers only
// with a setting or a mode lich does not track are left out (docs/ceilings.md).
var claudeProposalModes = []string{"acceptEdits", "default"}

// readClaudeProposal reads ExitPlanMode's input into a question item.
func readClaudeProposal(raw json.RawMessage) api.ChatQuestion {
	var in struct {
		Plan string `json:"plan"`
	}
	_ = json.Unmarshal(raw, &in)
	var ask api.ChatAsk
	for _, mode := range claudeProposalModes {
		ask.Options = append(ask.Options, api.ChatOffer{Label: mode})
	}
	return api.ChatQuestion{Questions: []api.ChatAsk{ask}, Proposal: &api.ChatProposal{Plan: in.Plan}}
}

// askProposal turns ExitPlanMode's can_use_tool into a question item. A mode
// picked allows it and switches the session to that mode, the way the TUI
// answers; anything else denies it with the person's words, if any, which
// keeps Claude Code in plan mode (measured on 2.1.330). A deny must carry a
// message: an empty one fails the next API call.
func (d *claudeDriver) askProposal(ask claudeAsk, respond func(map[string]any) error) {
	q := readClaudeProposal(ask.Input)
	text := prompt.For(d.lang)
	d.c.question(ask.ToolUseID, q, func(r api.ChatQReplied) error {
		if mode, ok := proposalMode(r); ok {
			// The TUI's input stays empty unless the person edited the plan,
			// and so approves it as written (measured); a plan in it reads as
			// edited and is written back to its file (read in its code).
			return respond(map[string]any{"behavior": "allow", "updatedInput": map[string]any{},
				"updatedPermissions": []map[string]any{{"type": "setMode", "mode": mode, "destination": "session"}}})
		}
		message := text.ChatPlanDeclined
		if words := proposalWords(r); words != "" {
			message = fmt.Sprintf(text.ChatPlanFeedback, words)
		}
		return respond(map[string]any{"behavior": "deny", "message": message})
	})
}

// proposalMode is the mode a reply starts the work in, false for none.
func proposalMode(r api.ChatQReplied) (string, bool) {
	if r.Declined || len(r.Answers) != 1 || len(r.Answers[0].Picked) != 1 {
		return "", false
	}
	mode := r.Answers[0].Picked[0]
	return mode, slices.Contains(claudeProposalModes, mode)
}

// proposalWords is what the person wrote to keep the agent planning.
func proposalWords(r api.ChatQReplied) string {
	if r.Declined || len(r.Answers) == 0 {
		return ""
	}
	return strings.TrimSpace(r.Answers[0].Other)
}

// readClaudeProposalResult records how a plan was answered, from its tool
// result: approved in a mode the transcript does not keep, or not.
func readClaudeProposalResult(c *chatSession, it api.ChatItem, b claudeBlock) {
	if b.IsError {
		c.replied(it.ID, api.ChatQReplied{Declined: true})
		return
	}
	c.replied(it.ID, api.ChatQReplied{Answers: []api.ChatAskAnswer{{}}})
}
