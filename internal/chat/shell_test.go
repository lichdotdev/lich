//go:build !windows

package chat

import (
	"errors"
	"strings"
	"testing"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/providers"
)

func (r *rig) shell(command string) error {
	_, err := r.svc.RunShell(r.t.Context(), api.ChatRunShellOptions{ID: "s1", Command: command})
	return err
}

// shellCall is the tool item of the shell command that opened turn.
func (r *rig) shellCall(turn int) api.ChatItem {
	r.t.Helper()
	var got api.ChatItem
	r.eventually("the shell command's call", func() bool {
		for _, it := range r.history() {
			if it.Kind == api.ItemTool && it.Turn == turn && it.Tool != nil && (it.Tool.State == api.ToolDone || it.Tool.State == api.ToolFailed) {
				got = it
				return true
			}
		}
		return false
	})
	return got
}

// TestClaudeShellRunsBesideTheConversation: a command goes as stream-json's
// bash_command, the person's message keeps the "!" they typed, its output
// is a Bash call under it, and the row Claude Code echoes is no second
// message.
func TestClaudeShellRunsBesideTheConversation(t *testing.T) {
	r := newChat(t, providers.Claude)
	if err := r.shell("ls a&b"); err != nil {
		t.Fatalf("Shell: %v", err)
	}
	if got := r.turnEnd(0); got.StopReason != stopEndTurn {
		t.Errorf("turn ended %q, want %q", got.StopReason, stopEndTurn)
	}
	if !strings.Contains(r.fakeLog(), `"type":"bash_command"`) {
		t.Errorf("fake log = %q, want a bash_command", r.fakeLog())
	}
	if got := strings.Join(r.texts(api.ItemUser), "|"); got != "!ls a&b" {
		t.Errorf("messages %q, want the command once, as typed", got)
	}
	call := r.shellCall(1)
	if call.Tool.Name != claudeShellTool || call.Tool.Title != "ls a&b" || call.Tool.Output != "ran ls a&b" || call.Tool.State != api.ToolDone {
		t.Errorf("call = %+v", call.Tool)
	}

	if err := r.shell("fail"); err != nil {
		t.Fatalf("Shell(fail): %v", err)
	}
	r.turnEnd(1)
	if call := r.shellCall(2); call.Tool.State != api.ToolFailed || call.Tool.Output != "x > y" {
		t.Errorf("a command that exits 4 is %+v, want failed with its stderr", call.Tool)
	}
}

// TestClaudeShellStoppedIsCancelled: Stop interrupts the command, which
// Claude Code ends with [interrupted], and the turn ends cancelled.
func TestClaudeShellStoppedIsCancelled(t *testing.T) {
	r := newChat(t, providers.Claude)
	if err := r.shell("sleep"); err != nil {
		t.Fatalf("Shell: %v", err)
	}
	if _, err := r.svc.Cancel(t.Context(), api.SessionIDOptions{ID: "s1"}); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if got := r.turnEnd(0); got.StopReason != stopCancelled {
		t.Errorf("turn ended %q, want %q", got.StopReason, stopCancelled)
	}
	if call := r.shellCall(1); call.Tool.State != api.ToolFailed || call.Tool.Output != "[interrupted]" {
		t.Errorf("call = %+v", call.Tool)
	}
}

// TestClaudeShellNotRunIsAFailedCall: a command Claude Code refuses says
// why in its call.
func TestClaudeShellNotRunIsAFailedCall(t *testing.T) {
	r := newChat(t, providers.Claude)
	if err := r.shell(" "); err != nil {
		t.Fatalf("Shell: %v", err)
	}
	r.turnEnd(0)
	if call := r.shellCall(1); call.Tool.State != api.ToolFailed || call.Tool.Output != "Not run: no command was given." {
		t.Errorf("call = %+v", call.Tool)
	}
}

// TestClaudeShellOfTheTranscript: a command the person ran with the TUI's
// `!` comes back as their message, unescaped, and its output as a call.
func TestClaudeShellOfTheTranscript(t *testing.T) {
	r := newRig(t, providers.Claude)
	_ = r.db.SetProviderSession("s1", "conv-1")
	r.writeClaudeTranscript("conv-1",
		`{"type":"user","message":{"content":"<bash-input>grep a &amp;&amp; echo &lt;b&gt;</bash-input>"}}`,
		`{"type":"user","message":{"content":"<bash-stdout>a\n&lt;b&gt;</bash-stdout><bash-stderr></bash-stderr>"}}`,
		`{"type":"user","message":{"content":"asked"}}`,
	)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	if got, want := strings.Join(r.texts(api.ItemUser), "|"), "!grep a && echo <b>|asked"; got != want {
		t.Errorf("messages %q, want %q", got, want)
	}
	if call := r.shellCall(1); call.Tool.Title != "grep a && echo <b>" || call.Tool.Output != "a\n<b>" || call.Tool.State != api.ToolDone {
		t.Errorf("call = %+v", call.Tool)
	}
}

// TestCodexShellIsItsOwnTurn: Codex runs the command in a turn it starts
// and ends, and the commandExecution it reports is the call.
func TestCodexShellIsItsOwnTurn(t *testing.T) {
	r := newChat(t, providers.Codex)
	if err := r.shell("ls"); err != nil {
		t.Fatalf("Shell: %v", err)
	}
	if got := r.turnEnd(0); got.StopReason != stopEndTurn {
		t.Errorf("turn ended %q", got.StopReason)
	}
	if !strings.Contains(r.fakeLog(), `"method":"thread/shellCommand","params":{"command":"ls","threadId":"codex-new"}`) {
		t.Errorf("fake log = %q", r.fakeLog())
	}
	if call := r.shellCall(1); call.Tool.Output != "ran ls\n" {
		t.Errorf("call = %+v", call.Tool)
	}
	if got := strings.Join(r.texts(api.ItemUser), "|"); got != "!ls" {
		t.Errorf("messages %q", got)
	}
}

// TestOpenCodeShellRunsUnderTheAgentInForce: the command goes to opencode's
// server under the mode's agent, and the tool call it reports over ACP is the
// call; the server's answer ends the turn.
func TestOpenCodeShellRunsUnderTheAgentInForce(t *testing.T) {
	r := newChat(t, providers.OpenCode)
	mode, ok := r.option(api.OptionMode)
	if !ok || mode.Current == "" {
		t.Fatalf("mode = %+v", mode)
	}
	if err := r.shell("ls"); err != nil {
		t.Fatalf("Shell: %v", err)
	}
	if got := r.turnEnd(0); got.StopReason != stopEndTurn {
		t.Errorf("turn ended %q", got.StopReason)
	}
	if call := r.shellCall(1); call.Tool.Title != "ls" || call.Tool.Output != "ran as "+mode.Current {
		t.Errorf("call = %+v", call.Tool)
	}
}

// TestShellIsTextWhereTheProtocolHasNone: an ACP agent other than
// opencode 1 runs no command of the person's, and gets the line as typed.
func TestShellIsTextWhereTheProtocolHasNone(t *testing.T) {
	for _, kind := range []string{providers.OMP, providers.Antigravity, providers.Kiro} {
		r := newChat(t, kind)
		if err := r.shell("ls"); err != nil {
			t.Errorf("%s: Shell = %v", kind, err)
		}
		r.turnEnd(0)
		if got := strings.Join(r.texts(api.ItemUser), "|"); got != "!ls" {
			t.Errorf("%s: messages %q, want the line as typed", kind, got)
		}
		if !strings.Contains(r.fakeLog(), `"session/prompt"`) || !strings.Contains(r.fakeLog(), `"text":"!ls"`) {
			t.Errorf("%s: fake log = %q, want the line as a prompt", kind, r.fakeLog())
		}
	}
}

// TestACPReplayLeavesOutWhatIsForTheAgentAlone: the user message opencode
// files a shell command under is the agent's, not the person's.
func TestACPReplayLeavesOutWhatIsForTheAgentAlone(t *testing.T) {
	r := newRig(t, providers.OMP)
	_ = r.db.SetProviderSession("s1", "omp-shelled")
	r.writeOMPSession("omp-shelled")
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	r.mustItem(api.ItemUser, "and now?")
	if got := strings.Join(r.texts(api.ItemUser), "|"); got != "and now?" {
		t.Errorf("messages %q, want the person's alone", got)
	}
}

// TestShellWaitsForTheTurn: a command sent while a turn runs is refused,
// as a second command is while one runs.
func TestShellWaitsForTheTurn(t *testing.T) {
	r := newChat(t, providers.Claude)
	if err := r.shell("sleep"); err != nil {
		t.Fatalf("Shell: %v", err)
	}
	if err := r.shell("ls"); !errors.Is(err, errTurnRunning) {
		t.Errorf("Shell during a turn = %v, want %v", err, errTurnRunning)
	}
	_, _ = r.svc.Cancel(t.Context(), api.SessionIDOptions{ID: "s1"})
	r.turnEnd(0)
}

// TestClaudeShellKeepsTheOutputsIndent: the first line's indent stays, as
// its columns line up with the lines under it.
func TestClaudeShellKeepsTheOutputsIndent(t *testing.T) {
	r := newRig(t, providers.Claude)
	_ = r.db.SetProviderSession("s1", "conv-1")
	r.writeClaudeTranscript("conv-1",
		`{"type":"user","message":{"content":"<bash-input>git status -s</bash-input>"}}`,
		`{"type":"user","message":{"content":"<bash-stdout> M a.go\n?? b.go\n</bash-stdout><bash-stderr></bash-stderr>"}}`,
	)
	if err := r.setView(api.ViewChat); err != nil {
		t.Fatalf("SetView: %v", err)
	}
	if call := r.shellCall(1); call.Tool.Output != " M a.go\n?? b.go" {
		t.Errorf("output %q, want its lines as printed", call.Tool.Output)
	}
}
