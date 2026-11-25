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
	"github.com/lichdotdev/lich/internal/relay"
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
		if got := chatArgs(a, ChatLaunch{}); !slices.Equal(got, tc.want) {
			t.Errorf("chatArgs(%s) = %v, want %v", tc.kind, got, tc.want)
		}
	}

	a := base
	a.kind, a.resume = providers.Kiro, "conv-1"
	if got := chatArgs(a, ChatLaunch{Model: "m2"}); !slices.Contains(got, "m2") || slices.Contains(got, "m1") {
		t.Errorf("kiro relaunch argv = %v, want the launch model on a resume", got)
	}
	if got := chatArgs(a, ChatLaunch{}); slices.Contains(got, "--model") {
		t.Errorf("kiro resume argv = %v, want no birth model", got)
	}

	a.kind = providers.Claude
	if got := chatArgs(a, ChatLaunch{AllowBypass: true}); !slices.Contains(got, claudeAllowBypassFlag) {
		t.Errorf("claude relaunch argv = %v, want %s", got, claudeAllowBypassFlag)
	}
	got := chatArgs(a, ChatLaunch{})
	if !slices.Equal(got[:len(claudeChatArgs)], claudeChatArgs) {
		t.Errorf("claude chat argv = %v, want the stream-json flags first", got)
	}
	if rest := got[len(claudeChatArgs):]; !slices.Equal(rest[:2], []string{"--resume", "conv-1"}) {
		t.Errorf("claude chat argv = %v, want its terminal argv after the flags", got)
	}
}

// TestCodexChatArgsAreGlobalOverridesThenAppServer: Codex's chat takes the
// terminal's `-c` overrides (trust, briefing, lich's MCP server) before the
// subcommand, and no flag app-server lacks.
func TestCodexChatArgsAreGlobalOverridesThenAppServer(t *testing.T) {
	t.Parallel()
	a := spawnArgs{kind: providers.Codex, cwd: "/w", model: "m1", effort: "high", skipPermissions: true, confined: true, lichBin: "/bin/lich", lang: prompt.English}
	got := chatArgs(a, ChatLaunch{})
	if got[len(got)-1] != codexAppServerSubcmd || slices.Contains(got, "--model") || slices.Contains(got, skipPermissionFlags[providers.Codex]) {
		t.Errorf("codex chat argv = %v", got)
	}
	joined := strings.Join(got, " ")
	for _, want := range []string{`trust_level="trusted"`, codexInstructionsKey + "=", "mcp_servers." + relay.MCPServerName + ".command"} {
		if !strings.Contains(joined, want) {
			t.Errorf("codex chat argv = %v, want %s", got, want)
		}
	}
	a.resume = "conv-1"
	if got := strings.Join(chatArgs(a, ChatLaunch{}), " "); strings.Contains(got, codexInstructionsKey) || strings.Contains(got, "conv-1") {
		t.Errorf("codex resume argv = %v, want neither the briefing nor the id, which the protocol carries", got)
	}
}

// TestChatBirthCarriesWhatTheArgvCannot: the model and effort reach the
// protocol only where no flag carried them, and only at birth; skip-permissions
// becomes lich answering yes on every provider, whatever its argv carried.
func TestChatBirthCarriesWhatTheArgvCannot(t *testing.T) {
	t.Parallel()
	svc := New(stubBins{model: "m1", effort: "low", skipPerms: true}, nil, events.New())
	cases := []struct {
		kind, resume string
		want         ChatBirth
	}{
		{providers.OMP, "", ChatBirth{Model: "m1", Effort: "low", AutoApprove: true}},
		{providers.OMP, "conv", ChatBirth{AutoApprove: true}},
		{providers.Antigravity, "", ChatBirth{Model: "m1", Effort: "low", AutoApprove: true}},
		{providers.Codex, "", ChatBirth{Model: "m1", Effort: "low", AutoApprove: true}},
		{providers.Claude, "", ChatBirth{AutoApprove: true}},
		{providers.Kiro, "", ChatBirth{AutoApprove: true}},
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
	p, err := svc.StartChat(t.Context(), api.StartOptions{ID: "s1", ProjectID: "p1", Cwd: t.TempDir(), Kind: providers.OMP}, ChatLaunch{})
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

	p, err := svc.StartChat(t.Context(), api.StartOptions{ID: "s1", Cwd: t.TempDir(), Kind: providers.OMP}, ChatLaunch{})
	if err != nil {
		t.Fatalf("StartChat: %v", err)
	}
	t.Cleanup(p.Close)
	if _, err := svc.Start(t.Context(), api.StartOptions{ID: "s1", Kind: providers.OMP}); !errors.Is(err, ErrChatView) {
		t.Errorf("Start beside a running chat = %v, want ErrChatView", err)
	}

	svc.sessions["s2"] = spawnSession(t)
	if _, err := svc.StartChat(t.Context(), api.StartOptions{ID: "s2", Cwd: t.TempDir(), Kind: providers.OMP}, ChatLaunch{}); err == nil {
		t.Error("StartChat beside a running terminal succeeded")
	}
	if _, err := svc.StartChat(t.Context(), api.StartOptions{ID: "s3", Kind: KindShell}, ChatLaunch{}); err == nil {
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

// TestOpenCodeChatRunsItsServerOnLichsPort: an opencode 1 chat's server
// listens where the chat looks for its questions; opencode 2, unmeasured, and
// every other provider run as before.
func TestOpenCodeChatRunsItsServerOnLichsPort(t *testing.T) {
	t.Parallel()
	l := ChatLaunch{Server: &OpenCodeServer{Port: 41234, Password: "p"}}
	a := spawnArgs{kind: providers.OpenCode}
	if got := chatArgs(a, l); !slices.Equal(got, []string{"acp", "--port", "41234"}) {
		t.Errorf("opencode chat argv = %v", got)
	}
	a.oc2 = true
	if got := chatArgs(a, l); !slices.Equal(got, []string{"acp"}) {
		t.Errorf("opencode 2 chat argv = %v, want it unchanged", got)
	}
	if openCodeServer(providers.OMP, false, l) != nil {
		t.Error("another provider was given opencode's server")
	}
}

// todoEnvBin is a provider that prints what it was told of its task tools,
// then waits for its stdin to close.
func todoEnvBin(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-claude")
	script := "#!/bin/sh\necho \"todo=${" + claudeTodoToolsVar + "-unset}\"\nwhile read line; do :; done\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub bin: %v", err)
	}
	return path
}

// A Claude Code spawn turns its task tools on, and a value the person exports
// stays as they set it.
func TestClaudeSpawnTurnsTheTaskToolsOnUnlessThePersonSaidSo(t *testing.T) {
	cases := []struct {
		name string
		env  []string
		want string
	}{
		{"unset", []string{"PATH=" + os.Getenv("PATH")}, "todo=1\n"},
		{"the person's own", []string{"PATH=" + os.Getenv("PATH"), claudeTodoToolsVar + "=0"}, "todo=0\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := New(stubBins{bin: todoEnvBin(t)}, nil, events.New())
			svc.SetEnv(c.env)
			p, err := svc.StartChat(t.Context(), api.StartOptions{ID: "s1", ProjectID: "p1", Cwd: t.TempDir(), Kind: providers.Claude}, ChatLaunch{})
			if err != nil {
				t.Fatalf("StartChat: %v", err)
			}
			defer p.Close()
			if line, err := bufio.NewReader(p.Stdout).ReadString('\n'); err != nil || line != c.want {
				t.Errorf("the spawn read %q (%v), want %q", line, err, c.want)
			}
		})
	}
}

func TestTodoToolsEnvIsClaudeCodesAlone(t *testing.T) {
	if got := todoToolsEnv([]string{"A=1"}, providers.Codex); !slices.Equal(got, []string{"A=1"}) {
		t.Errorf("todoToolsEnv(codex) = %v, want the env untouched", got)
	}
}
