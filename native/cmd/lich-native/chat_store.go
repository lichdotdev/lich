package main

import (
	"context"
	"log"
	"slices"
	"sync"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/native/backend"
)

// chatStore is every chat session's conversation as ChatHistory and the chat
// events left it, and the providers that can be drawn as a chat. switching is
// the window's own half: a view switch it asked for and has not heard land.
type chatStore struct {
	mu        sync.Mutex
	kinds     []string
	items     map[string][]backend.ChatItem
	running   map[string]bool
	ends      map[string]map[int]api.ChatTurn
	options   map[string][]backend.ChatOption
	commands  map[string][]backend.ChatCommand
	switching map[string]string
	// queue is each session's messages written while its turn ran, sent
	// together once it ends (chatQueue).
	queue map[string][]string
	// held is each session's message waiting to go (chatHold), or gone and
	// not yet heard back.
	held map[string]*chatHeld
}

func (s *chatStore) loadKinds(ctx context.Context, client *backend.Client) error {
	kinds, err := client.ChatKinds(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.kinds = kinds
	s.mu.Unlock()
	return nil
}

// offers reports whether a session of provider kind can be drawn as a chat.
// A plain terminal never is, whatever the backend lists.
func (s *chatStore) offers(kind string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return kind != "shell" && slices.Contains(s.kinds, kind)
}

// history takes session id's conversation whole. It is read with the event
// stream held behind it, so what was heard before it is older and goes, and
// what is heard after lands on top.
func (s *chatStore) history(id string, items []backend.ChatItem) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.items == nil {
		s.items = map[string][]backend.ChatItem{}
	}
	s.items[id] = slices.Clone(items)
	s.forgetSent(id)
	// The turns' numbers are the conversation's own, and a history read back
	// may number them anew: an end kept from before would close another turn.
	delete(s.ends, id)
}

// apply replaces the item with the update's id, or appends it.
func (s *chatStore) apply(u api.ChatItemUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.items == nil {
		s.items = map[string][]backend.ChatItem{}
	}
	s.heard(u.SessionID, u.Item)
	list := s.items[u.SessionID]
	if i := slices.IndexFunc(list, func(it backend.ChatItem) bool { return it.ID == u.Item.ID }); i >= 0 {
		list[i] = u.Item
		return
	}
	s.items[u.SessionID] = append(list, u.Item)
}

func (s *chatStore) turn(t api.ChatTurn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running == nil {
		s.running = map[string]bool{}
	}
	s.running[t.SessionID] = t.Running
	if t.Running {
		return
	}
	s.forgetSent(t.SessionID)
	// The event does not name its turn: it is the last one the conversation
	// holds, whose items were all heard before the end.
	list := s.items[t.SessionID]
	if len(list) == 0 {
		return
	}
	if s.ends == nil {
		s.ends = map[string]map[int]api.ChatTurn{}
	}
	if s.ends[t.SessionID] == nil {
		s.ends[t.SessionID] = map[int]api.ChatTurn{}
	}
	s.ends[t.SessionID][list[len(list)-1].Turn] = t
}

// endOf is how session id's turn ended, as the event said it, false for a turn
// read back with the history, whose end nobody reported to this window.
func (s *chatStore) endOf(id string, turn int) (api.ChatTurn, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.ends[id][turn]
	return t, ok
}

// setOptions takes session id's options whole, as ChatOptions or the options
// event left them.
func (s *chatStore) setOptions(id string, options []backend.ChatOption) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.options == nil {
		s.options = map[string][]backend.ChatOption{}
	}
	s.options[id] = slices.Clone(options)
}

// setCommands takes session id's commands whole, as ChatCommands or the
// commands event left them.
func (s *chatStore) setCommands(id string, commands []backend.ChatCommand) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.commands == nil {
		s.commands = map[string][]backend.ChatCommand{}
	}
	s.commands[id] = slices.Clone(commands)
}

// commandsOf is what session id's provider takes typed at its prompt.
func (s *chatStore) commandsOf(id string) []backend.ChatCommand {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.commands[id]
}

// optionOf is the option of session id in category that offers a choice,
// false when its provider offers none.
func (s *chatStore) optionOf(id, category string) (backend.ChatOption, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.options[id], func(o backend.ChatOption) bool { return o.Category == category && len(o.Choices) > 0 })
	if i < 0 {
		return backend.ChatOption{}, false
	}
	return s.options[id][i], true
}

// holds reports whether session id's conversation was read or heard.
func (s *chatStore) holds(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.items[id]
	return ok
}

func (s *chatStore) of(id string) []backend.ChatItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.withHeld(id, slices.Clone(s.items[id]))
}

// isRunning is whether session id's turn runs, a message waiting to go
// counting as its start.
func (s *chatStore) isRunning(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running[id] || s.held[id] != nil
}

// beginSwitch marks a switch of session id to view asked for, false when
// one already is.
func (s *chatStore) beginSwitch(id, view string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.switching[id] != "" {
		return false
	}
	if s.switching == nil {
		s.switching = map[string]string{}
	}
	s.switching[id] = view
	return true
}

// switchingTo is the view session id's switch in flight goes to, "" for none.
func (s *chatStore) switchingTo(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.switching[id]
}

func (s *chatStore) isSwitching(id string) bool { return s.switchingTo(id) != "" }

// switched ends session id's switch, landed or refused. The process it moved
// from is gone, and its turn with it.
func (s *chatStore) switched(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.switching, id)
	delete(s.running, id)
}

// chatEvent applies the chat events and a session's view landing.
func (m *model) chatEvent(ctx context.Context, ev backend.Event) {
	switch ev.Name {
	case api.ChatItemEvent.Name:
		if u, err := backend.Decode(ev, api.ChatItemEvent); err == nil {
			m.chat.apply(u)
		}
	case api.ChatTurnEvent.Name:
		if t, err := backend.Decode(ev, api.ChatTurnEvent); err == nil {
			m.chat.turn(t)
			m.flushQueue(ctx, t)
		}
	case api.ChatOptionsEvent.Name:
		if u, err := backend.Decode(ev, api.ChatOptionsEvent); err == nil {
			m.chat.setOptions(u.SessionID, u.Options)
		}
	case api.ChatCommandsEvent.Name:
		if u, err := backend.Decode(ev, api.ChatCommandsEvent); err == nil {
			m.chat.setCommands(u.SessionID, u.Commands)
		}
	case api.SessionViewEvent.Name:
		if v, err := backend.Decode(ev, api.SessionViewEvent); err == nil {
			m.applyView(ctx, v)
		}
	}
}

// applyView records session v.ID's new view. Leaving the chat, the window
// readies the terminal's resume before the store flips, so no frame starts
// it without one; reaching it, the conversation so far is read first.
func (m *model) applyView(ctx context.Context, v api.SessionView) {
	if v.View != api.ViewChat && m.toTerminal != nil {
		m.toTerminal(ctx, v.ID)
	}
	if v.View == api.ViewChat {
		m.loadChat(ctx, v.ID)
	}
	m.mu.Lock()
	if p, i := m.sessionAt(v.ID); p != nil {
		p.Sessions[i].View = v.View
	}
	m.mu.Unlock()
	m.chat.switched(v.ID)
	if v.View == api.ViewChat && m.toChat != nil {
		m.toChat(v.ID)
	}
}

// loadChats reads the conversation of every open session drawn as a chat
// the store does not hold yet: when the window attaches, and when a reload
// brings a session back (a reopen, a project opened). It runs with the event
// stream held behind it, as history needs.
func (m *model) loadChats(ctx context.Context) {
	m.mu.Lock()
	var ids []string
	for _, p := range m.projects {
		for _, s := range p.Sessions {
			if s.View == api.ViewChat && !m.chat.holds(s.ID) {
				ids = append(ids, s.ID)
			}
		}
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.loadChat(ctx, id)
	}
}

func (m *model) loadChat(ctx context.Context, id string) {
	if m.client == nil {
		return
	}
	items, err := m.client.ChatHistory(ctx, id)
	if err != nil {
		log.Printf("chat history of %s: %v", id, err)
		return
	}
	m.chat.history(id, items)
	options, err := m.client.ChatOptions(ctx, id)
	if err != nil {
		log.Printf("chat options of %s: %v", id, err)
		return
	}
	m.chat.setOptions(id, options)
	commands, err := m.client.ChatCommands(ctx, id)
	if err != nil {
		log.Printf("chat commands of %s: %v", id, err)
		return
	}
	m.chat.setCommands(id, commands)
}
