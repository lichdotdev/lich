//go:build !windows

package terminal

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/prompt"
	"github.com/lichdotdev/lich/internal/providers"
)

// TestChatArgsPassOnlyWhatEachModeTakes pins the chat argv per provider: Claude
// Code keeps its whole terminal argv behind the stream-json flags, Kiro and
// Antigravity take the flags their own `acp --help` lists, and the rest take
// none of lich's, which would kill the spawn.
func TestChatArgsPassOnlyWhatEachModeTakes(t *testing.T) {
	t.Parallel()
	base := spawnArgs{model: "m1", effort: "high", skipPermissions: true, agent: "lich", lang: prompt.English}
	cases := []struct {
		kind string
		want []string
	}{
		{providers.Kiro, []string{"acp", "--agent", "lich", "--trust-all-tools", "--model", "m1", "--effort", "high"}},
		{providers.Antigravity, []string{"acp", "--dangerously-skip-permissions"}},
		{providers.OpenCode, []string{"acp"}},
		{providers.OMP, []string{"acp"}},
		{providers.Cursor, []string{"acp"}},
	}
	for _, tc := range cases {
		a := base
		a.kind = tc.kind
		if got := chatArgs(a); !slices.Equal(got, tc.want) {
			t.Errorf("chatArgs(%s) = %v, want %v", tc.kind, got, tc.want)
		}
	}

	a := base
	a.kind, a.resume = providers.Claude, "conv-1"
	got := chatArgs(a)
	if !slices.Equal(got[:len(claudeChatArgs)], claudeChatArgs) {
		t.Errorf("claude chat argv = %v, want the stream-json flags first", got)
	}
	if rest := got[len(claudeChatArgs):]; !slices.Equal(rest[:2], []string{"--resume", "conv-1"}) {
		t.Errorf("claude chat argv = %v, want its terminal argv after the flags", got)
	}
}

// TestChatBirthCarriesWhatTheArgvCannot: the model and effort reach the
// protocol only where no flag carried them, and only at birth; skip-permissions
// becomes lich answering yes where the ACP server has no flag for it.
func TestChatBirthCarriesWhatTheArgvCannot(t *testing.T) {
	t.Parallel()
	svc := New(stubBins{model: "m1", effort: "low", skipPerms: true}, nil, events.New())
	cases := []struct {
		kind, resume string
		want         ChatBirth
	}{
		{providers.OMP, "", ChatBirth{Model: "m1", Effort: "low", AutoApprove: true}},
		{providers.OMP, "conv", ChatBirth{AutoApprove: true}},
		{providers.Antigravity, "", ChatBirth{Model: "m1", Effort: "low"}},
		{providers.Claude, "", ChatBirth{}},
		{providers.Kiro, "", ChatBirth{}},
	}
	for _, tc := range cases {
		if got := svc.chatBirth(api.StartOptions{ID: "s1", Kind: tc.kind, Resume: tc.resume}); got != tc.want {
			t.Errorf("chatBirth(%s, resume %q) = %+v, want %+v", tc.kind, tc.resume, got, tc.want)
		}
	}
}

// chatEchoBin is a provider that answers every line it reads with the same line,
// prefixed, and leaves when its stdin closes.
func chatEchoBin(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-agent")
	script := "#!/bin/sh\necho started $LICH_SESSION_ID >&2\nwhile read line; do echo \"got $line\"; done\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub bin: %v", err)
	}
	return path
}

// TestStartChatRunsTheProviderOnPipes drives a chat process end to end: what
// is written reaches it, what it prints comes back, it is registered as a chat
// for as long as it runs, and Close ends it.
func TestStartChatRunsTheProviderOnPipes(t *testing.T) {
	svc := New(stubBins{bin: chatEchoBin(t)}, nil, events.New())
	p, err := svc.StartChat(t.Context(), api.StartOptions{ID: "s1", ProjectID: "p1", Cwd: t.TempDir(), Kind: providers.OMP})
	if err != nil {
		t.Fatalf("StartChat: %v", err)
	}
	if !svc.spawnOf("s1").chat {
		t.Error("a running chat is not registered as one")
	}
	if _, err := fmt.Fprintln(p.Stdin, "hello"); err != nil {
		t.Fatalf("write: %v", err)
	}
	line, err := bufio.NewReader(p.Stdout).ReadString('\n')
	if err != nil || line != "got hello\n" {
		t.Fatalf("read = %q, %v; want the echo", line, err)
	}
	p.Close()
	select {
	case <-p.Done():
	default:
		t.Fatal("Close returned with the process still running")
	}
	if !strings.Contains(p.Stderr(), "started") {
		t.Errorf("stderr tail = %q, want what the process wrote", p.Stderr())
	}
	deadline := time.Now().Add(2 * time.Second)
	for svc.spawnOf("s1").chat && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if svc.spawnOf("s1").chat {
		t.Error("an exited chat is still registered")
	}
}

// TestChatAndTerminalNeverRunTogether: neither side spawns while the other
// runs, and a session drawn as a chat refuses its terminal outright.
func TestChatAndTerminalNeverRunTogether(t *testing.T) {
	svc := New(stubBins{bin: chatEchoBin(t), views: map[string]string{"chat-row": api.ViewChat}}, nil, events.New())
	if _, err := svc.Start(t.Context(), api.StartOptions{ID: "chat-row", Kind: providers.Claude}); !errors.Is(err, ErrChatView) {
		t.Errorf("Start of a chat-view session = %v, want ErrChatView", err)
	}

	p, err := svc.StartChat(t.Context(), api.StartOptions{ID: "s1", Cwd: t.TempDir(), Kind: providers.OMP})
	if err != nil {
		t.Fatalf("StartChat: %v", err)
	}
	t.Cleanup(p.Close)
	if _, err := svc.Start(t.Context(), api.StartOptions{ID: "s1", Kind: providers.OMP}); !errors.Is(err, ErrChatView) {
		t.Errorf("Start beside a running chat = %v, want ErrChatView", err)
	}

	svc.sessions["s2"] = spawnSession(t)
	if _, err := svc.StartChat(t.Context(), api.StartOptions{ID: "s2", Cwd: t.TempDir(), Kind: providers.OMP}); err == nil {
		t.Error("StartChat beside a running terminal succeeded")
	}
	if _, err := svc.StartChat(t.Context(), api.StartOptions{ID: "s3", Kind: KindShell}); err == nil {
		t.Error("StartChat of a shell succeeded")
	}
}

// TestStopTerminalWaitsForTheChild: the switch to a chat must not spawn while
// the terminal's process is still alive.
func TestStopTerminalWaitsForTheChild(t *testing.T) {
	svc := New(stubBins{bin: stayAliveBin(t)}, nil, events.New())
	if _, err := svc.Start(t.Context(), api.StartOptions{ID: "s1", Cwd: t.TempDir(), Kind: providers.Claude, Cols: 80, Rows: 24}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	sess := svc.sessionOf("s1")
	if err := svc.StopTerminal("s1"); err != nil {
		t.Fatalf("StopTerminal: %v", err)
	}
	select {
	case <-sess.reaped:
	default:
		t.Fatal("StopTerminal returned before the child was reaped")
	}
	if svc.live("s1") {
		t.Error("the session still runs a terminal")
	}
	if err := svc.StopTerminal("s1"); err != nil {
		t.Errorf("StopTerminal with nothing running = %v, want nil", err)
	}
}

// TestAChatReportsItsOwnState: the chat's report reaches the card like a
// hook's does, and the hooks its provider fires meanwhile are dropped, so the
// ring follows one source.
func TestAChatReportsItsOwnState(t *testing.T) {
	t.Parallel()
	svc := New(stubBins{}, nil, events.New())
	svc.spawns.Store("s1", spawn{kind: providers.Claude, chat: true})

	svc.onHookState(hookRequest{SessionID: "s1", State: statusBusy})
	if got := svc.Status("s1").State; got != "" {
		t.Errorf("a hook of a chat session published %q", got)
	}
	svc.ReportChatState("s1", statusBusy, "Read", "a.txt")
	if got := svc.Status("s1"); got.State != statusBusy || got.Tool != "Read" {
		t.Errorf("status = %+v, want the chat's busy", got)
	}
	if !svc.TurnOpen("s1") {
		t.Error("a busy chat has no turn open")
	}
	svc.ReportChatState("s1", statusDone, "", "")
	if svc.TurnOpen("s1") {
		t.Error("a done chat still has its turn open")
	}
}

func TestTailBufferKeepsTheEnd(t *testing.T) {
	t.Parallel()
	b := &tailBuffer{max: 4}
	_, _ = b.Write([]byte("abc"))
	_, _ = b.Write([]byte("def"))
	if got := b.String(); got != "cdef" {
		t.Errorf("tail = %q, want %q", got, "cdef")
	}
}
