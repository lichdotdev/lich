package chat

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/prompt"
)

// claudeDriver speaks Claude Code's stream-json with its control protocol, as
// measured on 2.1.327: lich writes `user` messages and `control_request`s
// (initialize, interrupt), Claude Code writes every message of the turn, its
// partial text as `stream_event`s, `control_request` can_use_tool when a tool
// needs the person, and one `result` when the turn ends.
type claudeDriver struct {
	c    *chatSession
	lang prompt.Lang

	mu       sync.Mutex
	next     int
	calls    map[string]chan claudeControl
	expected string
	// blocks counts the assistant lines each message id has written: Claude
	// Code writes one line per content block, in order, so the nth line is
	// block n, the index its stream events used.
	blocks map[string]int
	// held is the start of an answer's text that could still be claudeNoResponse,
	// kept from the window until it is not.
	held        map[string]string
	interrupted bool
	run         claudeRunning
	// cmds are the commands initialize listed, skills those system/init
	// names skills and terminalOnly those it says only the TUI runs.
	cmds                 []claudeCommand
	skills, terminalOnly []string
	// details is what `claude plugin details` said of each installed plugin,
	// by its install path, which a new version changes.
	details map[string]claudeDetails
	tasks   *claudeTasks
}

type claudeControl struct {
	Subtype string          `json:"subtype"`
	ID      string          `json:"request_id"`
	Error   string          `json:"error"`
	Code    string          `json:"error_code"`
	Body    json.RawMessage `json:"response"`
}

func newClaudeDriver(c *chatSession, lang prompt.Lang) *claudeDriver {
	return &claudeDriver{c: c, lang: lang, calls: map[string]chan claudeControl{}, blocks: map[string]int{}, held: map[string]string{}, tasks: newClaudeTasks()}
}

// claudeLine is every field lich reads off one line Claude Code writes, of any
// type.
type claudeLine struct {
	Type      string          `json:"type"`
	Subtype   string          `json:"subtype"`
	SessionID string          `json:"session_id"`
	Message   claudeMessage   `json:"message"`
	Parent    *string         `json:"parent_tool_use_id"`
	Event     claudeEvent     `json:"event"`
	MessageID string          `json:"api_message_id"`
	RequestID string          `json:"request_id"`
	Request   json.RawMessage `json:"request"`
	Response  claudeControl   `json:"response"`
	// system/init and system/status
	Model          string `json:"model"`
	PermissionMode string `json:"permissionMode"`
	// system/init
	Skills           []string `json:"skills"`
	TerminalCommands []string `json:"terminal_slash_commands"`
	// result
	IsError  bool     `json:"is_error"`
	Result   string   `json:"result"`
	Errors   []string `json:"errors"`
	Terminal string   `json:"terminal_reason"`
	// the tool's own reading of its result, live and in the transcript
	ToolResult  json.RawMessage `json:"tool_use_result"`
	SavedResult json.RawMessage `json:"toolUseResult"`
	// transcript
	Sidechain bool   `json:"isSidechain"`
	Meta      bool   `json:"isMeta"`
	Timestamp string `json:"timestamp"`
}

type claudeMessage struct {
	ID      string          `json:"id"`
	Model   string          `json:"model"`
	Content json.RawMessage `json:"content"`
}

type claudeBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

type claudeEvent struct {
	Type  string      `json:"type"`
	Index int         `json:"index"`
	Block claudeBlock `json:"content_block"`
	Delta struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		Thinking string `json:"thinking"`
	} `json:"delta"`
}

func (d *claudeDriver) open(ctx context.Context, resume string) (string, error) {
	d.mu.Lock()
	d.expected = resume
	d.mu.Unlock()
	init, err := d.control(ctx, map[string]any{"subtype": "initialize"})
	if err != nil {
		return "", fmt.Errorf("Claude Code did not start its chat: %w", err)
	}
	d.readInitialize(init.Body, d.c.svc.store.SessionModel(d.c.id))
	// The transcript was read before the models were known: its items carry
	// the ids Claude Code wrote.
	d.c.conv.renameModels(d.modelName)
	if err := d.readApplied(ctx); err != nil {
		d.c.conv.notice("Claude Code did not say which model it runs: " + err.Error())
	}
	d.publish()
	// Claude Code names the conversation in system/init, which it writes only
	// once the first message arrives; a resume keeps the id it was given.
	return resume, nil
}

// control sends a control request and waits for its response.
func (d *claudeDriver) control(ctx context.Context, request map[string]any) (claudeControl, error) {
	d.mu.Lock()
	d.next++
	id := "lich-" + strconv.Itoa(d.next)
	reply := make(chan claudeControl, 1)
	d.calls[id] = reply
	d.mu.Unlock()
	if err := d.c.out.send(map[string]any{"type": "control_request", "request_id": id, "request": request}); err != nil {
		return claudeControl{}, err
	}
	select {
	case r, ok := <-reply:
		if !ok {
			return claudeControl{}, errExited
		}
		if r.Subtype == "error" {
			return r, fmt.Errorf("%s", r.Error)
		}
		return r, nil
	case <-ctx.Done():
		d.mu.Lock()
		delete(d.calls, id)
		d.mu.Unlock()
		return claudeControl{}, ctx.Err()
	}
}

func (d *claudeDriver) prompt(m message) error {
	d.mu.Lock()
	d.interrupted = false
	d.mu.Unlock()
	return d.c.out.send(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": claudeContent(m)}})
}

// claudeContent is m as a user message's content: its text alone, or the
// API's blocks with an image block for each image. A message of images alone
// has no text block, which the API refuses empty (measured on 2.1.330: an
// image with and without text was seen). The text goes after the images:
// Claude Code reads an @path only from a text block that ends the message.
func claudeContent(m message) any {
	if len(m.images) == 0 {
		return m.text
	}
	var blocks []map[string]any
	for _, img := range m.images {
		blocks = append(blocks, map[string]any{"type": "image", "source": map[string]any{
			"type": "base64", "media_type": img.mime, "data": base64.StdEncoding.EncodeToString(img.data),
		}})
	}
	if m.text != "" {
		blocks = append(blocks, map[string]any{"type": "text", "text": m.text})
	}
	return blocks
}

// follow writes the message during the turn: Claude Code takes it into the
// turn running, after the denied question (measured on 2.1.328).
func (d *claudeDriver) follow(m message) error { return d.prompt(m) }

func (d *claudeDriver) cancel() error {
	d.mu.Lock()
	d.interrupted = true
	d.next++
	id := "lich-" + strconv.Itoa(d.next)
	d.mu.Unlock()
	// Sent without waiting for its response: the spike saw an interrupt
	// answered once and left unanswered once (2.1.327), and the turn's result
	// is what says it stopped.
	return d.c.out.send(map[string]any{"type": "control_request", "request_id": id, "request": map[string]any{"subtype": "interrupt"}})
}

func (d *claudeDriver) stopped() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for id, reply := range d.calls {
		close(reply)
		delete(d.calls, id)
	}
}

func (d *claudeDriver) handle(raw []byte) {
	var line claudeLine
	if err := json.Unmarshal(raw, &line); err != nil {
		return
	}
	switch line.Type {
	case "control_response":
		d.mu.Lock()
		reply, ok := d.calls[line.Response.ID]
		delete(d.calls, line.Response.ID)
		d.mu.Unlock()
		if ok {
			reply <- line.Response
		}
	case "control_request":
		d.request(line.RequestID, line.Request)
	case "system":
		if line.Subtype == "init" {
			d.mu.Lock()
			expected := d.expected
			d.expected = ""
			d.mu.Unlock()
			d.c.bind(line.SessionID, expected)
			d.markCommands(line.Skills, line.TerminalCommands)
		}
		d.running(line.Model, line.PermissionMode)
	case "result":
		d.result(line)
	default:
		readClaudeMessage(d.c, line, false)
	}
}

// readClaudeMessage turns one message line, live or read back from the
// transcript, into items. A subagent's own messages stay inside its tool call.
func readClaudeMessage(c *chatSession, line claudeLine, history bool) {
	if (line.Parent != nil && *line.Parent != "") || line.Sidechain || line.Meta {
		return
	}
	d, _ := c.drv.(*claudeDriver)
	switch line.Type {
	case "stream_event":
		if d != nil {
			d.stream(line)
		}
	case "assistant":
		readClaudeAssistant(c, d, line, history)
	case "user":
		readClaudeUser(c, line, history)
	}
}

// stream reads a partial message: text and thinking as they arrive.
func (d *claudeDriver) stream(line claudeLine) {
	id := fmt.Sprintf("%s/%d", line.MessageID, line.Event.Index)
	delta := line.Event.Delta
	// A redacted thinking block streams empty deltas (measured on 2.1.327),
	// which would draw an empty thought.
	switch {
	case line.Event.Type != "content_block_delta":
	case delta.Type == "text_delta" && delta.Text != "":
		text, held := d.hold(id, delta.Text)
		if held {
			return
		}
		model, effort := d.producer("", false)
		d.c.conv.update(id, api.ItemAgent, false, func(it *api.ChatItem) { it.Text += text; attribute(it, model, effort) })
	case delta.Type == "thinking_delta" && delta.Thinking != "":
		model, effort := d.producer("", false)
		d.c.conv.update(id, api.ItemThought, false, func(it *api.ChatItem) {
			it.Text += delta.Thinking
			attribute(it, model, effort)
		})
	}
}

func readClaudeAssistant(c *chatSession, d *claudeDriver, line claudeLine, history bool) {
	var blocks []claudeBlock
	if json.Unmarshal(line.Message.Content, &blocks) != nil {
		return
	}
	model, effort := d.producer(line.Message.Model, history)
	for _, b := range blocks {
		index := 0
		if d != nil {
			d.mu.Lock()
			index = d.blocks[line.Message.ID]
			d.blocks[line.Message.ID]++
			d.mu.Unlock()
		}
		id := fmt.Sprintf("%s/%d", line.Message.ID, index)
		switch b.Type {
		case "text":
			if b.Text == claudeNoResponse {
				d.drop(id)
				continue
			}
			c.conv.put(api.ChatItem{ID: id, Kind: api.ItemAgent, Text: b.Text, At: stamp(line.Timestamp), Model: model, Effort: effort}, true)
		case "thinking":
			if b.Thinking != "" {
				c.conv.put(api.ChatItem{ID: id, Kind: api.ItemThought, Text: b.Thinking, At: stamp(line.Timestamp), Model: model, Effort: effort}, true)
			}
		case "tool_use":
			if q, ok := claudeAsking(b); ok {
				c.conv.update(b.ID, api.ItemQuestion, true, func(it *api.ChatItem) {
					if it.Question == nil {
						it.Question, it.At, it.Model, it.Effort = &q, stamp(line.Timestamp), model, effort
					}
				})
				continue
			}
			tool := claudeTool(b.Name, b.Input)
			tool.State = api.ToolRunning
			c.conv.put(api.ChatItem{ID: b.ID, Kind: api.ItemTool, Tool: &tool, At: stamp(line.Timestamp), Model: model, Effort: effort}, true)
			c.toolStarted(tool.Name, tool.Title)
			if d != nil && d.tasks.use(b) {
				c.conv.plan(d.tasks.list())
			}
		}
	}
}

// The user lines Claude Code writes when a turn is interrupted, the second
// when it was a tool call that was (measured on 2.1.327): markers, not
// something the person said.
const (
	claudeInterrupted        = "[Request interrupted by user]"
	claudeInterruptedForTool = "[Request interrupted by user for tool use]"
)

// isInterruptMarker is whether text is one of Claude Code's interrupt markers,
// by equality: a message that merely mentions one is the person's.
func isInterruptMarker(text string) bool {
	return text == claudeInterrupted || text == claudeInterruptedForTool
}

func readClaudeUser(c *chatSession, line claudeLine, history bool) {
	var text string
	if json.Unmarshal(line.Message.Content, &text) == nil {
		if localCommand(c, text) {
			return
		}
		// Live, the person's message is already an item: lich wrote it.
		if history && !isInterruptMarker(text) {
			c.conv.userMessage(text, nil)
		}
		return
	}
	var blocks []claudeBlock
	if json.Unmarshal(line.Message.Content, &blocks) != nil {
		return
	}
	result := line.ToolResult
	if len(result) == 0 {
		result = line.SavedResult
	}
	for _, b := range blocks {
		it, _ := c.conv.get(b.ToolUseID)
		switch {
		case b.Type == "tool_result" && it.Kind == api.ItemQuestion && it.Question != nil:
			readClaudeQuestionResult(c, it, b, result)
		case b.Type == "tool_result":
			if d, ok := c.drv.(*claudeDriver); ok && d.tasks.result(b) {
				c.conv.plan(d.tasks.list())
			}
			c.conv.update(b.ToolUseID, api.ItemTool, true, func(it *api.ChatItem) {
				if it.Tool == nil {
					it.Tool = &api.ChatToolCall{}
				}
				it.Tool.State = api.ToolDone
				if b.IsError {
					it.Tool.State = api.ToolFailed
				}
				it.Tool.Output = shorten(blockText(b.Content))
				it.Tool.Ended = stamp(line.Timestamp)
			})
		case b.Type == "text" && localCommand(c, b.Text):
		case b.Type == "text" && history && !isInterruptMarker(b.Text):
			c.conv.userMessage(b.Text, nil)
		}
	}
}

// blockText reads a tool result's content, a string or a list of blocks.
func blockText(raw json.RawMessage) string {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var blocks []claudeBlock
	_ = json.Unmarshal(raw, &blocks)
	parts := []string{}
	for _, b := range blocks {
		if b.Type == "text" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// result ends the turn. An interrupt ends it as an error during execution
// (measured), which is the person's cancel, not a failure.
func (d *claudeDriver) result(line claudeLine) {
	d.mu.Lock()
	interrupted := d.interrupted
	d.interrupted = false
	d.mu.Unlock()
	switch {
	case !line.IsError:
		d.c.endTurn(stopEndTurn)
	case interrupted || line.Terminal == "aborted_streaming":
		d.c.endTurn(stopCancelled)
	default:
		if text := strings.TrimSpace(strings.Join(append([]string{line.Result}, line.Errors...), "\n")); text != "" {
			d.c.conv.notice(text)
		}
		d.c.endTurn(stopError)
	}
}

type claudeAsk struct {
	Subtype     string            `json:"subtype"`
	Tool        string            `json:"tool_name"`
	Input       json.RawMessage   `json:"input"`
	Suggestions []json.RawMessage `json:"permission_suggestions"`
	ToolUseID   string            `json:"tool_use_id"`
}

// request answers what Claude Code asks of lich: can_use_tool becomes a
// permission item, or a question item for AskUserQuestion and ExitPlanMode,
// and anything else is refused, which is what the protocol expects of a
// client that does not serve it.
func (d *claudeDriver) request(id string, raw json.RawMessage) {
	var ask claudeAsk
	_ = json.Unmarshal(raw, &ask)
	if ask.Subtype != "can_use_tool" {
		_ = d.c.out.send(map[string]any{"type": "control_response", "response": map[string]any{
			"subtype": "error", "request_id": id, "error": "lich does not serve " + ask.Subtype,
		}})
		return
	}
	respond := func(behavior map[string]any) error {
		return d.c.out.send(map[string]any{"type": "control_response", "response": map[string]any{
			"subtype": "success", "request_id": id, "response": behavior,
		}})
	}
	switch ask.Tool {
	case claudeQuestionTool:
		d.askQuestion(ask, respond)
		return
	case claudeExitPlanTool:
		d.askProposal(ask, respond)
		return
	}
	options := []api.ChatPermitOption{{ID: api.PermitAllowOnce, Kind: api.PermitAllowOnce}}
	if len(ask.Suggestions) > 0 {
		options = append(options, api.ChatPermitOption{ID: api.PermitAllowAlways, Kind: api.PermitAllowAlways})
	}
	options = append(options, api.ChatPermitOption{ID: api.PermitRejectOnce, Kind: api.PermitRejectOnce})
	deny := func() error {
		return respond(map[string]any{"behavior": "deny", "message": prompt.For(d.lang).ChatPermissionDenied})
	}
	tool := claudeTool(ask.Tool, ask.Input)
	tool.State = api.ToolPending
	d.c.ask("perm-"+id, tool, permission{
		options: options,
		reply: func(o api.ChatPermitOption) error {
			switch o.Kind {
			case api.PermitAllowOnce:
				return respond(map[string]any{"behavior": "allow", "updatedInput": ask.Input})
			case api.PermitAllowAlways:
				return respond(map[string]any{"behavior": "allow", "updatedInput": ask.Input, "updatedPermissions": ask.Suggestions})
			}
			return deny()
		},
		dismiss: func() { _ = deny() },
	})
}

// claudeTool reads what a Claude Code tool call acts on out of its input.
func claudeTool(name string, raw json.RawMessage) api.ChatToolCall {
	var in map[string]any
	_ = json.Unmarshal(raw, &in)
	tool := api.ChatToolCall{Name: name}
	for _, key := range []string{"file_path", "command", "pattern", "path", "url", "query", "description"} {
		if v, ok := in[key].(string); ok && v != "" {
			tool.Title = v
			break
		}
	}
	path, _ := in["file_path"].(string)
	switch name {
	case "Edit":
		old, _ := in["old_string"].(string)
		next, _ := in["new_string"].(string)
		tool.Diff = unifiedDiff(path, old, next)
	case "Write":
		content, _ := in["content"].(string)
		tool.Diff = unifiedDiff(path, "", content)
	}
	return tool
}

// hold keeps the first words of an answer from the window while they could be
// the whole of claudeNoResponse, which a stream gives a few characters at a
// time. It returns what to add to the item, and true while to wait.
func (d *claudeDriver) hold(id, delta string) (string, bool) {
	if _, begun := d.c.conv.get(id); begun {
		return delta, false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	text := d.held[id] + delta
	if strings.HasPrefix(claudeNoResponse, text) {
		d.held[id] = text
		return "", true
	}
	delete(d.held, id)
	return text, false
}

// drop forgets what a held answer collected: the message turned out to be
// claudeNoResponse.
func (d *claudeDriver) drop(id string) {
	if d == nil {
		return
	}
	d.mu.Lock()
	delete(d.held, id)
	d.mu.Unlock()
}
