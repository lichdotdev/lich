package subagent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/relay"
)

// removeWorktree is spawn.RemoveWorktree: closing a card that has its own
// worktree takes the checkout with it, nothing having been done there yet.
const removeWorktree = "remove"

// open opens every task of req as a session and hands it its task, all of
// them or none: past the budget, or on any failure, what was opened is closed
// and the call runs natively.
func (s *Service) open(ctx context.Context, req api.SubagentRequest, asker Asker) (api.SubagentAnswer, func(bool)) {
	ctx, cancel := context.WithTimeout(ctx, s.budget)
	defer cancel()
	// Unbuffered: the opener either hands its sessions to a decision still
	// waiting for them or, once that decision gave up, closes them itself.
	result := make(chan []api.SubagentOpened)
	abandoned := make(chan struct{})
	go func() {
		opened, err := s.openAll(ctx, req)
		if err != nil {
			slog.Warn("subagent: open a card", "session", req.SessionID, "err", err)
			s.closeAll(req, opened)
			opened = nil
		}
		select {
		case result <- opened:
		case <-abandoned:
			s.closeAll(req, opened)
		}
	}()
	select {
	case opened := <-result:
		if opened == nil {
			return s.allowed(req)
		}
		answer := api.SubagentAnswer{Decision: api.SubagentDeny, Reason: s.openedReason(opened), Sessions: opened}
		return s.denied(req, answer, opened, asker)
	case <-ctx.Done():
		close(abandoned)
		return s.allowed(req)
	}
}

// openAll opens and hands over each task in turn, returning what it opened
// so far with the first failure.
func (s *Service) openAll(ctx context.Context, req api.SubagentRequest) ([]api.SubagentOpened, error) {
	var opened []api.SubagentOpened
	for _, t := range req.Tasks {
		if err := ctx.Err(); err != nil {
			return opened, err
		}
		worktree := ""
		if req.Isolation == "worktree" {
			worktree = worktreeName(t)
		}
		sess, err := s.spawner.OpenSubagentNow(ctx, api.OpenSubagentOptions{From: req.SessionID, Worktree: worktree, Model: strings.TrimSpace(req.Model)})
		if err != nil {
			return opened, err
		}
		o := api.SubagentOpened{ID: sess.ID, Label: sess.Label}
		opened = append(opened, o)
		res, err := s.relay.HandSubagent(ctx, relay.SendOptions{From: req.SessionID, Target: sess.ID, Prompt: t.Prompt})
		if err != nil {
			return opened, fmt.Errorf("hand %q its task: %w", sess.Label, err)
		}
		opened[len(opened)-1].Ticket = res.Ticket
	}
	return opened, nil
}

// denied is answer as a deny whose commit remembers it once written, and
// closes what it opened when it was not.
func (s *Service) denied(req api.SubagentRequest, answer api.SubagentAnswer, opened []api.SubagentOpened, asker Asker) (api.SubagentAnswer, func(bool)) {
	return answer, func(written bool) {
		if !written {
			s.closeAll(req, opened)
			s.mu.Lock()
			s.conv(req).allowed = true
			s.mu.Unlock()
			return
		}
		s.remember(req, answer, opened)
	}
}

// remember files a written deny: by its tool call for a repeat, by its
// prompt while the worker runs, against its conversation for the waits, and
// as a veto the asker's chat draws.
func (s *Service) remember(req api.SubagentRequest, answer api.SubagentAnswer, opened []api.SubagentOpened) {
	s.mu.Lock()
	s.conv(req).denied++
	if req.ToolUseID != "" {
		s.answers[req.SessionID+"\x00"+req.ToolUseID] = answer
	}
	if len(req.Tasks) == 1 && len(opened) == 1 {
		s.running[promptKey(req.SessionID, req.Tasks[0].Prompt)] = opened
	}
	s.mu.Unlock()
	s.veto(req, opened)
}

// veto tells the asker's chat a written deny's call was opened in opened, and
// keeps it for a window that attaches later. A call with no id or nothing
// opened has nothing to draw.
func (s *Service) veto(req api.SubagentRequest, opened []api.SubagentOpened) {
	if req.ToolUseID == "" || len(opened) == 0 {
		return
	}
	veto := api.SubagentVeto{SessionID: req.SessionID, ToolUseID: req.ToolUseID, Sessions: opened}
	s.mu.Lock()
	s.vetoes = append(s.vetoes, veto)
	s.mu.Unlock()
	if s.emit != nil {
		s.emit(veto)
	}
}

// closeAll closes what a call opened, as the asker, so no worker-stopped
// reaches it for a task its native subagent is doing instead.
func (s *Service) closeAll(req api.SubagentRequest, opened []api.SubagentOpened) {
	for _, o := range opened {
		opts := api.CloseSessionOptions{From: req.SessionID, Target: o.ID}
		if req.Isolation == "worktree" {
			opts.Worktree = removeWorktree
		}
		if _, err := s.spawner.Close(context.Background(), opts); err != nil {
			slog.Warn("subagent: close a card the hook never heard of", "session", o.ID, "err", err)
		}
	}
}

// worktreeName is the branch an isolated task gets its checkout on: its title
// in a few words, or task, under subagent/, with a short suffix so two tasks
// named alike do not collide.
func worktreeName(t api.SubagentTask) string {
	return "subagent/" + slug(t.Title) + "-" + suffix()
}
