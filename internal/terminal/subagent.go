package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/scope"
)

// subagentBodyLimit holds a /subagent body to 64 KiB, room for several tasks
// at the 8 KB a prompt may run to (docs/hooks/subagent-cards.md, Request).
const subagentBodyLimit = 64 << 10

// subagentRequest is the /subagent body (docs/hooks/subagent-cards.md).
type subagentRequest struct{ api.SubagentRequest }

func (r subagentRequest) session() string { return r.SessionID }

// parseSubagentRequest reads a /subagent body, defaulting call to spawn and
// trimming isolation.
func parseSubagentRequest(body []byte) (subagentRequest, error) {
	var req subagentRequest
	if err := json.Unmarshal(body, &req.SubagentRequest); err != nil {
		return subagentRequest{}, fmt.Errorf("invalid subagent body: %w", err)
	}
	if req.SessionID == "" {
		return subagentRequest{}, errors.New("subagent request missing session_id")
	}
	if strings.TrimSpace(req.Tool) == "" {
		return subagentRequest{}, errors.New("subagent request missing tool")
	}
	if req.Call == "" {
		req.Call = api.SubagentSpawn
	}
	req.Isolation = strings.TrimSpace(req.Isolation)
	if req.Isolation != "" && req.Isolation != "worktree" {
		return subagentRequest{}, fmt.Errorf("subagent isolation %q is not worktree", req.Isolation)
	}
	if req.Tasks == nil {
		req.Tasks = []api.SubagentTask{}
	}
	return req, checkSubagentTasks(req.SubagentRequest)
}

// checkSubagentTasks holds a call to its tasks: a spawn starts at least one,
// each with a prompt unless the provider sealed them, and a wait starts none.
func checkSubagentTasks(req api.SubagentRequest) error {
	switch req.Call {
	case api.SubagentWait:
		if len(req.Tasks) > 0 {
			return errors.New("a subagent wait carries no tasks")
		}
		return nil
	case api.SubagentSpawn:
	default:
		return fmt.Errorf("subagent call %q is not spawn or wait", req.Call)
	}
	if len(req.Tasks) == 0 {
		return errors.New("a subagent spawn starts at least one task")
	}
	for _, t := range req.Tasks {
		if !req.Sealed && strings.TrimSpace(t.Prompt) == "" {
			return errors.New("every subagent task needs a prompt")
		}
	}
	return nil
}

// subagentDecider is what answers a /subagent call (internal/subagent).
type subagentDecider func(context.Context, api.SubagentRequest) (api.SubagentAnswer, func(written bool))

// subagentHook answers a provider's own subagent call with lich's decision,
// 200 and JSON whatever it is.
// A deny that never reached the client, because its request ended first or
// the write failed, is committed as unwritten, which closes what it opened.
func (t *transport) subagentHook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	c, ok := t.callerOf(r)
	if !ok {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, subagentBodyLimit))
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	plugin := r.Header.Get(pluginVersionHeader)
	req, err := parseSubagentRequest(body)
	if err != nil {
		slog.Warn("hook: bad payload", "path", r.URL.Path, "plugin", pluginLabel(plugin), "err", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !c.ownSession(req.SessionID) {
		http.Error(w, "the token belongs to another session", http.StatusForbidden)
		return
	}
	t.plugins.note(req.SessionID, plugin)
	t.answerSubagent(w, r, c, req.SubagentRequest)
}

func (t *transport) answerSubagent(w http.ResponseWriter, r *http.Request, c caller, req api.SubagentRequest) {
	t.mu.Lock()
	decide := t.decideSubagent
	t.mu.Unlock()
	answer, commit := api.SubagentAnswer{Decision: api.SubagentAllow}, func(bool) {}
	if decide != nil {
		ctx := r.Context()
		if c.scoped() {
			ctx = scope.With(ctx, c.session)
		}
		answer, commit = decide(ctx, req)
	}
	body, err := json.Marshal(answer)
	if err != nil || r.Context().Err() != nil {
		commit(false)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(body)
	commit(err == nil)
}

// SetSubagent wires what answers /subagent: internal/subagent's Decide.
// Startup wiring, before any session runs; with none every call is allowed.
func (s *Service) SetSubagent(fn func(context.Context, api.SubagentRequest) (api.SubagentAnswer, func(written bool))) {
	if s.ws == nil {
		return
	}
	s.ws.mu.Lock()
	s.ws.decideSubagent = fn
	s.ws.mu.Unlock()
}
