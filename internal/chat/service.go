// Package chat runs a session as a chat (api/chat.go): its provider's CLI in
// the CLI's structured mode, Claude Code's stream-json or the Agent Client
// Protocol, read into items the window draws. A session runs either its chat or
// its terminal, never both, and switching between them resumes the same
// conversation on the other side.
package chat

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/prompt"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/store"
	"github.com/lichdotdev/lich/internal/terminal"
)

// Kinds is the providers whose sessions can be drawn as a chat. Codex is out:
// its structured mode is its own app-server protocol. Crush is out: its server
// speaks HTTP, not stdio. Cursor's ACP server is in unmeasured, behind a login
// (docs/ceilings.md).
var Kinds = []string{providers.Claude, providers.OpenCode, providers.OMP, providers.Antigravity, providers.Kiro, providers.Cursor}

// openTimeout bounds a chat's handshake and the conversation it opens, which
// for a long resumed one is the whole replay.
const openTimeout = 90 * time.Second

// callTimeout bounds every other request lich makes of a provider.
const callTimeout = 30 * time.Second

var errBusy = errors.New("the agent is working; wait for the turn to end or stop it first")

// Service runs every session's chat.
type Service struct {
	term  *terminal.Service
	store *store.Service
	hub   *events.Hub
	lang  func() prompt.Lang

	mu        sync.Mutex
	sessions  map[string]*chatSession
	switching map[string]bool
}

// New is a chat service running its providers through term.
func New(term *terminal.Service, db *store.Service, hub *events.Hub, lang func() prompt.Lang) *Service {
	return &Service{term: term, store: db, hub: hub, lang: lang, sessions: map[string]*chatSession{}, switching: map[string]bool{}}
}

// Kinds answers api.ChatKinds.
func (s *Service) Kinds(context.Context, api.None) ([]string, error) {
	return slices.Clone(Kinds), nil
}

// SetView switches session opts.ID between its terminal and its chat: the side
// it leaves stops and is waited out, then the other resumes the conversation,
// and the window hears of the switch once the new side runs.
func (s *Service) SetView(ctx context.Context, opts api.ChatViewOptions) (api.None, error) {
	if opts.View != api.ViewChat && opts.View != api.ViewTerminal {
		return api.None{}, fmt.Errorf("unknown view %q", opts.View)
	}
	release, err := s.claim(opts.ID)
	if err != nil {
		return api.None{}, err
	}
	defer release()
	if opts.View == api.ViewChat {
		err = s.toChat(ctx, opts.ID)
	} else {
		err = s.toTerminal(ctx, opts.ID)
	}
	if err != nil {
		return api.None{}, err
	}
	events.Send(s.hub.Emit, api.SessionViewEvent, api.SessionView{ID: opts.ID, View: opts.View})
	return api.None{}, nil
}

// claim holds session id for one switch or start at a time.
func (s *Service) claim(id string) (func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.switching[id] {
		return nil, errors.New("this session is already switching")
	}
	s.switching[id] = true
	return func() {
		s.mu.Lock()
		delete(s.switching, id)
		s.mu.Unlock()
	}, nil
}

func (s *Service) toChat(ctx context.Context, id string) error {
	projectID, kind, cwd, err := s.store.SessionSpawn(id)
	if err != nil {
		return err
	}
	if !slices.Contains(Kinds, kind) {
		return fmt.Errorf("a %s session cannot be drawn as a chat", kind)
	}
	if s.term.TurnOpen(id) {
		return errBusy
	}
	if s.running(id) != nil {
		return s.store.SetSessionView(id, api.ViewChat)
	}
	if err := s.term.StopTerminal(id); err != nil {
		return err
	}
	if _, err := s.start(ctx, id, projectID, kind, cwd, s.storedCarry(id)); err != nil {
		return errors.Join(err, s.resumeTerminal(id, projectID, kind, cwd))
	}
	return s.store.SetSessionView(id, api.ViewChat)
}

func (s *Service) toTerminal(ctx context.Context, id string) error {
	projectID, kind, cwd, err := s.store.SessionSpawn(id)
	if err != nil {
		return err
	}
	if c := s.running(id); c != nil {
		if c.turnRunning() || s.term.TurnOpen(id) {
			return errBusy
		}
		c.stop()
	}
	if err := s.store.SetSessionView(id, api.ViewTerminal); err != nil {
		return err
	}
	return s.resumeTerminal(id, projectID, kind, cwd)
}

// resumeTerminal brings session id's terminal up on its conversation, the way
// the window would on a resume it accepted, so the window only attaches to it.
func (s *Service) resumeTerminal(id, projectID, kind, cwd string) error {
	conversation, err := s.store.ProviderSession(id)
	if err != nil {
		return err
	}
	if !s.term.Resumable(kind, conversation, cwd) {
		conversation = ""
	}
	_, err = s.term.Start(context.Background(), api.StartOptions{ID: id, ProjectID: projectID, Cwd: cwd, Kind: kind, Resume: conversation})
	return err
}

// running is session id's chat, nil when its process is not running.
func (s *Service) running(id string) *chatSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[id]
}

// forget drops c once its process is gone.
func (s *Service) forget(c *chatSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions[c.id] == c {
		delete(s.sessions, c.id)
	}
}

// start spawns session id's chat on its conversation and opens it on the
// settings carry names.
func (s *Service) start(ctx context.Context, id, projectID, kind, cwd string, carry map[string]string) (*chatSession, error) {
	resume, err := s.store.ProviderSession(id)
	if err != nil {
		return nil, err
	}
	lost := resume != "" && !s.term.Resumable(kind, resume, cwd)
	if lost {
		resume = ""
	}
	launch := launchFor(kind, carry)
	proc, err := s.term.StartChat(ctx, api.StartOptions{ID: id, ProjectID: projectID, Cwd: cwd, Kind: kind, Resume: resume}, launch)
	if err != nil {
		return nil, err
	}
	c := &chatSession{
		svc: s, id: id, kind: kind, projectID: projectID, cwd: cwd, proc: proc,
		out: &wire{w: proc.Stdin}, perms: map[string]permission{}, questions: map[string]question{}, exited: make(chan struct{}),
	}
	c.conv = newConversation(func(it api.ChatItem) {
		events.Send(s.hub.Emit, api.ChatItemEvent, api.ChatItemUpdate{SessionID: id, Item: it})
	})
	if kind == providers.Claude {
		c.drv = newClaudeDriver(c, s.lang())
	} else {
		c.drv = newACPDriver(c, launch.Server)
	}
	if lost {
		c.conv.notice("The conversation did not come back: the agent no longer has it, so a new one starts.")
	}
	if kind == providers.Claude && resume != "" {
		readClaudeHistory(c, resume)
	}
	s.mu.Lock()
	s.sessions[id] = c
	s.mu.Unlock()
	go c.read()
	openCtx, cancel := context.WithTimeout(ctx, openTimeout)
	defer cancel()
	opened, err := c.drv.open(openCtx, resume)
	if err != nil {
		c.stop()
		return nil, err
	}
	c.bind(opened, "")
	c.applyCarry(openCtx, carry)
	return c, nil
}

// ensure is session id's running chat, started on its conversation when the
// session is drawn as a chat and its process is not running: after a restart of
// lich, or a process that exited.
func (s *Service) ensure(ctx context.Context, id string) (*chatSession, error) {
	if c := s.running(id); c != nil {
		return c, nil
	}
	if s.store.SessionView(id) != api.ViewChat {
		return nil, errors.New("this session is drawn as its terminal, not as a chat")
	}
	release, err := s.claim(id)
	if err != nil {
		return nil, err
	}
	defer release()
	if c := s.running(id); c != nil {
		return c, nil
	}
	projectID, kind, cwd, err := s.store.SessionSpawn(id)
	if err != nil {
		return nil, err
	}
	return s.start(ctx, id, projectID, kind, cwd, s.storedCarry(id))
}

// History answers api.ChatHistory.
func (s *Service) History(ctx context.Context, opts api.SessionIDOptions) ([]api.ChatItem, error) {
	c, err := s.ensure(ctx, opts.ID)
	if err != nil {
		return nil, err
	}
	return c.conv.snapshot(), nil
}

// Prompt answers api.ChatPrompt.
func (s *Service) Prompt(ctx context.Context, opts api.ChatPromptOptions) (api.None, error) {
	c, err := s.ensure(ctx, opts.ID)
	if err != nil {
		return api.None{}, err
	}
	if s.term.TurnOpen(opts.ID) && !c.turnRunning() {
		return api.None{}, errBusy
	}
	// A message written while the agent waits on its questions answers none
	// of them, and goes to the agent after.
	if c.declineQuestions() {
		c.conv.userMessage(opts.Text)
		return api.None{}, c.drv.follow(opts.Text)
	}
	if err := c.beginTurn(); err != nil {
		return api.None{}, err
	}
	c.conv.userMessage(opts.Text)
	return api.None{}, c.send(opts.Text)
}

// Cancel answers api.ChatCancel.
func (s *Service) Cancel(_ context.Context, opts api.SessionIDOptions) (api.None, error) {
	c := s.running(opts.ID)
	if c == nil || !c.turnRunning() {
		return api.None{}, errNoTurn
	}
	c.dismissPermissions()
	return api.None{}, c.drv.cancel()
}

// Answer answers api.ChatAnswer.
func (s *Service) Answer(_ context.Context, opts api.ChatAnswerOptions) (api.None, error) {
	c := s.running(opts.ID)
	if c == nil {
		return api.None{}, fmt.Errorf("session %q has no chat running", opts.ID)
	}
	return api.None{}, c.answer(opts.ItemID, opts.OptionID)
}

// Reply answers api.ChatReply.
func (s *Service) Reply(_ context.Context, opts api.ChatReplyOptions) (api.None, error) {
	c := s.running(opts.ID)
	if c == nil {
		return api.None{}, fmt.Errorf("session %q has no chat running", opts.ID)
	}
	return api.None{}, c.reply(opts.ItemID, opts.Reply)
}

// End stops session id's chat, for a card that closed.
func (s *Service) End(id string) {
	if c := s.running(id); c != nil {
		c.stop()
	}
}

// Shutdown stops every chat, on the way out of lich.
func (s *Service) Shutdown() {
	s.mu.Lock()
	running := make([]*chatSession, 0, len(s.sessions))
	for _, c := range s.sessions {
		running = append(running, c)
	}
	s.mu.Unlock()
	var wg sync.WaitGroup
	for _, c := range running {
		wg.Go(c.stop)
	}
	wg.Wait()
}
