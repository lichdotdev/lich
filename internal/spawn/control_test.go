package spawn

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/relay"
	"github.com/lichdotdev/lich/internal/scope"
	"github.com/lichdotdev/lich/internal/store"
)

// newControlService is a workspace where the caller s1 and the target auth-fix
// run Claude Code as chats, codex-run runs Codex as a chat, term-run is a
// Claude Code session on its terminal and crush-run runs Crush, which has no
// chat.
func newControlService(t *testing.T) (*Service, *fakeChat) {
	t.Helper()
	svc, sessions, _, _, _ := newService(t)
	sessions.projects = []store.Project{{ID: "p1", Name: "lich", Path: "/src/lich", Sessions: []store.Session{
		{ID: "s1", Label: "Session 3", Kind: "claude"},
		{ID: "s2", Label: "auth-fix", Kind: "claude"},
		{ID: "s3", Label: "codex-run", Kind: "codex"},
		{ID: "s4", Label: "term-run", Kind: "claude"},
		{ID: "s5", Label: "crush-run", Kind: "crush"},
	}}}
	chat := &fakeChat{
		drawn:    true,
		terminal: map[string]bool{"s4": true, "s5": true},
		receipt:  relay.PromptReceipt{State: relay.PromptAcked},
		options: []api.ChatOption{
			{ID: "model", Category: api.OptionModel, Choices: []api.ChatChoice{{ID: "opus"}, {ID: "sonnet"}}},
			{ID: "effort", Category: api.OptionEffort, Choices: []api.ChatChoice{{ID: "high"}, {ID: "low"}}},
		},
	}
	svc.SetChat(chat)
	return svc, chat
}

// Every action reaches the session's chat the way the person's own would:
// a prompt or a slash command as its next message, an abort as Stop, a model
// or an effort through the chat's own picker.
func TestControlDrivesTheSessionsChat(t *testing.T) {
	tests := []struct {
		action, value, args string
		want                string
	}{
		{"prompt", "run the tests", "", "take s2 run the tests"},
		{"abort", "", "", "cancel s2"},
		{"model", "opus", "", "set s2 model=opus"},
		{"effort", "high", "", "set s2 effort=high"},
		{"command", "compact", "keep the plan", "take s2 /compact keep the plan"},
		{"command", "/clear", "", "take s2 /clear"},
	}
	for _, tc := range tests {
		t.Run(tc.action+" "+tc.value, func(t *testing.T) {
			svc, chat := newControlService(t)
			got, err := svc.Control(context.Background(), ControlOptions{
				From: "s1", Target: "auth-fix", Action: tc.action, Value: tc.value, Args: tc.args,
			})
			if err != nil {
				t.Fatalf("Control: %v", err)
			}
			if len(chat.calls) != 1 || chat.calls[0] != tc.want {
				t.Fatalf("chat got %q, want %q", chat.calls, tc.want)
			}
			want := Controlled{ID: "s2", Project: "lich", Label: "auth-fix", Action: tc.action,
				Value: tc.value, State: ControlDone}
			if got != want {
				t.Fatalf("Control = %+v, want %+v", got, want)
			}
		})
	}
}

// Control is the chat's, so a chat of another provider is driven the same way.
func TestControlDrivesAnotherProvidersChat(t *testing.T) {
	svc, chat := newControlService(t)
	if _, err := svc.Control(context.Background(), ControlOptions{From: "s1", Target: "codex-run", Action: "abort"}); err != nil {
		t.Fatalf("Control: %v", err)
	}
	if len(chat.calls) != 1 || chat.calls[0] != "cancel s3" {
		t.Fatalf("chat got %q, want the Codex chat's turn stopped", chat.calls)
	}
}

func TestControlRefusesBeforeReachingTheSession(t *testing.T) {
	tests := []struct {
		name, from, target, action, value, args, want string
	}{
		{"a terminal session", "s1", "term-run", "abort", "", "", `"term-run" runs in a terminal`},
		{"a provider with no chat", "s1", "crush-run", "abort", "", "", `"crush-run" runs Crush, which lich cannot draw as a chat`},
		{"the caller itself", "s1", "Session 3", "abort", "", "", "cannot control itself"},
		{"the caller itself, by its id", "s1", "s1", "abort", "", "", `"Session 3" is this session, and a session cannot control itself`},
		{"a value on an abort", "s1", "auth-fix", "abort", "now", "", "takes nothing after it"},
		{"args on a prompt", "s1", "auth-fix", "prompt", "go", "x", "only a command takes arguments"},
		{"an empty prompt", "s1", "auth-fix", "prompt", " ", "", "needs its text"},
		{"no model", "s1", "auth-fix", "model", "", "", "needs the model"},
		{"no effort", "s1", "auth-fix", "effort", " ", "", "needs the effort"},
		{"no command", "s1", "auth-fix", "command", "/", "", "needs the name of a slash command"},
		{"/model as a command", "s1", "auth-fix", "command", "/model", "opus", "The model action"},
		{"/effort as a command", "s1", "auth-fix", "command", "Effort", "", "The effort action"},
		{"an unknown action", "s1", "auth-fix", "compact", "", "", "the actions are prompt, abort"},
		{"no target", "s1", " ", "abort", "", "", "name the session to control"},
		{"a target nobody has", "s1", "nobody", "abort", "", "", "no session named"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, chat := newControlService(t)
			_, err := svc.Control(context.Background(), ControlOptions{
				From: tc.from, Target: tc.target, Action: tc.action, Value: tc.value, Args: tc.args,
			})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want one saying %q", err, tc.want)
			}
			if len(chat.calls) != 0 {
				t.Fatalf("the refusal still reached the chat: %q", chat.calls)
			}
		})
	}
}

// A model or an effort a chat does not offer is refused with the ones it
// does, before anything is set.
func TestControlNamesTheChoicesAChatOffers(t *testing.T) {
	svc, chat := newControlService(t)
	_, err := svc.Control(context.Background(), ControlOptions{From: "s1", Target: "auth-fix", Action: "model", Value: "gpt"})
	if err == nil || !strings.Contains(err.Error(), `"auth-fix" offers no model "gpt"`) || !strings.Contains(err.Error(), "opus, sonnet") {
		t.Fatalf("err = %v, want the model refused with the choices", err)
	}
	chat.options = chat.options[:1]
	_, err = svc.Control(context.Background(), ControlOptions{From: "s1", Target: "auth-fix", Action: "effort", Value: "high"})
	if err == nil || !strings.Contains(err.Error(), `"auth-fix" has no effort to set`) {
		t.Fatalf("err = %v, want a chat with no effort option refused", err)
	}
	if len(chat.calls) != 0 {
		t.Fatalf("a refused choice still reached the chat: %q", chat.calls)
	}
}

// A model or an effort only changes what the next requests carry, so the turn
// asking for it survives it.
func TestControlSetsTheCallersOwnModelAndEffort(t *testing.T) {
	for _, tc := range []struct{ action, value string }{{"model", "sonnet"}, {"effort", "high"}} {
		t.Run(tc.action, func(t *testing.T) {
			svc, chat := newControlService(t)
			_, err := svc.Control(context.Background(), ControlOptions{From: "s1", Target: "s1", Action: tc.action, Value: tc.value})
			if err != nil {
				t.Fatalf("Control: %v", err)
			}
			if len(chat.calls) != 1 || !strings.HasPrefix(chat.calls[0], "set s1 ") {
				t.Fatalf("chat got %q, want the caller's own option set", chat.calls)
			}
		})
	}
}

func TestControlReachesASessionByItsID(t *testing.T) {
	svc, chat := newControlService(t)
	got, err := svc.Control(context.Background(), ControlOptions{From: "s1", Target: "s2", Action: "abort"})
	if err != nil {
		t.Fatalf("Control: %v", err)
	}
	if got.Label != "auth-fix" || len(chat.calls) != 1 || chat.calls[0] != "cancel s2" {
		t.Fatalf("Control = %+v with %q, want auth-fix's turn stopped", got, chat.calls)
	}
}

// A prompt queued behind a running turn has been taken and still runs; one
// whose chat ended first never will.
func TestControlReportsHowFarThePromptGot(t *testing.T) {
	for _, tc := range []struct{ receipt, want string }{
		{relay.PromptDelivered, ControlDelivered},
		{relay.PromptEnded, ControlEnded},
	} {
		t.Run(tc.want, func(t *testing.T) {
			svc, chat := newControlService(t)
			chat.receipt = relay.PromptReceipt{State: tc.receipt}
			got, err := svc.Control(context.Background(), ControlOptions{From: "s1", Target: "auth-fix", Action: "prompt", Value: "go"})
			if err != nil {
				t.Fatalf("Control: %v", err)
			}
			if got.State != tc.want {
				t.Fatalf("Control = %+v, want state %s", got, tc.want)
			}
		})
	}
}

// The chat's own refusal reaches the caller naming the session: an abort with
// no turn running, an option the provider would not take, a prompt a chat that
// is not open could not take.
func TestControlPassesTheChatsRefusalOn(t *testing.T) {
	for _, action := range []string{"abort", "model", "prompt"} {
		t.Run(action, func(t *testing.T) {
			svc, chat := newControlService(t)
			chat.err = errors.New("no turn is running")
			value := map[string]string{"model": "opus", "prompt": "go"}[action]
			_, err := svc.Control(context.Background(), ControlOptions{From: "s1", Target: "auth-fix", Action: action, Value: value})
			if !errors.Is(err, chat.err) || !strings.HasPrefix(err.Error(), `"auth-fix": `) {
				t.Fatalf("err = %v, want the chat's, naming the session", err)
			}
		})
	}
}

// The wait on a prompt's turn is bounded under what an MCP call may block.
func TestControlBoundsTheWait(t *testing.T) {
	svc, chat := newControlService(t)
	if _, err := svc.Control(context.Background(), ControlOptions{From: "s1", Target: "auth-fix", Action: "prompt", Value: "go"}); err != nil {
		t.Fatalf("Control: %v", err)
	}
	if chat.deadline.IsZero() || time.Until(chat.deadline) > ControlWaitLimit {
		t.Fatalf("waited until %v, want a deadline within %v", chat.deadline, ControlWaitLimit)
	}
}

// A prompt handed to a session is carried out by that session, so a confined
// caller reaches only confined ones.
func TestAConfinedCallerSteersOnlyConfinedSessions(t *testing.T) {
	svc, chat := newControlService(t)
	scoped := scope.With(context.Background(), "s1")
	_, err := svc.Control(scoped, ControlOptions{From: "s1", Target: "auth-fix", Action: "prompt", Value: "run make"})
	if err == nil || !strings.Contains(err.Error(), `"auth-fix"`) || !strings.Contains(err.Error(), "sandbox") {
		t.Errorf("prompt to an unconfined session: err = %v, want a refusal naming it", err)
	}
	if len(chat.calls) != 0 {
		t.Errorf("the chat was handed %q", chat.calls)
	}
}

func TestAConfinedCallerSteersAConfinedSession(t *testing.T) {
	svc, chat := newControlService(t)
	svc.sessions.(*fakeSessions).projects[0].Sessions[1].Sandbox = store.SessionConfined
	_, err := svc.Control(scope.With(context.Background(), "s1"), ControlOptions{From: "s1", Target: "auth-fix", Action: "prompt", Value: "run make"})
	if err != nil {
		t.Fatalf("Control: %v", err)
	}
	if len(chat.calls) != 1 {
		t.Errorf("the chat was handed %q, want the prompt", chat.calls)
	}
}
