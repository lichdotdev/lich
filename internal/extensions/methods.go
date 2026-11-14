package extensions

import (
	"context"
	"fmt"
	"slices"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/internal/relay"
)

// Every method calls its service with no From: an extension is no session,
// so it reaches lich as `lich` run outside one does, and names the project
// it means. What it sends carries the extension's identity, so the target is
// told who relayed it.

func (s *Service) listProjects(ctx context.Context, _ *installed, _ extension.NoParams) ([]extension.Project, error) {
	projects, err := s.lich.Projects(ctx, api.None{})
	if err != nil {
		return nil, fmt.Errorf("read the workspace: %w", err)
	}
	out := make([]extension.Project, 0, len(projects))
	for _, p := range projects {
		out = append(out, extension.Project{ID: p.ID, Name: p.Name, Path: p.Path})
	}
	return out, nil
}

func (s *Service) listSessions(ctx context.Context, _ *installed, _ extension.NoParams) ([]extension.Session, error) {
	peers, err := s.lich.Peers(ctx, api.PeersOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]extension.Session, 0, len(peers))
	for _, p := range peers {
		out = append(out, extension.Session{
			Label: p.Label, Name: p.Name, Project: p.Project, Kind: p.Kind, State: p.State, ID: p.ID,
			Path: p.Path, ProjectPath: p.ProjectPath, Tool: p.Tool, Detail: p.Detail, Reason: p.Reason,
			Focused: p.Focused,
		})
	}
	return out, nil
}

func (s *Service) listWorktrees(ctx context.Context, _ *installed, params extension.WorktreesParams) ([]extension.Worktree, error) {
	checkouts, err := s.lich.Worktrees(ctx, api.WorktreesOptions{Project: params.Project})
	if err != nil {
		return nil, err
	}
	out := make([]extension.Worktree, 0, len(checkouts))
	for _, c := range checkouts {
		out = append(out, extension.Worktree{Name: c.Name, Path: c.Path, Dirty: c.Dirty, Sessions: c.Sessions})
	}
	return out, nil
}

// openSession reports a first task that did not arrive beside the session
// rather than as a failed call, as open_session does: the session is open
// either way, and a failed call would read as if it were not.
func (s *Service) openSession(ctx context.Context, x *installed, params extension.OpenSessionParams) (extension.OpenedSession, error) {
	opened, err := s.lich.Open(ctx, api.OpenSessionOptions{
		Project: params.Project, Kind: params.Provider, Worktree: params.Worktree,
	})
	if err != nil {
		return extension.OpenedSession{}, err
	}
	out := extension.OpenedSession{
		ID: opened.ID, ProjectID: opened.ProjectID, Project: opened.Project, Label: opened.Label,
		Name: opened.Name, Kind: opened.Kind, Path: opened.Path, Confined: opened.Confined,
	}
	if params.Prompt == "" {
		return out, nil
	}
	result, err := s.lich.Send(ctx, api.SendOptions{
		Target: opened.Label, Project: opened.Project, Prompt: params.Prompt, WaitSeconds: handOverWaitSeconds,
		ExtensionID: x.id, ExtensionName: s.promptName(x),
	})
	if err != nil {
		out.TaskError = err.Error()
		return out, nil
	}
	task := s.held(x, result)
	out.Task = &task
	return out, nil
}

func (s *Service) sendToSession(ctx context.Context, x *installed, params extension.SendParams) (extension.SendResult, error) {
	result, err := s.lich.Send(ctx, api.SendOptions{
		Target: params.Session, Project: params.Project, Prompt: params.Prompt, WaitSeconds: clampWait(params.WaitSeconds),
		ExtensionID: x.id, ExtensionName: s.promptName(x),
	})
	if err != nil {
		return extension.SendResult{}, err
	}
	return s.held(x, result), nil
}

// promptName is x's name in the prompt language, its manifest's when that
// language has no translation of it.
func (s *Service) promptName(x *installed) string {
	if s.lich.PromptLang != nil {
		if name := x.text[string(s.lich.PromptLang())].Name; name != "" {
			return name
		}
	}
	return x.manifest.Name
}

// clampWait is send_to_session's and wait_for_answer's cap on holding the
// line, and what they hold for when none is asked.
func clampWait(seconds int) int {
	if seconds <= 0 || seconds > maxSendWaitSeconds {
		return maxSendWaitSeconds
	}
	return seconds
}

// held records a pending result's ticket as x's: the relay redeems any ticket
// for anyone, so the one owner check is here.
func (s *Service) held(x *installed, r api.SendResult) extension.SendResult {
	if r.Status == relay.StatusPending {
		s.mu.Lock()
		if x.tickets == nil {
			x.tickets = map[string]bool{}
		}
		x.tickets[r.Ticket] = true
		s.mu.Unlock()
	}
	return sendResult(r)
}

// collectAnswer waits on one of x's tickets, which it holds until an outcome
// other than pending comes back.
func (s *Service) collectAnswer(ctx context.Context, x *installed, params extension.AnswerParams) (extension.SendResult, error) {
	s.mu.Lock()
	owned := x.tickets[params.Ticket]
	s.mu.Unlock()
	if !owned {
		return extension.SendResult{}, fmt.Errorf("ticket %q is not one this extension's sessions.send or sessions.open returned pending", params.Ticket)
	}
	result, err := s.lich.Wait(ctx, api.WaitOptions{Ticket: params.Ticket, WaitSeconds: clampWait(params.WaitSeconds)})
	if err != nil {
		return extension.SendResult{}, err
	}
	if result.Status != relay.StatusPending {
		s.mu.Lock()
		delete(x.tickets, params.Ticket)
		s.mu.Unlock()
	}
	return sendResult(result), nil
}

func (s *Service) askSession(ctx context.Context, _ *installed, params extension.AskParams) (extension.AskResult, error) {
	answer, err := s.lich.Ask(ctx, api.AskSessionOptions{Target: params.Session, Project: params.Project, Question: params.Question})
	if err != nil {
		return extension.AskResult{}, err
	}
	return extension.AskResult{ID: answer.ID, Project: answer.Project, Label: answer.Label, Answer: answer.Answer}, nil
}

func sendResult(r api.SendResult) extension.SendResult {
	return extension.SendResult{Ticket: r.Ticket, Target: r.Target, Status: r.Status, Answer: r.Answer}
}

// subscribe checks every name before starting any, so a refused call starts
// nothing.
func (s *Service) subscribe(_ context.Context, x *installed, params extension.SubscribeParams) (extension.NoParams, error) {
	for _, name := range params.Names {
		permission, known := extension.EventPermission(name)
		if !known {
			return extension.NoParams{}, fmt.Errorf("no event %q", name)
		}
		if !slices.Contains(x.manifest.Permissions, permission) {
			return extension.NoParams{}, missingPermission("the event "+name, permission)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if x.subscribed == nil {
		x.subscribed = map[string]bool{}
	}
	for _, name := range params.Names {
		x.subscribed[name] = true
	}
	return extension.NoParams{}, nil
}

// Observe hands every app event to the extensions subscribed to it. It runs
// on the emitter's goroutine, so it only queues.
func (s *Service) Observe(name string, data any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch name {
	case api.SessionStatusEvent.Name:
		if status, ok := data.(api.SessionStatus); ok {
			s.broadcast(extension.SessionStatus.Name, extension.SessionStatus.New(extension.SessionStatusData{
				ID: status.ID, State: status.State, Tool: status.Tool, Detail: status.Detail, Reason: status.Reason,
			}))
		}
	case api.ServicesEvent.Name:
		if list, ok := data.([]api.Service); ok {
			s.observeServices(list)
		}
	}
}
