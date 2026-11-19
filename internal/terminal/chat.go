package terminal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"sync"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/winexec"
)

// A session drawn as a chat (internal/chat) runs its provider in the provider's
// structured mode instead of its TUI: the same binary, environment, profile,
// sandbox and tokens as the terminal (prepareSpawnLocked), on pipes instead of
// a PTY. Only one of the two runs a conversation at a time: the spike that
// measured these modes found no lock between them, and two processes on one
// conversation write one transcript from two histories.

// How Claude Code is run as a chat: stream-json both ways, every message on
// stdout (--verbose is required with stream-json output), text as it streams,
// and its permission questions asked over the same stdio (measured on
// 2.1.327). They go first because --mcp-config, at the end of providerArgs, is
// variadic.
var claudeChatArgs = []string{
	"-p",
	"--input-format", "stream-json",
	"--output-format", "stream-json",
	"--verbose",
	"--include-partial-messages",
	"--permission-prompt-tool", "stdio",
}

// acpSubcmd starts the Agent Client Protocol server of every provider whose
// chat speaks it.
const acpSubcmd = "acp"

// chatCloseGrace is how long a chat process is given to leave once its stdin
// closes, which is how both protocols are told to end, before it is killed.
const chatCloseGrace = 3 * time.Second

// chatDrain is how long the reader is left to empty the pipe after the process
// exited before the pipe is closed under it: a child the provider started can
// hold the write end open long after the provider is gone.
const chatDrain = 500 * time.Millisecond

// stopTerminalWait bounds StopTerminal's wait for a PTY child to be reaped.
const stopTerminalWait = 5 * time.Second

// ErrChatView refuses a terminal spawn for a session drawn as a chat.
var ErrChatView = errors.New("this session is drawn as a chat; switch it to the terminal first")

// chatArgs is the argv of a session's chat. Each ACP server takes far fewer
// flags than its TUI, so only the ones its own `acp --help` lists are passed
// (measured on Kiro 2.21.0 and Antigravity 1.5.3; opencode, oh-my-pi and Cursor
// take none of lich's): an unknown flag kills the spawn before the session
// exists. What a flag cannot carry, the protocol sets (chatBirth). l is what a
// chat restarted for a setting its protocol cannot change carries.
func chatArgs(a spawnArgs, l ChatLaunch) []string {
	switch a.kind {
	case providers.Claude:
		return slices.Concat(claudeChatArgs, claudeLaunchArgs(l), providerArgs(
			a.kind, a.name, a.resume, a.model, a.effort, a.lichBin, a.agent, false, a.skipPermissions, a.ultracode, a.route, a.lang,
		))
	case providers.Kiro:
		model := modelArgs(a.kind, a.model, a.resume)
		if l.Model != "" {
			model = modelArgs(a.kind, l.Model, "")
		}
		return slices.Concat([]string{acpSubcmd}, agentArgs(a.kind, a.agent), skipPermissionArgs(a.kind, a.skipPermissions),
			model, effortArgs(a.kind, a.effort, a.resume))
	case providers.Antigravity:
		return slices.Concat([]string{acpSubcmd}, skipPermissionArgs(a.kind, a.skipPermissions))
	}
	return []string{acpSubcmd}
}

// claudeAllowBypassFlag lets a Claude Code chat be set to bypassPermissions
// over the protocol without starting in it: set_permission_mode refuses that
// mode to a process launched without it or --dangerously-skip-permissions
// (`bypass_not_launched`, measured on 2.1.327).
const claudeAllowBypassFlag = "--allow-dangerously-skip-permissions"

func claudeLaunchArgs(l ChatLaunch) []string {
	if !l.AllowBypass {
		return nil
	}
	return []string{claudeAllowBypassFlag}
}

// ChatLaunch is what a chat's process has to be started with for a setting
// its protocol cannot change in flight: Claude Code's bypassPermissions, which
// needs AllowBypass, and Kiro's model, whose session/set_model never answers
// and leaves the model as it was (measured on 2.21.0) while `acp --model`
// holds on a session/load. Model is passed on a resume too, unlike a birth
// model: it is the person's choice of a moment ago.
type ChatLaunch struct {
	Model       string
	AllowBypass bool
}

// ChatBirth is what a chat's argv could not carry, for the protocol to set:
// the model and effort the session was born on, empty on a resume (a birth
// value, modelArgs), and whether every permission is answered yes, which is
// the checkout's skip-permissions setting.
type ChatBirth struct {
	Model, Effort string
	AutoApprove   bool
}

// Claude Code and Kiro take all three on their argv and Antigravity takes the
// last (chatArgs).
func (s *Service) chatBirth(opts api.StartOptions) ChatBirth {
	var b ChatBirth
	flagged := opts.Kind == providers.Claude || opts.Kind == providers.Kiro
	if opts.Resume == "" && !flagged {
		b.Model, _ = flagValue(s.store.SessionModel(opts.ID))
		b.Effort, _ = flagValue(s.store.SessionEffort(opts.ID))
	}
	if !flagged && opts.Kind != providers.Antigravity {
		b.AutoApprove = s.store.SkipPermissions(opts.Kind, opts.ProjectID, opts.Cwd)
	}
	return b
}

// ChatProcess is a session's provider running as a chat: Stdin and Stdout
// carry the protocol.
type ChatProcess struct {
	Stdin  io.WriteCloser
	Stdout io.ReadCloser
	Birth  ChatBirth

	cmd    *exec.Cmd
	stderr *tailBuffer
	done   chan struct{}
	stop   sync.Once
}

// Done closes once the process has exited.
func (p *ChatProcess) Done() <-chan struct{} { return p.done }

// Stderr is the end of what the process wrote to stderr, which is what says
// why one that died did.
func (p *ChatProcess) Stderr() string { return p.stderr.String() }

// Close ends the process the way both protocols are told to end, by closing
// its stdin, and kills it if it has not left within chatCloseGrace. It returns
// once the process is gone.
func (p *ChatProcess) Close() {
	p.stop.Do(func() {
		_ = p.Stdin.Close()
		select {
		case <-p.done:
		case <-time.After(chatCloseGrace):
			_ = p.cmd.Process.Kill()
			<-p.done
		}
	})
}

// StartChat spawns session opts.ID's provider as a chat, rooted where its
// terminal would be and with everything its terminal would carry, plus what
// launch adds. Refused while the session's terminal runs.
func (s *Service) StartChat(_ context.Context, opts api.StartOptions, launch ChatLaunch) (*ChatProcess, error) {
	if !providers.Known(opts.Kind) {
		return nil, fmt.Errorf("%q has no chat", opts.Kind)
	}
	name := s.AgentName(opts.ID)
	bornIn, err := s.store.SessionProfile(opts.ID)
	if err != nil {
		return nil, err
	}
	profileDir, err := s.profileDir(opts.ID, opts.Kind, opts.Resume)
	if err != nil {
		return nil, err
	}
	oc2, err := s.opencode2Spawn(&opts, profileDir)
	if err != nil {
		return nil, err
	}
	inheritClaudeTrust(opts.Kind, opts.Cwd, profileDir)
	inheritCursorTrust(opts.Kind, opts.Cwd)
	p, l, err := s.spawnChat(opts, launch, profileDir, name, oc2)
	if err != nil {
		return nil, err
	}
	s.publishCwd(cwdEvent{ID: opts.ID, Cwd: opts.Cwd})
	events.Send(s.hub.Emit, api.SessionAgentEvent, agentEvent{ID: opts.ID, Agent: opts.Kind})
	s.reportProfileMove(opts.ID, bornIn)
	s.reportSandbox(opts.ID, l.confined, l.links)
	s.reportMCPServers(opts.ID, opts.Kind, opts.Cwd, profileDir)
	return p, nil
}

// spawnChat is the locked half of StartChat, spawnSession's counterpart.
func (s *Service) spawnChat(opts api.StartOptions, launch ChatLaunch, profileDir, name string, oc2 bool) (*ChatProcess, spawnLaunch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, running := s.sessions[opts.ID]; running {
		return nil, spawnLaunch{}, fmt.Errorf("session %q still runs its terminal", opts.ID)
	}
	s.forgetTurn(opts.ID)
	if opts.Cwd == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, spawnLaunch{}, fmt.Errorf("failed to resolve home directory: %w", err)
		}
		opts.Cwd = home
	}
	l, err := s.prepareSpawnLocked(opts, profileDir, name, oc2, func(a spawnArgs) []string { return chatArgs(a, launch) })
	if err != nil {
		return nil, spawnLaunch{}, err
	}
	started := spawn{kind: opts.Kind, cwd: opts.Cwd, projectID: opts.ProjectID, profile: profileSlug(opts.Kind, profileDir), chat: true}
	// Dropped before Done closes, so whoever waited the process out can start
	// the terminal at once; only this process's own entry, in case one started
	// after it under the same id. The turn is left to the chat to close: it
	// reports the end a dead process took with it.
	s.spawns.Store(opts.ID, started)
	p, err := startChatProcess(l.spec, func() { s.spawns.CompareAndDelete(opts.ID, started) })
	if err != nil {
		s.spawns.CompareAndDelete(opts.ID, started)
		return nil, spawnLaunch{}, fmt.Errorf("failed to start the chat of %q: %w", opts.ID, err)
	}
	p.Birth = s.chatBirth(opts)
	return p, l, nil
}

// startChatProcess runs spec on pipes. The pipes are the OS's own rather than
// exec's copying goroutines, so Wait never races the reader for stdout.
func startChatProcess(spec ptySpec, exited func()) (*ChatProcess, error) {
	path, err := exec.LookPath(spec.bin)
	if err != nil {
		return nil, err
	}
	argv := wrapArgv(path, spec.args)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir, cmd.Env = spec.dir, spec.env
	winexec.Hide(cmd)
	inR, inW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		_ = inR.Close()
		_ = inW.Close()
		return nil, err
	}
	stderr := &tailBuffer{max: chatStderrTail}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = inR, outW, stderr
	cmd.WaitDelay = chatCloseGrace
	if err := cmd.Start(); err != nil {
		for _, f := range []*os.File{inR, inW, outR, outW} {
			_ = f.Close()
		}
		return nil, err
	}
	_ = inR.Close()
	_ = outW.Close()
	p := &ChatProcess{Stdin: inW, Stdout: outR, cmd: cmd, stderr: stderr, done: make(chan struct{})}
	go func() {
		_ = cmd.Wait()
		exited()
		close(p.done)
		time.AfterFunc(chatDrain, func() { _ = outR.Close() })
	}()
	return p, nil
}

// chatStderrTail is how much of a chat process's stderr is kept.
const chatStderrTail = 4096

// tailBuffer keeps the last max bytes written to it.
type tailBuffer struct {
	mu  sync.Mutex
	max int
	buf []byte
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	if over := len(b.buf) - b.max; over > 0 {
		b.buf = b.buf[over:]
	}
	return len(p), nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

// StopTerminal ends session id's terminal the way a close does, keeping the
// session, and returns once its child has been reaped. A session with no
// terminal running is left alone.
func (s *Service) StopTerminal(id string) error {
	sess := s.sessionOf(id)
	if sess == nil {
		return nil
	}
	err := s.endProcess(id, "")
	if sess.reaped == nil {
		return err
	}
	select {
	case <-sess.reaped:
	case <-time.After(stopTerminalWait):
		return errors.Join(err, fmt.Errorf("the terminal of %q did not exit", id))
	}
	return err
}

// ReportChatState is a chat's own state report, read the way a hook's is:
// the card's ring, the unread mark, the relay's watch and the readouts taken
// off the transcript all follow it. A chat reports for itself, so the hooks its
// provider still fires are dropped (onHookState).
func (s *Service) ReportChatState(id, state, tool, detail string) {
	s.applyState(hookRequest{SessionID: id, State: state, Tool: tool, Detail: detail})
}

// BindConversation records the conversation session id's chat runs, which is
// what its terminal resumes.
func (s *Service) BindConversation(id, providerSessionID string) error {
	return s.setConversation(id, providerSessionID)
}

// TurnOpen reports whether session id is in the middle of a turn.
func (s *Service) TurnOpen(id string) bool { return s.turns.busy(id) }

// Resumable reports whether conversation providerSessionID of a kind session
// can still be reopened (resumeAvailable).
func (s *Service) Resumable(kind, providerSessionID, cwd string) bool {
	return s.resumeAvailable(kind, providerSessionID, cwd)
}

// ClaudeTranscript is where Claude Code keeps conversation id, found under
// every profile.
func ClaudeTranscript(id string) (string, bool) { return claudeTranscriptPath(id) }
