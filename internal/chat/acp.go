package chat

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/terminal"
)

// acpDriver speaks the Agent Client Protocol: JSON-RPC 2.0, one message per
// line, over the server's stdio. Measured against opencode 1.19.4, oh-my-pi
// 18.8.9, Antigravity 1.5.3 and Kiro 2.21.0; Cursor's `agent acp` answered
// initialize and nothing after it, behind a login.
type acpDriver struct {
	c *chatSession
	// server is opencode's own HTTP server, nil for every other agent
	// (opencode_question.go).
	server *terminal.OpenCodeServer

	mu    sync.Mutex
	next  int64
	calls map[string]chan acpReply
	// asked holds the opencode question tool calls already put to the person.
	asked   map[string]bool
	session string
	// run is the item the provider's streamed text is going into, and runKind
	// what that item is: a chunk of another kind, or a tool call, starts a new
	// one.
	run     string
	runKind string
	runs    int
	// replayed counts the items session/load reproduced.
	replayed int
	settings acpSettings
	// images is whether the agent takes an image in a prompt, as its
	// initialize says (promptCapabilities.image).
	images bool
}

type acpMessage struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *acpError       `json:"error,omitempty"`
}

type acpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *acpError) Error() string { return e.Message }

type acpReply struct {
	result json.RawMessage
	err    error
}

var errExited = errors.New("the agent's process exited")

func newACPDriver(c *chatSession, server *terminal.OpenCodeServer) *acpDriver {
	return &acpDriver{c: c, server: server, calls: map[string]chan acpReply{}, asked: map[string]bool{}}
}

// call sends a request and waits for its response.
func (d *acpDriver) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	d.mu.Lock()
	d.next++
	n := d.next
	id := strconv.FormatInt(n, 10)
	reply := make(chan acpReply, 1)
	d.calls[id] = reply
	d.mu.Unlock()
	if err := d.c.out.send(map[string]any{"jsonrpc": "2.0", "id": n, "method": method, "params": params}); err != nil {
		return nil, err
	}
	select {
	case r, ok := <-reply:
		if !ok {
			return nil, errExited
		}
		return r.result, r.err
	case <-ctx.Done():
		d.mu.Lock()
		delete(d.calls, id)
		d.mu.Unlock()
		return nil, ctx.Err()
	}
}

func (d *acpDriver) notify(method string, params any) error {
	return d.c.out.send(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

// open runs initialize, then session/load for a resume, session/new for the
// rest. A load that fails, or brings nothing back, is said in a notice: Kiro
// was measured opening a fresh conversation for an id it did not find, with no
// error.
func (d *acpDriver) open(ctx context.Context, resume string) (string, error) {
	raw, err := d.call(ctx, "initialize", map[string]any{
		"protocolVersion": 1,
		// lich serves neither fs nor terminal: a file the agent asked lich to
		// write would be written outside the session's sandbox. A form
		// elicitation is the agent's question to the person (acp_question.go).
		"clientCapabilities": map[string]any{
			"fs":       map[string]bool{"readTextFile": false, "writeTextFile": false},
			"terminal": false, "elicitation": map[string]any{"form": map[string]any{}},
		},
		"clientInfo": map[string]string{"name": "lich", "version": "1"},
	})
	if err != nil {
		return "", fmt.Errorf("the agent did not start its chat: %w", err)
	}
	var init struct {
		Caps struct {
			Load   bool `json:"loadSession"`
			Prompt struct {
				Image bool `json:"image"`
			} `json:"promptCapabilities"`
		} `json:"agentCapabilities"`
	}
	_ = json.Unmarshal(raw, &init)
	d.mu.Lock()
	d.images = init.Caps.Prompt.Image
	d.mu.Unlock()
	if resume != "" && init.Caps.Load {
		if d.load(ctx, resume) {
			return resume, nil
		}
	} else if resume != "" {
		d.c.conv.notice("The conversation did not come back: this agent cannot reopen one.")
	}
	return d.create(ctx)
}

// load reopens conversation id, its history replayed into the chat without
// reaching the window, which asks for it once.
func (d *acpDriver) load(ctx context.Context, id string) bool {
	d.c.conv.setQuiet(true)
	d.setSession(id)
	raw, err := d.call(ctx, "session/load", map[string]any{"sessionId": id, "cwd": d.c.cwd, "mcpServers": []any{}})
	d.endRun()
	d.c.conv.setQuiet(false)
	if err != nil {
		d.c.conv.notice("The conversation did not come back: " + err.Error())
		return false
	}
	d.readSession(raw)
	d.mu.Lock()
	replayed := d.replayed
	d.mu.Unlock()
	if replayed == 0 {
		d.c.conv.notice("The conversation came back empty: the agent may have opened a new one.")
	}
	return true
}

// create opens a new conversation, on the model and effort the session was
// born with when the argv could not carry them.
func (d *acpDriver) create(ctx context.Context) (string, error) {
	raw, err := d.call(ctx, "session/new", map[string]any{"cwd": d.c.cwd, "mcpServers": []any{}})
	if err != nil {
		return "", fmt.Errorf("the agent did not open a conversation: %w", err)
	}
	var created struct {
		ID string `json:"sessionId"`
	}
	if err := json.Unmarshal(raw, &created); err != nil || created.ID == "" {
		return "", fmt.Errorf("the agent opened a conversation with no id")
	}
	d.setSession(created.ID)
	d.readSession(raw)
	d.birth(ctx, api.OptionModel, d.c.proc.Birth.Model)
	d.birth(ctx, api.OptionEffort, d.c.proc.Birth.Effort)
	return created.ID, nil
}

// birth sets the option of category to the value the session was born with,
// which the argv could not carry.
func (d *acpDriver) birth(ctx context.Context, category, value string) {
	if value == "" {
		return
	}
	o, ok := d.c.optionOf(category)
	if !ok {
		d.c.conv.notice(fmt.Sprintf("The %s %s was not set: the agent offers no choice of it.", category, value))
		return
	}
	if err := d.setOption(ctx, o, value); err != nil {
		d.c.conv.notice(fmt.Sprintf("The %s %s was not set: %v", category, value, err))
	}
}

func (d *acpDriver) setSession(id string) {
	d.mu.Lock()
	d.session = id
	d.mu.Unlock()
}

func (d *acpDriver) sessionID() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.session
}

// prompt sends the message; session/prompt answers when the turn ends, with its
// stop reason, or with the error that ended it.
func (d *acpDriver) prompt(m message) error {
	d.endRun()
	blocks := d.content(m)
	go func() {
		raw, err := d.call(context.Background(), "session/prompt", map[string]any{
			"sessionId": d.sessionID(),
			"prompt":    blocks,
		})
		d.endRun()
		if errors.Is(err, errExited) {
			return // the chat's death ends the turn, saying why (died)
		}
		if err != nil {
			d.c.conv.notice(err.Error())
			d.c.endTurn(stopError)
			return
		}
		var done struct {
			Stop string `json:"stopReason"`
		}
		_ = json.Unmarshal(raw, &done)
		if done.Stop == "" {
			done.Stop = stopEndTurn
		}
		d.c.endTurn(done.Stop)
	}()
	return nil
}

// content is m as a prompt's content blocks: an image block for each image
// where the agent takes one, its path under the text where it does not. A
// message of images alone has no text block.
func (d *acpDriver) content(m message) []map[string]string {
	d.mu.Lock()
	images := d.images
	d.mu.Unlock()
	if !images {
		m = m.imagesAsText()
	}
	var blocks []map[string]string
	if m.text != "" || len(m.images) == 0 {
		blocks = append(blocks, map[string]string{"type": "text", "text": m.text})
	}
	for _, img := range m.images {
		blocks = append(blocks, map[string]string{"type": "image", "mimeType": img.mime, "data": base64.StdEncoding.EncodeToString(img.data)})
	}
	return blocks
}

// follow stops the turn and sends the message once it ended: ACP takes no
// message during a turn. The decline may already have ended it (oh-my-pi ends
// its turn on one), and then the message goes now.
func (d *acpDriver) follow(m message) error {
	if d.c.holdForNext(m) {
		return d.cancel()
	}
	go d.c.sendNext(pending{msg: m})
	return nil
}

func (d *acpDriver) cancel() error {
	return d.notify("session/cancel", map[string]string{"sessionId": d.sessionID()})
}

func (d *acpDriver) stopped() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for id, reply := range d.calls {
		close(reply)
		delete(d.calls, id)
	}
}

func (d *acpDriver) handle(raw []byte) {
	var m acpMessage
	if json.Unmarshal(raw, &m) != nil {
		return
	}
	switch {
	case m.Method != "" && len(m.ID) > 0:
		d.request(m)
	case m.Method == "session/update":
		d.update(m.Params)
	case m.Method == kiroCommands:
		d.kiroCommandsAvailable(m.Params)
	case m.Method == "" && len(m.ID) > 0:
		d.reply(m)
	}
}

// reply hands a response to its call. Ids are read as their JSON spelling, so
// 0 and "0" both find theirs.
func (d *acpDriver) reply(m acpMessage) {
	id := strings.Trim(string(m.ID), `"`)
	d.mu.Lock()
	reply, ok := d.calls[id]
	delete(d.calls, id)
	d.mu.Unlock()
	if !ok {
		return
	}
	if m.Error != nil {
		reply <- acpReply{err: m.Error}
		return
	}
	reply <- acpReply{result: m.Result}
}

// request answers what the agent asks of lich: a permission becomes an item,
// an elicitation a question item, and anything else is refused as a method
// lich does not serve.
func (d *acpDriver) request(m acpMessage) {
	if m.Method == acpElicit {
		d.elicit(m)
		return
	}
	if m.Method != "session/request_permission" {
		_ = d.c.out.send(map[string]any{"jsonrpc": "2.0", "id": m.ID, "error": map[string]any{"code": -32601, "message": "method not found"}})
		return
	}
	var ask struct {
		Tool    acpToolCall `json:"toolCall"`
		Options []struct {
			ID   string `json:"optionId"`
			Name string `json:"name"`
			Kind string `json:"kind"`
		} `json:"options"`
	}
	_ = json.Unmarshal(m.Params, &ask)
	options := make([]api.ChatPermitOption, 0, len(ask.Options))
	for _, o := range ask.Options {
		options = append(options, api.ChatPermitOption{ID: o.ID, Label: o.Name, Kind: o.Kind})
	}
	respond := func(outcome map[string]any) error {
		return d.c.out.send(map[string]any{"jsonrpc": "2.0", "id": m.ID, "result": map[string]any{"outcome": outcome}})
	}
	tool := ask.Tool.call(api.ChatToolCall{State: api.ToolPending})
	d.c.ask("perm-"+strings.Trim(string(m.ID), `"`), tool, permission{
		options: options,
		reply: func(o api.ChatPermitOption) error {
			return respond(map[string]any{"outcome": "selected", "optionId": o.ID})
		},
		dismiss: func() {
			if err := respond(map[string]any{"outcome": "cancelled"}); err != nil {
				slog.Debug("chat: dismiss permission", "session", d.c.id, "err", err)
			}
		},
	})
}

type acpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// acpUpdate is one session/update. Its content is a block for a chunk and a
// list of blocks for a tool call (acpToolCall.Body).
type acpUpdate struct {
	Kind string `json:"sessionUpdate"`
	Mode string `json:"currentModeId"`
	acpToolCall
}

func (d *acpDriver) update(params json.RawMessage) {
	var p struct {
		Update json.RawMessage `json:"update"`
	}
	_ = json.Unmarshal(params, &p)
	var u acpUpdate
	if json.Unmarshal(p.Update, &u) != nil {
		return
	}
	var content acpContent
	_ = json.Unmarshal(u.Body, &content)
	switch u.Kind {
	case "agent_message_chunk":
		d.chunk(api.ItemAgent, content)
	case "agent_thought_chunk":
		d.chunk(api.ItemThought, content)
	case "user_message_chunk":
		// Outside a load the person's message is already an item: lich wrote
		// it, and Antigravity echoes it.
		if d.c.conv.replaying() {
			d.chunk(api.ItemUser, content)
		}
	case "tool_call", "tool_call_update":
		d.tool(u.acpToolCall)
		d.openCodeQuestion(u.acpToolCall)
	case "plan":
		d.c.conv.plan(acpPlan(p.Update))
	case "config_option_update":
		d.configChanged(p.Update)
	case "current_mode_update":
		d.modeChanged(u.Mode)
	case "available_commands_update":
		d.commandsAvailable(p.Update)
	}
}

// chunk appends streamed text to the item it belongs to.
func (d *acpDriver) chunk(kind string, content acpContent) {
	if content.Type != "text" {
		return
	}
	d.mu.Lock()
	if d.runKind != kind {
		d.runs++
		d.run, d.runKind = "", kind
	}
	id, first := d.run, d.run == ""
	if first && kind != api.ItemUser {
		id = kind + "-" + strconv.Itoa(d.runs)
		d.run = id
	}
	d.replayed++
	d.mu.Unlock()
	if first && kind == api.ItemUser {
		turn := d.c.conv.userMessage(content.Text, nil)
		d.mu.Lock()
		d.run = "user-" + strconv.Itoa(turn)
		d.mu.Unlock()
		return
	}
	model, effort := d.producer()
	d.c.conv.update(id, kind, false, func(it *api.ChatItem) {
		it.Text += content.Text
		attribute(it, model, effort)
	})
}

// endRun closes the streamed item, so the next chunk starts a new one.
func (d *acpDriver) endRun() {
	d.mu.Lock()
	d.run, d.runKind = "", ""
	d.mu.Unlock()
}

func (d *acpDriver) tool(t acpToolCall) {
	d.endRun()
	d.mu.Lock()
	d.replayed++
	d.mu.Unlock()
	id := "tool-" + t.ID
	_, known := d.c.conv.get(id)
	model, effort := d.producer()
	d.c.conv.update(id, api.ItemTool, t.Status == "completed" || t.Status == "failed", func(it *api.ChatItem) {
		var prev api.ChatToolCall
		if it.Tool != nil {
			prev = *it.Tool
		}
		next := t.call(prev)
		it.Tool = &next
		attribute(it, model, effort)
	})
	if !known {
		if it, ok := d.c.conv.get(id); ok {
			d.c.toolStarted(it.Tool.Name, it.Tool.Title)
		}
	}
}

// acpPlan reads a plan update, the whole list each time; an empty one clears
// it (oh-my-pi sends one when its todos are cleared).
func acpPlan(raw json.RawMessage) []api.ChatPlanEntry {
	var u struct {
		Entries []api.ChatPlanEntry `json:"entries"`
	}
	_ = json.Unmarshal(raw, &u)
	return u.Entries
}
