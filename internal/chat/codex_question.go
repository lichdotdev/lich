package chat

import (
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// What Codex asks of the person over app-server (0.170.0): a command, a patch
// or more permissions to run, which become permission items, and its
// request_user_input tool's questions and an MCP server's form, which become
// question items. Every request gets an answer: the turn waits on it.

// The decisions a command or patch approval takes.
const (
	codexAccept           = "accept"
	codexAcceptForSession = "acceptForSession"
	codexDecline          = "decline"
	codexCancel           = "cancel"
)

// codexPermits are a command's or patch's answers, as the window names them
// by kind.
var codexPermits = []api.ChatPermitOption{
	{ID: codexAccept, Kind: api.PermitAllowOnce},
	{ID: codexAcceptForSession, Kind: api.PermitAllowAlways},
	{ID: codexDecline, Kind: api.PermitRejectOnce},
}

// The grant scopes item/permissions/requestApproval takes.
const (
	codexScopeTurn    = "turn"
	codexScopeSession = "session"
)

func (d *codexDriver) request(m acpMessage) {
	if d.isFork(m.Params) {
		d.refuse(m)
		return
	}
	switch m.Method {
	case "item/commandExecution/requestApproval", "item/fileChange/requestApproval":
		d.approve(m)
	case "item/permissions/requestApproval":
		d.permissions(m)
	case "item/tool/requestUserInput":
		d.userInput(m)
	case "mcpServer/elicitation/request":
		d.elicit(m)
	default:
		d.refuse(m)
	}
}

type codexApproval struct {
	ItemID  string `json:"itemId"`
	Command string `json:"command"`
	Reason  string `json:"reason"`
}

// approve asks about a command or a patch. A patch's request carries only its
// item, which item/started drew with its diff just before.
func (d *codexDriver) approve(m acpMessage) {
	var p codexApproval
	_ = json.Unmarshal(m.Params, &p)
	tool := api.ChatToolCall{Name: codexShellTool, Title: p.Command}
	if it, ok := d.c.conv.get(p.ItemID); ok && it.Tool != nil {
		tool = *it.Tool
	}
	if tool.Title == "" {
		tool.Title = p.Reason
	}
	tool.State = api.ToolPending
	decide := func(decision string) error { return d.respond(m.ID, map[string]any{"decision": decision}) }
	itemID := "perm-" + strings.Trim(string(m.ID), `"`)
	d.track(m.ID, itemID)
	d.c.ask(itemID, tool, permission{
		options: codexPermits,
		reply:   func(o api.ChatPermitOption) error { return decide(o.ID) },
		dismiss: func() {
			if err := decide(codexCancel); err != nil {
				slog.Debug("chat: dismiss a Codex approval", "session", d.c.id, "err", err)
			}
		},
	})
}

// permissions asks to grant what the agent requests beyond its sandbox: the
// network or more of the file system, granted as asked for this turn or the
// session, or nothing.
func (d *codexDriver) permissions(m acpMessage) {
	var p struct {
		Reason      string          `json:"reason"`
		Permissions json.RawMessage `json:"permissions"`
	}
	_ = json.Unmarshal(m.Params, &p)
	grant := func(permissions json.RawMessage, scope string) error {
		return d.respond(m.ID, map[string]any{"permissions": permissions, "scope": scope})
	}
	none := json.RawMessage(`{}`)
	itemID := "perm-" + strings.Trim(string(m.ID), `"`)
	d.track(m.ID, itemID)
	d.c.ask(itemID, api.ChatToolCall{Name: "request_permissions", Title: p.Reason, State: api.ToolPending, Output: string(p.Permissions)}, permission{
		options: []api.ChatPermitOption{
			{ID: codexScopeTurn, Kind: api.PermitAllowOnce},
			{ID: codexScopeSession, Kind: api.PermitAllowAlways},
			{ID: codexDecline, Kind: api.PermitRejectOnce},
		},
		reply: func(o api.ChatPermitOption) error {
			if o.ID == codexDecline {
				return grant(none, codexScopeTurn)
			}
			return grant(p.Permissions, o.ID)
		},
		dismiss: func() {
			if err := grant(none, codexScopeTurn); err != nil {
				slog.Debug("chat: dismiss a Codex permission", "session", d.c.id, "err", err)
			}
		},
	})
}

type codexUserQuestion struct {
	ID       string `json:"id"`
	Header   string `json:"header"`
	Question string `json:"question"`
	Options  []struct {
		Label       string `json:"label"`
		Description string `json:"description"`
	} `json:"options"`
}

// readCodexQuestions reads request_user_input's questions; each also takes
// the person's own words, which the contract offers every ask.
func readCodexQuestions(qs []codexUserQuestion) api.ChatQuestion {
	q := api.ChatQuestion{Questions: []api.ChatAsk{}}
	for _, in := range qs {
		ask := api.ChatAsk{Header: in.Header, Text: in.Question}
		for _, o := range in.Options {
			ask.Options = append(ask.Options, api.ChatOffer{Label: o.Label, Description: o.Description})
		}
		q.Questions = append(q.Questions, ask)
	}
	return q
}

// codexAnswers is request_user_input's answer: each question's id to the
// labels picked and the person's own words. A decline answers none.
func codexAnswers(qs []codexUserQuestion, r api.ChatQReplied) map[string]any {
	answers := map[string]any{}
	if r.Declined {
		return answers
	}
	for i, q := range qs {
		picked := append([]string{}, r.Answers[i].Picked...)
		if other := strings.TrimSpace(r.Answers[i].Other); other != "" {
			picked = append(picked, other)
		}
		answers[q.ID] = map[string]any{"answers": picked}
	}
	return answers
}

func (d *codexDriver) userInput(m acpMessage) {
	var p struct {
		ItemID    string              `json:"itemId"`
		Questions []codexUserQuestion `json:"questions"`
	}
	_ = json.Unmarshal(m.Params, &p)
	itemID := "ask-" + strings.Trim(string(m.ID), `"`)
	d.track(m.ID, itemID)
	d.c.question(itemID, readCodexQuestions(p.Questions), func(r api.ChatQReplied) error {
		return d.respond(m.ID, map[string]any{"answers": codexAnswers(p.Questions, r)})
	})
}

// elicit turns an MCP server's form into a question item, the way an ACP
// elicitation is read; a URL or another form dialect is declined.
func (d *codexDriver) elicit(m acpMessage) {
	var req struct {
		Mode   string `json:"mode"`
		Schema struct {
			Properties map[string]acpProperty `json:"properties"`
		} `json:"requestedSchema"`
	}
	_ = json.Unmarshal(m.Params, &req)
	answer := func(action string, content any) error {
		return d.respond(m.ID, map[string]any{"action": action, "content": content, "_meta": nil})
	}
	q, asks := readElicitation(req.Schema.Properties)
	if req.Mode != "form" || len(asks) == 0 {
		if err := answer(codexDecline, nil); err != nil {
			slog.Debug("chat: decline a Codex elicitation", "session", d.c.id, "err", err)
		}
		return
	}
	itemID := "elicit-" + strings.Trim(string(m.ID), `"`)
	d.track(m.ID, itemID)
	d.c.question(itemID, q, func(r api.ChatQReplied) error {
		if r.Declined {
			return answer(codexDecline, nil)
		}
		return answer(codexAccept, elicitContent(asks, r))
	})
}
