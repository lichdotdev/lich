package extensions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/internal/prompt"
)

const (
	// maxCallsInFlight is how many calls one extension has running at once;
	// one past it is refused, so a loop gone wrong holds a bounded share of
	// lich rather than queueing without end.
	maxCallsInFlight = 8
	// callTimeout bounds one call. The longest, sessions.open with a prompt,
	// spends at most the 60s an open is given plus the 20s its first task is
	// held for, as open_session does (internal/cli, openCall and deliverWait).
	callTimeout = 2 * time.Minute
	// maxSendWaitSeconds is send_to_session's cap on holding for a quick
	// answer, and what it holds for when none is asked (internal/cli, mcpMaxWait).
	maxSendWaitSeconds = 90
	// handOverWaitSeconds is how long open_session holds its first task for
	// (internal/cli, deliverWait).
	handOverWaitSeconds = 20

	msgUndelivered = "extensions: dropped a line to the extension"
)

// Lich is what an extension's calls reach: the services lich's MCP tools
// call, each called as the lich command line outside a session calls it.
type Lich struct {
	Projects  func(context.Context, api.None) ([]api.Project, error)
	Peers     func(context.Context, api.PeersOptions) ([]api.Peer, error)
	Worktrees func(context.Context, api.WorktreesOptions) ([]api.WorktreeCheckout, error)
	Open      func(context.Context, api.OpenSessionOptions) (api.OpenedSession, error)
	Send      func(context.Context, api.SendOptions) (api.SendResult, error)
	Wait      func(context.Context, api.WaitOptions) (api.SendResult, error)
	Ask       func(context.Context, api.AskSessionOptions) (api.SessionAnswer, error)

	Project        func(context.Context, api.ProjectRefOptions) (api.ProjectRef, error)
	Services       func(context.Context, api.None) ([]api.Service, error)
	StartService   func(context.Context, api.ServiceIDOptions) (api.None, error)
	StopService    func(context.Context, api.ServiceIDOptions) (api.None, error)
	RestartService func(context.Context, api.ServiceIDOptions) (api.None, error)
	Output         func(context.Context, api.ServiceIDOptions) ([]byte, error)
	LiveOutput     func(context.Context, api.ServiceIDOptions) ([]byte, error)

	Diff        func(context.Context, api.PathOptions) (api.DiffStats, error)
	BaseStatus  func(context.Context, api.PathOptions) (*api.BaseStatus, error)
	Upstream    func(context.Context, api.PathOptions) (api.BaseFreshness, error)
	PullRequest func(context.Context, api.PathOptions) (*api.PullRequest, error)

	Notify   func(context.Context, api.NotifyOptions) (api.None, error)
	ShowCode func(context.Context, api.ShowCodeOptions) (api.ShowCode, error)
	OpenURL  func(context.Context, api.BrowserOpenOptions) (api.BrowserHandle, error)
	// Attached is whether a window is connected to show what ShowCode and
	// OpenURL do.
	Attached func() bool
	// PromptLang is the language an extension's name is worded in for the
	// agents it sends to.
	PromptLang func() prompt.Lang
}

// SetLich wires the services calls reach, before Boot.
func (s *Service) SetLich(lich Lich) {
	s.lich = lich
}

// route is one method: the permission it needs and how it runs.
type route struct {
	permission extension.Permission
	run        func(context.Context, *installed, extension.CallData) (extension.Message, error)
}

func handle[P, R any](m extension.Method[P, R], run func(context.Context, *installed, P) (R, error)) (string, route) {
	return m.Name, route{permission: m.Permission, run: func(ctx context.Context, x *installed, call extension.CallData) (extension.Message, error) {
		params, err := m.Params(call)
		if err != nil {
			return extension.Message{}, err
		}
		result, err := run(ctx, x, params)
		if err != nil {
			return extension.Message{}, err
		}
		return m.Answer(call.ID, result), nil
	}}
}

func (s *Service) routes() map[string]route {
	table := map[string]route{}
	add := func(name string, r route) { table[name] = r }
	add(handle(extension.ListProjects, s.listProjects))
	add(handle(extension.ListSessions, s.listSessions))
	add(handle(extension.ListWorktrees, s.listWorktrees))
	add(handle(extension.OpenSession, s.openSession))
	add(handle(extension.SendToSession, s.sendToSession))
	add(handle(extension.AskSession, s.askSession))
	add(handle(extension.CollectAnswer, s.collectAnswer))
	add(handle(extension.ListServices, s.listServices))
	add(handle(extension.StartService, s.serviceAction(func(l Lich) serviceAct { return l.StartService })))
	add(handle(extension.StopService, s.serviceAction(func(l Lich) serviceAct { return l.StopService })))
	add(handle(extension.RestartService, s.serviceAction(func(l Lich) serviceAct { return l.RestartService })))
	add(handle(extension.ServiceOutput, s.serviceOutput))
	add(handle(extension.GitStatus, s.gitStatus))
	add(handle(extension.GitPullRequest, s.gitPullRequest))
	add(handle(extension.Notify, s.notify))
	add(handle(extension.ShowCode, s.showCode))
	add(handle(extension.OpenURL, s.openURL))
	add(handle(extension.Subscribe, s.subscribe))
	return table
}

// call runs one call from x and answers it, an error included: an extension
// waiting on an id lich dropped would wait forever.
func (s *Service) call(x *installed, call extension.CallData) {
	r, known := s.calls[call.Method]
	if !known {
		s.answer(x, failed(call.ID, fmt.Errorf("no method %q", call.Method)))
		return
	}
	if r.permission != "" && !slices.Contains(x.manifest.Permissions, r.permission) {
		s.answer(x, failed(call.ID, missingPermission(call.Method, r.permission)))
		return
	}
	select {
	case x.inFlight <- struct{}{}:
	default:
		s.answer(x, failed(call.ID, fmt.Errorf("%d calls are already running, the most one extension may have", maxCallsInFlight)))
		return
	}
	s.mu.Lock()
	if s.ctx.Err() != nil {
		s.mu.Unlock()
		<-x.inFlight
		return
	}
	s.wg.Add(1)
	s.mu.Unlock()
	go func() {
		defer s.wg.Done()
		msg := s.run(x, r, call)
		// Freed before the answer: an extension at the limit that calls again
		// the moment it reads a result must find the slot.
		<-x.inFlight
		s.answer(x, msg)
	}()
}

func (s *Service) run(x *installed, r route, call extension.CallData) extension.Message {
	ctx, cancel := context.WithTimeout(s.ctx, s.timing.callTimeout)
	defer cancel()
	msg, err := r.run(ctx, x, call)
	if errors.Is(err, context.DeadlineExceeded) {
		err = fmt.Errorf("%s ran past its %s: %w", call.Method, s.timing.callTimeout, err)
	}
	if err != nil {
		return failed(call.ID, err)
	}
	return msg
}

func failed(id string, err error) extension.Message {
	return extension.CallResult.New(extension.ResultData{ID: id, Error: err.Error()})
}

func missingPermission(what string, p extension.Permission) error {
	return fmt.Errorf("%s needs the permission %q, which the extension's manifest does not list", what, p)
}

// answer writes msg to x's stdin, dropping it when x is gone or not reading:
// a full queue is an extension that stopped reading, and blocking on it would
// hold the caller.
func (s *Service) answer(x *installed, msg extension.Message) {
	line, err := json.Marshal(msg)
	if err != nil {
		x.refused.note(x.id, msgUndelivered, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enqueue(x, append(line, '\n'))
}

// enqueue is answer's write. Called with mu held.
func (s *Service) enqueue(x *installed, line []byte) {
	if x.queue == nil {
		return
	}
	select {
	case x.queue <- line:
	default:
		x.refused.note(x.id, msgUndelivered, "it is not reading its input")
	}
}
