package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"strconv"
	"strings"
	"sync"

	"github.com/lichdotdev/lich/api"
)

// codexDriver speaks Codex's app-server: JSON-RPC 2.0 without the "jsonrpc"
// member on Codex's side, one message per line over stdio, as measured on
// 0.170.0 against the schema `codex app-server generate-ts` writes. A thread is
// the conversation, the same rollout `codex resume <id>` opens; turn/start
// answers at once with the turn's id, and turn/completed ends it. Nothing
// experimental is asked for: the stable API carries every method used here.
type codexDriver struct {
	c *chatSession

	mu     sync.Mutex
	next   int64
	calls  map[string]chan acpReply
	thread string
	turn   string
	run    codexRunning
	// asks is the item each server request still waiting on the person
	// shows, by the request's id: serverRequest/resolved names only the id.
	asks map[string]string
	// skills are the skills skills/list gave, their paths by name.
	skills map[string]string
	// sources are the instruction files the thread loaded.
	sources []string
}

func newCodexDriver(c *chatSession) *codexDriver {
	return &codexDriver{c: c, calls: map[string]chan acpReply{}, asks: map[string]string{}}
}

// call sends a request and waits for its response.
func (d *codexDriver) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
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

// respond answers the server request id.
func (d *codexDriver) respond(id json.RawMessage, result any) error {
	return d.c.out.send(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

// codexThread is the part of thread/start's and thread/resume's answer lich
// reads: the thread, and what it runs on.
type codexThread struct {
	Thread struct {
		ID   string `json:"id"`
		Path string `json:"path"`
	} `json:"thread"`
	Model    string        `json:"model"`
	Effort   string        `json:"reasoningEffort"`
	Approval any           `json:"approvalPolicy"`
	Sandbox  codexSandboxP `json:"sandbox"`
	Sources  []string      `json:"instructionSources"`
}

// open runs initialize, reads the models, then resumes thread resume or
// starts one. A resume that fails is said in a notice and a new thread opens.
func (d *codexDriver) open(ctx context.Context, resume string) (string, error) {
	if _, err := d.call(ctx, "initialize", map[string]any{
		"clientInfo":   map[string]any{"name": "lich", "title": nil, "version": "1"},
		"capabilities": map[string]any{"experimentalApi": false, "requestAttestation": false},
	}); err != nil {
		return "", fmt.Errorf("Codex did not start its chat: %w", err)
	}
	if err := d.c.out.send(map[string]any{"jsonrpc": "2.0", "method": "initialized"}); err != nil {
		return "", err
	}
	if err := d.readModels(ctx); err != nil {
		d.c.conv.notice("Codex did not list its models: " + err.Error())
	}
	d.listSkills(ctx)
	if resume == "" {
		return d.start(ctx)
	}
	id, err := d.resume(ctx, resume)
	if err != nil {
		d.c.conv.notice("The conversation did not come back: " + err.Error())
		return d.start(ctx)
	}
	return id, nil
}

// birthMode is what a thread opens on when the checkout skips permissions:
// the TUI's --dangerously-bypass-approvals-and-sandbox, which app-server does
// not take on its argv.
func (d *codexDriver) birthMode(params map[string]any) map[string]any {
	if d.c.proc.Birth.AutoApprove {
		params["approvalPolicy"], params["sandbox"] = codexApprovalNever, "danger-full-access"
	}
	return params
}

func (d *codexDriver) start(ctx context.Context) (string, error) {
	params := d.birthMode(map[string]any{"cwd": d.c.cwd})
	if model := d.c.proc.Birth.Model; model != "" {
		params["model"] = model
	}
	raw, err := d.call(ctx, "thread/start", params)
	if err != nil {
		return "", fmt.Errorf("Codex did not open a conversation: %w", err)
	}
	t, err := d.readThread(raw)
	if err != nil {
		return "", err
	}
	if effort := d.c.proc.Birth.Effort; effort != "" {
		d.stage(api.OptionEffort, effort)
	}
	d.publish()
	return t.Thread.ID, nil
}

// resume reopens thread id and reads its turns back into the chat, without
// reaching the window, which asks for the history once.
func (d *codexDriver) resume(ctx context.Context, id string) (string, error) {
	raw, err := d.call(ctx, "thread/resume", d.birthMode(map[string]any{"threadId": id, "cwd": d.c.cwd, "excludeTurns": true}))
	if err != nil {
		return "", err
	}
	t, err := d.readThread(raw)
	if err != nil {
		return "", err
	}
	d.publish()
	d.c.conv.setQuiet(true)
	defer d.c.conv.setQuiet(false)
	if err := d.readHistory(ctx, t); err != nil {
		d.c.conv.notice("Codex did not read the conversation back: " + err.Error())
	}
	return t.Thread.ID, nil
}

func (d *codexDriver) readThread(raw json.RawMessage) (codexThread, error) {
	var t codexThread
	if err := json.Unmarshal(raw, &t); err != nil || t.Thread.ID == "" {
		return t, fmt.Errorf("Codex opened a conversation with no id")
	}
	d.mu.Lock()
	d.thread = t.Thread.ID
	d.run.model, d.run.effort, d.run.mode = t.Model, t.Effort, codexModeOf(t.Sandbox)
	d.run.on = codexProducer{Model: t.Model, Effort: t.Effort}
	d.sources = t.Sources
	d.mu.Unlock()
	return t, nil
}

func (d *codexDriver) threadID() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.thread
}

func (d *codexDriver) turnID() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.turn
}

func textInput(text string) []map[string]any {
	return []map[string]any{{"type": "text", "text": text, "text_elements": []any{}}}
}

// prompt starts a turn, carrying what the person set since the last one:
// turn/start's overrides hold for the turns after it too (measured).
func (d *codexDriver) prompt(m message) error {
	if codexCompactTyped(m) {
		return d.compact()
	}
	params := map[string]any{"threadId": d.threadID(), "input": d.input(m)}
	staged := d.staged()
	maps.Copy(params, staged.params())
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	raw, err := d.call(ctx, "turn/start", params)
	if err != nil {
		return err
	}
	d.unstage(staged)
	var started struct {
		Turn struct {
			ID string `json:"id"`
		} `json:"turn"`
	}
	_ = json.Unmarshal(raw, &started)
	d.setTurn(started.Turn.ID)
	return nil
}

func (d *codexDriver) runsShell() bool { return true }

// shell runs the command as the TUI's `!` does, in a turn of its own Codex
// starts and ends, with the thread's shell, unsandboxed; the agent sees it
// on its next turn (measured on 0.170.0).
func (d *codexDriver) shell(command string) error {
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	_, err := d.call(ctx, "thread/shellCommand", map[string]any{"threadId": d.threadID(), "command": command})
	return err
}

func (d *codexDriver) setTurn(id string) {
	if id == "" {
		return
	}
	d.mu.Lock()
	d.turn = id
	d.mu.Unlock()
}

// follow adds the message to the turn running, turn/steer's job; a turn the
// decline already ended takes it as a new one.
func (d *codexDriver) follow(m message) error {
	turn := d.turnID()
	if turn == "" {
		go d.c.sendNext(pending{msg: m})
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	_, err := d.call(ctx, "turn/steer", map[string]any{"threadId": d.threadID(), "expectedTurnId": turn, "input": d.input(m)})
	return err
}

// cancel interrupts the turn, which then completes as interrupted.
func (d *codexDriver) cancel() error {
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	_, err := d.call(ctx, "turn/interrupt", map[string]any{"threadId": d.threadID(), "turnId": d.turnID()})
	return err
}

func (d *codexDriver) stopped() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for id, reply := range d.calls {
		close(reply)
		delete(d.calls, id)
	}
}

func (d *codexDriver) handle(raw []byte) {
	var m acpMessage
	if json.Unmarshal(raw, &m) != nil {
		return
	}
	switch {
	case m.Method != "" && len(m.ID) > 0:
		d.request(m)
	case m.Method != "":
		d.notification(m.Method, m.Params)
	case len(m.ID) > 0:
		d.reply(m)
	}
}

func (d *codexDriver) reply(m acpMessage) {
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

// codexItemEvent is item/started's and item/completed's params.
type codexItemEvent struct {
	Item codexItem `json:"item"`
}

// codexDelta is a streamed piece of an item's text.
type codexDelta struct {
	ItemID string `json:"itemId"`
	Delta  string `json:"delta"`
}

type codexTurnEvent struct {
	Turn struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	} `json:"turn"`
}

// notification reads what Codex reports. The person's own message comes back
// as a userMessage item, which lich already showed; an `error` is followed by
// the turn/completed that carries it, or is retried.
func (d *codexDriver) notification(method string, params json.RawMessage) {
	switch method {
	case "turn/started":
		var e codexTurnEvent
		_ = json.Unmarshal(params, &e)
		d.setTurn(e.Turn.ID)
	case "turn/completed":
		d.completed(params)
	case "item/started", "item/completed":
		var e codexItemEvent
		if json.Unmarshal(params, &e) == nil {
			d.item(e.Item, method == "item/completed")
		}
	case "thread/tokenUsage/updated":
		d.tokenUsage(params)
	case "item/agentMessage/delta":
		d.delta(params, api.ItemAgent)
	case "item/reasoning/summaryTextDelta":
		d.delta(params, api.ItemThought)
	case "turn/plan/updated":
		d.c.conv.plan(codexPlan(params))
	case "thread/settings/updated":
		d.settingsUpdated(params)
	case "serverRequest/resolved":
		d.resolved(params)
	case "skills/changed":
		// Off the reader: the call's answer comes in on it.
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
			defer cancel()
			d.listSkills(ctx)
		}()
	}
}

func (d *codexDriver) completed(params json.RawMessage) {
	var e codexTurnEvent
	_ = json.Unmarshal(params, &e)
	d.mu.Lock()
	d.turn = ""
	d.mu.Unlock()
	switch e.Turn.Status {
	case "interrupted":
		d.c.endTurn(stopCancelled)
	case "failed":
		if e.Turn.Error != nil && e.Turn.Error.Message != "" {
			d.c.failure(e.Turn.Error.Message)
		}
		d.c.endTurn(stopError)
	default:
		d.c.endTurn(stopEndTurn)
	}
}

func (d *codexDriver) delta(params json.RawMessage, kind string) {
	var p codexDelta
	if json.Unmarshal(params, &p) != nil || p.ItemID == "" || p.Delta == "" {
		return
	}
	model, effort := d.producer()
	d.c.conv.update(p.ItemID, kind, false, func(it *api.ChatItem) {
		it.Text += p.Delta
		attribute(it, model, effort)
	})
}

// item shows one item as it starts or completes, live.
func (d *codexDriver) item(it codexItem, done bool) {
	if it.Type == codexCompactItem {
		d.compactItem(done)
		return
	}
	model, effort := d.producer()
	ci, ok := it.under(d.c.cwd).chatItem(done)
	// A message or a reasoning that has only begun is empty: its deltas fill
	// it, and one that ends empty (a reasoning with no summary) is not drawn.
	if !ok || (ci.Kind != api.ItemTool && ci.Text == "") {
		return
	}
	_, known := d.c.conv.get(ci.ID)
	d.c.conv.update(ci.ID, ci.Kind, done, func(prev *api.ChatItem) {
		prev.Kind, prev.Text, prev.Tool = ci.Kind, ci.Text, ci.Tool
		if prev.At == 0 {
			prev.At = ci.At
		}
		attribute(prev, model, effort)
	})
	if ci.Tool != nil && !known {
		d.c.toolStarted(ci.Tool.Name, ci.Tool.Title)
	}
}

// resolved forgets a request Codex answered itself (a turn interrupted under
// it), so its card stops asking.
func (d *codexDriver) resolved(params json.RawMessage) {
	var p struct {
		ID json.RawMessage `json:"requestId"`
	}
	_ = json.Unmarshal(params, &p)
	key := strings.Trim(string(p.ID), `"`)
	d.mu.Lock()
	itemID, ok := d.asks[key]
	delete(d.asks, key)
	d.mu.Unlock()
	if ok {
		d.c.withdraw(itemID)
	}
}

// track remembers which item request id shows.
func (d *codexDriver) track(id json.RawMessage, itemID string) {
	d.mu.Lock()
	d.asks[strings.Trim(string(id), `"`)] = itemID
	d.mu.Unlock()
}

// refuse answers a request lich does not serve, which the protocol expects of
// a client that does not.
func (d *codexDriver) refuse(m acpMessage) {
	err := d.c.out.send(map[string]any{"jsonrpc": "2.0", "id": m.ID, "error": map[string]any{"code": -32601, "message": "lich does not serve " + m.Method}})
	if err != nil {
		slog.Debug("chat: refuse a Codex request", "session", d.c.id, "err", err)
	}
}
