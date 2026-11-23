package spawn

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/relay"
	"github.com/lichdotdev/lich/internal/scope"
	"github.com/lichdotdev/lich/internal/store"
)

// The actions Control takes.
const (
	ControlPrompt  = "prompt"
	ControlAbort   = "abort"
	ControlModel   = "model"
	ControlEffort  = "effort"
	ControlCommand = "command"
)

// promptWait bounds the wait for a prompt's turn to start: an idle chat starts
// it at once, and past that the session is busy and the prompt waits behind
// the turn running.
const promptWait = 10 * time.Second

// ControlWaitLimit is the longest Control takes: a model a chat takes only by
// restarting resumes its conversation first. It stays under the 90 seconds an
// MCP call may block (cli.mcpMaxWait).
const ControlWaitLimit = 60 * time.Second

// How far a command Control handed a session got by the time it answered.
const (
	ControlDone      = "done"
	ControlDelivered = "delivered"
	ControlEnded     = "ended"
)

// defaultWritingCommands maps the slash commands that save their value as the
// default for every new session (measured on Claude Code 2.1.288's TUI) to the
// action that sets the same thing for this session only. Refused everywhere:
// whether a chat's own /model writes the default was not measured.
var defaultWritingCommands = map[string]string{"model": ControlModel, "effort": ControlEffort}

type Controlled = api.ControlledSession

type ControlOptions = api.ControlSessionOptions

// Control gives a session drawn as a chat one command, the way its person
// would in the chat: a prompt or a slash command as its next message, an abort
// as Stop, a model or an effort through its picker. A prompt waits a bounded
// time for its turn to start; one still queued behind a running turn is not an
// error, and State says so. A terminal session is refused on every provider:
// lich has no channel into a TUI but typing.
func (s *Service) Control(ctx context.Context, opts ControlOptions) (Controlled, error) {
	action, value, args := opts.Action, opts.Value, opts.Args
	text, err := controlText(action, value, args)
	if err != nil {
		return Controlled{}, err
	}
	found, err := s.chatTarget(opts.Target, opts.Project)
	if err != nil {
		return Controlled{}, err
	}
	label := found.session.Label
	if err := scope.Target(ctx, label, found.session.Sandbox == store.SessionConfined); err != nil {
		return Controlled{}, err
	}
	// model and effort are the exception: they only change what the session's
	// next requests carry, which the asking turn survives.
	if found.session.ID == opts.From && action != ControlModel && action != ControlEffort {
		return Controlled{}, fmt.Errorf(
			"%q is this session, and a session cannot control itself: an abort would end the "+
				"turn asking for it, and a prompt or a slash command would only run once that "+
				"turn is over", label)
	}
	ctx, cancel := context.WithTimeout(ctx, ControlWaitLimit)
	defer cancel()
	state, err := s.runControl(ctx, found.session.ID, label, action, value, text)
	if err != nil {
		return Controlled{}, err
	}
	return Controlled{
		ID: found.session.ID, Project: found.project.Name, Label: label,
		Action: action, Value: value, State: state,
	}, nil
}

func (s *Service) runControl(ctx context.Context, id, label, action, value, text string) (string, error) {
	var err error
	switch action {
	case ControlAbort:
		_, err = s.chat.Cancel(ctx, api.SessionIDOptions{ID: id})
	case ControlModel, ControlEffort:
		err = s.setChoice(ctx, id, label, action, value)
	default:
		return s.takePrompt(ctx, id, label, text)
	}
	if err != nil {
		return "", named(label, err)
	}
	return ControlDone, nil
}

// controlText validates an action and words what it sends as a message: the
// prompt, or the slash command with its arguments. Empty for the actions that
// send none.
func controlText(action, value, args string) (string, error) {
	if args != "" && action != ControlCommand {
		return "", fmt.Errorf("only a command takes arguments, and %s does not", action)
	}
	switch action {
	case ControlPrompt:
		if strings.TrimSpace(value) == "" {
			return "", errors.New("a prompt needs its text, and none was given")
		}
		return value, nil
	case ControlAbort:
		if value != "" {
			return "", errors.New("an abort takes nothing after it")
		}
		return "", nil
	case ControlModel, ControlEffort:
		if strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("%s needs the %s to set, and none was given", action, action)
		}
		return "", nil
	case ControlCommand:
		return slashCommand(value, args)
	}
	return "", fmt.Errorf(
		"%q is not something lich control does: the actions are prompt, abort, model, effort and command",
		action)
}

// slashCommand is the message a command sends, or why lich will not send it.
// The refusal reads the first word with every leading slash gone, so no
// spelling of /model or /effort gets through.
func slashCommand(name, args string) (string, error) {
	name = strings.TrimLeft(strings.TrimSpace(name), "/")
	if name == "" {
		return "", errors.New("a command needs the name of a slash command, and none was given")
	}
	first := strings.ToLower(strings.Fields(name)[0])
	if action, refused := defaultWritingCommands[first]; refused {
		return "", fmt.Errorf(
			"/%s is refused as a command: Claude Code saves what it sets as the default for every "+
				"new session. The %s action changes this session only", first, action)
	}
	return strings.TrimSpace("/" + name + " " + args), nil
}

// takePrompt sends text as the chat's next message and waits for the turn
// carrying it to start.
func (s *Service) takePrompt(ctx context.Context, id, label, text string) (string, error) {
	if err := s.chat.Open(ctx, id); err != nil {
		return "", named(label, err)
	}
	handed, err := s.chat.Take(id, text)
	if err != nil {
		return "", named(label, err)
	}
	wait, cancel := context.WithTimeout(ctx, promptWait)
	defer cancel()
	switch handed.Await(wait).State {
	case relay.PromptAcked:
		return ControlDone, nil
	case relay.PromptEnded:
		return ControlEnded, nil
	}
	return ControlDelivered, nil
}

// setChoice sets the chat's model or effort to value, one of the choices its
// picker offers.
func (s *Service) setChoice(ctx context.Context, id, label, action, value string) error {
	opts, err := s.chat.Options(ctx, api.SessionIDOptions{ID: id})
	if err != nil {
		return err
	}
	category := map[string]string{ControlModel: api.OptionModel, ControlEffort: api.OptionEffort}[action]
	i := slices.IndexFunc(opts, func(o api.ChatOption) bool { return o.Category == category })
	if i < 0 {
		return fmt.Errorf("%q has no %s to set: its agent offers none", label, action)
	}
	o := opts[i]
	ids := make([]string, 0, len(o.Choices))
	for _, ch := range o.Choices {
		ids = append(ids, ch.ID)
	}
	if !slices.Contains(ids, value) {
		return fmt.Errorf("%q offers no %s %q: it offers %s", label, action, value, strings.Join(ids, ", "))
	}
	_, err = s.chat.SetOption(ctx, api.ChatSetOptionOptions{ID: id, OptionID: o.ID, Value: value})
	return err
}

func named(label string, err error) error { return fmt.Errorf("%q: %w", label, err) }

// chatTarget finds the session target names, refusing one that does not run
// as a chat: lich drives a session through its chat, and a TUI has no channel
// in but typing.
func (s *Service) chatTarget(target, projectName string) (located, error) {
	if strings.TrimSpace(target) == "" {
		return located{}, errors.New("name the session to control")
	}
	projects, err := s.sessions.LoadState(context.Background(), api.None{})
	if err != nil {
		return located{}, fmt.Errorf("read the workspace: %w", err)
	}
	found, err := findSession(projects, s.term.AgentName, target, projectName)
	if err != nil {
		return located{}, err
	}
	label, kind := found.session.Label, found.session.Kind
	switch {
	case kind == providers.Crush:
		return located{}, fmt.Errorf(
			"%q runs %s, which lich cannot draw as a chat, so lich control cannot reach it", label, kindName(kind))
	case s.chat == nil || !s.chat.Drawn(found.session.ID):
		return located{}, fmt.Errorf(
			"%q runs in a terminal, and lich control drives only a session drawn as a chat: switch "+
				"it to GUI first", label)
	}
	return found, nil
}
