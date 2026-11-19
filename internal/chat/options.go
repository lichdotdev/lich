package chat

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/terminal"
)

// errRelaunch is a driver saying its protocol cannot make a change in flight:
// only a process started for it can (terminal.ChatLaunch).
var errRelaunch = errors.New("the agent takes this setting only when it starts")

// carryOrder is the order carried settings are applied in: a Claude Code
// model decides which efforts and modes it offers.
var carryOrder = []string{api.OptionModel, api.OptionEffort, api.OptionMode}

// claudeBypass is Claude Code's mode that runs every tool without asking.
const claudeBypass = "bypassPermissions"

// Options answers api.ChatOptions.
func (s *Service) Options(ctx context.Context, opts api.SessionIDOptions) ([]api.ChatOption, error) {
	c, err := s.ensure(ctx, opts.ID)
	if err != nil {
		return nil, err
	}
	return c.options(), nil
}

// SetOption answers api.ChatSetOption: over the protocol when the provider
// takes the change in flight, by resuming the conversation in a process
// started for it when not. The choice is recorded for the session's later
// spawns.
func (s *Service) SetOption(ctx context.Context, opts api.ChatSetOptionOptions) (api.None, error) {
	c, err := s.ensure(ctx, opts.ID)
	if err != nil {
		return api.None{}, err
	}
	o, err := c.choice(opts.OptionID, opts.Value)
	if err != nil {
		return api.None{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	err = c.drv.setOption(callCtx, o, opts.Value)
	cancel()
	if errors.Is(err, errRelaunch) {
		err = s.relaunch(ctx, c, o.Category, opts.Value)
	}
	if err != nil {
		return api.None{}, err
	}
	return api.None{}, s.remember(opts.ID, o.Category, opts.Value)
}

// relaunch resumes c's conversation in a new process started for category set
// to value, carrying every other setting c ran on, the way a switch does: one
// process at a time, refused while a turn runs. The window hears of it as a
// view event, which is its cue to read the history again.
func (s *Service) relaunch(ctx context.Context, c *chatSession, category, value string) error {
	release, err := s.claim(c.id)
	if err != nil {
		return err
	}
	defer release()
	if c.turnRunning() || s.term.TurnOpen(c.id) {
		return errBusy
	}
	carry := c.currents()
	carry[category] = value
	c.stop()
	if _, err := s.start(ctx, c.id, c.projectID, c.kind, c.cwd, carry); err != nil {
		return err
	}
	events.Send(s.hub.Emit, api.SessionViewEvent, api.SessionView{ID: c.id, View: api.ViewChat})
	return nil
}

// remember records a choice where the session's later spawns read it.
func (s *Service) remember(id, category, value string) error {
	switch category {
	case api.OptionModel:
		return s.store.SetSessionModel(id, value)
	case api.OptionEffort:
		return s.store.SetSessionEffort(id, value)
	case api.OptionMode:
		return s.store.SetSessionChatMode(id, value)
	}
	return nil
}

// storedCarry is what a chat that is not a relaunch opens on: the mode the
// person last picked in it, which Claude Code would otherwise resume without.
// The model and effort are the provider's to bring back with the conversation.
func (s *Service) storedCarry(id string) map[string]string {
	return map[string]string{api.OptionMode: s.store.SessionChatMode(id)}
}

// launchFor is what a chat of kind opening on carry needs on its argv.
func launchFor(kind string, carry map[string]string) terminal.ChatLaunch {
	switch kind {
	case providers.Claude:
		return terminal.ChatLaunch{AllowBypass: carry[api.OptionMode] == claudeBypass}
	case providers.Kiro:
		return terminal.ChatLaunch{Model: carry[api.OptionModel]}
	}
	return terminal.ChatLaunch{}
}

// applyCarry sets what the chat should run on once it is open, past what the
// argv already carried: a setting that took, or that only a launch can take,
// is left alone.
func (c *chatSession) applyCarry(ctx context.Context, carry map[string]string) {
	for _, category := range carryOrder {
		value := carry[category]
		o, ok := c.optionOf(category)
		if value == "" || !ok || o.Current == value || !hasChoice(o, value) {
			continue
		}
		if err := c.drv.setOption(ctx, o, value); err != nil && !errors.Is(err, errRelaunch) {
			c.conv.notice(fmt.Sprintf("The %s %s was not set again: %v", category, value, err))
		}
	}
}

// setOptions records what the chat can be set to now and tells the window
// when it changed.
func (c *chatSession) setOptions(opts []api.ChatOption) {
	c.mu.Lock()
	same := reflect.DeepEqual(c.opts, opts)
	c.opts = opts
	c.mu.Unlock()
	if !same {
		events.Send(c.svc.hub.Emit, api.ChatOptionsEvent, api.ChatOptionsUpdate{SessionID: c.id, Options: opts})
	}
}

func (c *chatSession) options() []api.ChatOption {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.opts)
}

func (c *chatSession) optionOf(category string) (api.ChatOption, bool) {
	for _, o := range c.options() {
		if o.Category == category {
			return o, true
		}
	}
	return api.ChatOption{}, false
}

// currents is the choice in force of every option, by category.
func (c *chatSession) currents() map[string]string {
	carry := map[string]string{}
	for _, o := range c.options() {
		carry[o.Category] = o.Current
	}
	return carry
}

// choice is option id, checked to offer value.
func (c *chatSession) choice(id, value string) (api.ChatOption, error) {
	for _, o := range c.options() {
		if o.ID != id {
			continue
		}
		if !hasChoice(o, value) {
			return o, fmt.Errorf("%q is not a choice of %q", value, id)
		}
		return o, nil
	}
	return api.ChatOption{}, fmt.Errorf("this chat has no option %q", id)
}

func hasChoice(o api.ChatOption, value string) bool {
	return slices.ContainsFunc(o.Choices, func(ch api.ChatChoice) bool { return ch.ID == value })
}
