package terminal

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/agentplugin"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/relay"

	"github.com/lichdotdev/lich/internal/events"
)

// Start spawns the binary for session opts.ID under project opts.ProjectID — the user's
// shell when opts.Kind is "shell", otherwise the provider binary for that kind
// resolved from the project's settings (falling back to the provider's default
// on $PATH) — attached to a new PTY sized to opts.Cols x opts.Rows and rooted at
// opts.Cwd, then streams its output to the window. An empty opts.Cwd defaults to
// the user's home directory. Starting a session that is already running is a no-op.
//
// A non-empty opts.Resume is a provider conversation id to reopen, spelled for the
// session's kind by resumeArgs, which the window passes after the user
// accepted the prompt to continue the session this card ran before the last
// restart. The prompt is only raised for a conversation ResumeAvailable still
// finds, so an id the provider no longer knows normally never reaches here; one
// that slips through (pruned between the check and the spawn) fails in the PTY
// like any other bad invocation — the user sees the provider's own error.
//
// opts.Fork turns that resume into a branch: the provider copies the conversation
// into a new one of its own and leaves the original alone, so the card that ran
// it stays where it is with its history intact. Only the three kinds
// providers.SupportsFork names can do it, and only alongside a resume id — the
// conversation being branched is the one that id points at. The copy's own id
// is the provider's to assign, and reaches lich through this session's
// session-start report like any other.
//
// The name the session answers to in its provider's peer roster (Claude Code's
// `/list-agents`, the only provider with one) is a name already on record for
// this session (AgentName): a restart opens a conversation of its own, and a
// `/rename` typed in the last one is a decision the user made about this card
// rather than about that conversation. A fork with nothing on record is named by
// lich (nameArgs); any other session is left to name itself.
func (s *Service) Start(_ context.Context, opts api.StartOptions) (api.None, error) {
	// Refused before the spawn rather than dropped in resumeArgs: a fork the
	// provider cannot spell would reopen the parent's own conversation, and two
	// cards on one conversation is exactly what a fork promises not to do.
	if opts.Fork && !providers.SupportsFork(opts.Kind) {
		return api.None{}, fmt.Errorf("%s cannot fork a conversation — it only resumes one", opts.Kind)
	}
	if opts.Fork && opts.Resume == "" {
		return api.None{}, fmt.Errorf("a fork needs the conversation to branch, and no resume id was given")
	}
	if opts.Fork {
		s.recordForkCost(opts.ID, opts.Kind, opts.Resume)
		s.inheritUltracode(opts.ID, opts.Resume)
		s.inheritSubagent(opts.ID, opts.Resume)
	}
	// Resolved before the lock, because it reads a file: the conversation this
	// spawn replaces is still on the row until its successor reports, so a name
	// recorded there is the one this card answers to.
	name := s.AgentName(opts.ID)
	if name == "" && opts.Fork {
		name = relay.RosterName(opts.Cwd, opts.ID)
	}
	// Before the lock: it may read the disk for the conversation's profile.
	bornIn, err := s.store.SessionProfile(opts.ID)
	if err != nil {
		return api.None{}, err
	}
	profileDir, err := s.profileDir(opts.ID, opts.Kind, opts.Resume)
	if err != nil {
		return api.None{}, err
	}
	inheritClaudeTrust(opts.Kind, opts.Cwd, profileDir)
	inheritCursorTrust(opts.Kind, opts.Cwd)
	markCrushInitialized(opts.Kind, opts.Cwd)
	awaits := s.awaitsStart(opts.Kind, profileDir)
	sess, cwd, err := s.spawnSession(opts, profileDir, name)
	if err != nil || sess == nil {
		return api.None{}, err
	}
	// Set a moment after the spawn rather than inside it, since awaitsStart
	// may run the provider's CLI and the spawn holds s.mu. The gap is far
	// shorter than readySettle, and Ready asks for the report before it
	// trusts a quiet it latched, so nothing gets through it.
	s.holdForStart(opts.ID, sess, awaits)
	// Emitted outside s.mu: Emit blocks on a stalled /events client, which
	// would freeze every session's I/O. Both are unconditional so a respawn
	// overwrites whatever the previous PTY left in the window's stores.
	s.publishCwd(cwdEvent{ID: opts.ID, Cwd: cwd})
	events.Send(s.hub.Emit, api.SessionAgentEvent, agentEvent{ID: opts.ID, Agent: ""})
	s.reportProfileMove(opts.ID, bornIn)
	s.reportSandbox(opts.ID, sess.confined, sess.sandboxLinks)
	s.reportMCPServers(opts.ID, opts.Kind, cwd)
	go watchCwd(opts.ID, sess.pty.Pid(), cwd, sess.done, s.publishCwd)
	return api.None{}, nil
}

// spawnSession is the locked half of Start: dedupe, PTY spawn and session-map
// registration. A nil session with a nil error means opts.ID was already running.
// The returned cwd is the effective start directory (opts.Cwd, or the
// resolved home when it was empty).
func (s *Service) spawnSession(opts api.StartOptions, profileDir, name string) (*session, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, running := s.sessions[opts.ID]; running {
		return nil, "", nil
	}
	// Whatever the previous provider left open under this id died with its PTY,
	// and the first report of the new one has to be read against its own turn.
	s.forgetTurn(opts.ID)
	opts.Cols, opts.Rows = s.sizeFor(opts.Cols, opts.Rows)

	if opts.Cwd == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, "", fmt.Errorf("failed to resolve home directory: %w", err)
		}
		opts.Cwd = home
	}

	// The MCP registration is withheld without a transport for the same reason
	// the hook coordinates are: the server it points at would have nothing to
	// talk to, so the session would carry tools that can only fail.
	mcpBin := ""
	if s.ws != nil {
		mcpBin = lichBin()
	}
	skipPermissions := s.store.SkipPermissions(opts.Kind, opts.ProjectID, opts.Cwd)
	ultracode := s.store.Ultracode(opts.Kind) || s.store.SessionUltracode(opts.ID)
	depth := s.store.SessionSubagentDepth(opts.ID)
	cardsOn := s.subagentCardsOn(opts.Kind, depth)
	// The model and effort are read from the row rather than passed in, so every
	// spawn that starts a conversation of its own gets them: the window's first
	// view and a restart. A resume leaves them to the provider (modelArgs).
	env, err := pasteUnfoldEnv(
		subagentEnv(s.sessionEnv(opts.ID, opts.ProjectID, opts.Cwd), opts.Kind, depth, cardsOn), opts.Kind, s.store.PasteUnfold(opts.Kind),
	)
	if err != nil {
		return nil, "", err
	}
	// The configured binary still runs, so a wrapper exporting a
	// CLAUDE_CONFIG_DIR (or CODEX_HOME) of its own wins over the profile.
	if profileDir != "" {
		env = profileEnv(opts.Kind, env, profileDir)
	}
	// Decided before the argv, which carries the trust flags a confined spawn
	// is given (confinedTrustArgs).
	inSandbox := confined(s.store, opts.ID, opts.Kind, opts.ProjectID, opts.Cwd)
	// Here rather than beside the other trust writes in Start, before the lock,
	// because it depends on the sandbox verdict; it reads one small file and
	// asks git once, and only for an Antigravity spawn.
	inheritAntigravityTrust(opts.Kind, opts.Cwd, inSandbox)
	spec := ptySpec{
		bin: resolveCommand(opts.Kind, s.store.ProviderBin(opts.Kind, opts.ProjectID), userShell()),
		args: append(confinedTrustArgs(opts.Kind, opts.Cwd, inSandbox), providerArgs(
			opts.Kind, name, opts.Resume, s.store.SessionModel(opts.ID), s.store.SessionEffort(opts.ID), mcpBin,
			kiroPluginAgent(opts.Kind), opts.Fork, skipPermissions, ultracode, subagentRoute(opts.Kind, depth, cardsOn),
			s.promptLanguage(),
		)...),
		dir:  opts.Cwd,
		env:  env,
		cols: opts.Cols,
		rows: opts.Rows,
	}
	spec = withShellIntegration(spec, opts.Kind, s.shellIntegrationDir)
	creds := s.sandboxCredentials(opts.ProjectID, opts.Cwd, inSandbox)
	spec, skippedLinks := wrapSandbox(
		spec, opts.Kind, userHome(), sessionDropDir(s.dropDir, opts.ID, inSandbox), profileDir, inSandbox, creds,
	)
	header, err := encodeFrame(opts.ID, nil)
	if err != nil {
		return nil, "", fmt.Errorf("start %q: %w", opts.ID, err)
	}
	p, err := startPTY(spec)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start pty for %q: %w", opts.ID, err)
	}

	box := newOutbox(header, func(frame []byte) {
		if s.ws != nil && s.ws.send(frame) {
			return
		}
		s.emitData(opts.ID, frame[len(header):])
	}, func(tail []byte) {
		// The /events bridge has no replay: the window that asked for this
		// one reads /ws, and without /ws it reads no output at all.
		if s.ws != nil {
			s.ws.sendReplay(opts.ID, tail)
		}
	}, outboxDepth)
	out := newCoalescer(box.push, visibleFlushInterval, hiddenFlushInterval)
	sess := &session{
		pty:    p,
		out:    out,
		done:   make(chan struct{}),
		replay: newReplayBuffer(replayCapBytes),
		outbox: box,
		// Timed from the spawn, so a program that never writes anything still
		// becomes ready once: quiet is the signal, and silence from the start
		// is quiet too.
		lastOut:      time.Now(),
		confined:     inSandbox,
		sandboxLinks: skippedLinks,
	}
	s.sessions[opts.ID] = sess
	// Outside mu's protection by design (see Service.spawns), and stored with the
	// registration so no report can arrive for a session whose kind is unknown.
	s.spawns.Store(opts.ID, spawn{kind: opts.Kind, cwd: opts.Cwd, projectID: opts.ProjectID, profile: profileSlug(opts.Kind, profileDir)})
	go s.stream(opts.ID, sess)
	return sess, opts.Cwd, nil
}

// providerKind resolves which provider mark a session-start report puts on a
// card: the kind lich spawned when that kind is a provider, else the reported
// one. A session that is not running — the report raced its own PTY's exit —
// falls back to the report, which is all that is left to answer from.
func (s *Service) providerKind(id, reported string) string {
	if kind := s.kindOf(id); providers.Known(kind) {
		return kind
	}
	return reported
}

// spawn is how one live session's PTY was started: the kind it runs, the
// project and directory it was rooted at, and the profile it runs in ("" for
// Personal). The cwd is the spawn directory and not the foreground
// one the watcher follows (cwd.go) — Crush keeps its database in the checkout it
// was started in, which a `cd` inside the session does not move.
type spawn struct {
	kind      string
	cwd       string
	projectID string
	profile   string
}

// spawnOf is how a live session's PTY was started, or the zero spawn once it is
// gone. It never takes mu: a hook asks this on every report, and mu is held
// across a PTY spawn (see Service.spawns).
func (s *Service) spawnOf(id string) spawn {
	v, ok := s.spawns.Load(id)
	if !ok {
		return spawn{}
	}
	started, _ := v.(spawn)
	return started
}

// kindOf is what a live session's PTY was spawned to run, or "" once it is gone.
func (s *Service) kindOf(id string) string { return s.spawnOf(id).kind }

// repeatedEnding reports whether a `done` is the same turn ending a second
// time. Cursor CLI runs the plugin installed in Claude Code, and its TUI
// delivers that plugin's UserPromptSubmit and Stop (measured on 2026.08.11
// through 2026.10.01), so every Cursor turn opens with a `busy`. Esc ends one
// with two stop hooks for the same generation, "aborted" then "error", about
// 60ms apart; the second would ring the card again and close as unanswered an
// errand the relay delivered in that gap.
//
// Cursor only, because only Cursor is measured to do it: a `done` with no turn
// open is not a repeat everywhere — the one that ends a /compact follows
// `compacting`, which opens no turn.
func (s *Service) repeatedEnding(id, state string) bool {
	return state == statusDone && s.kindOf(id) == providers.Cursor && !s.turns.busy(id)
}

// reportSandbox records and announces how this spawn was confined: the verdict,
// and the home paths the sandbox skipped for being symlinks. The verdict is
// already on the row (confined writes it); the skipped links are written here,
// because only the spawn resolves them and the PTY outlives the window — a
// reload has no second spawn to hear them from.
//
// A failed write is logged and nothing else, for the reason reportMCPServers
// gives: it costs the card a line at the next reload, and refusing to spawn a
// session over what its tooltip says would be the worse trade.
func (s *Service) reportSandbox(id string, confined bool, links []string) {
	if err := s.store.SetSessionSandboxLinks(id, links); err != nil {
		slog.Warn("record sandbox skipped links", "session", id, "error", err)
	}
	events.Send(s.hub.Emit, api.SessionSandboxEvent, sandboxEvent{ID: id, Confined: confined, SkippedLinks: links})
}

// reportMCPServers records and announces the MCP servers this spawn's provider
// could reach. Resolved here rather than once at startup because the answer
// takes the harness's own config documents and this session's directory to
// reach, and both move while lich is open: a server registered after launch is
// divided on the next card to spawn.
//
// A failed write is logged and nothing else. It costs a card the split on the
// next reload — the event has already gone out — and refusing to spawn a
// session over the way its tool names draw would be the worse trade.
func (s *Service) reportMCPServers(id, kind, cwd string) {
	servers := agentplugin.MCPServers(kind, cwd)
	if err := s.store.SetSessionMCPServers(id, servers); err != nil {
		slog.Warn("record MCP servers", "session", id, "error", err)
	}
	events.Send(s.hub.Emit, api.SessionMCPEvent, mcpEvent{ID: id, Servers: servers})
}
