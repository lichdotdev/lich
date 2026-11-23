package terminal

import (
	"context"
	"log/slog"
	"os/exec"
	"slices"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/codexhooks"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/winexec"
)

// codexAppServer is the app-server subcommand a Codex terminal session's
// hook check runs on its own, beside the TUI.
var codexAppServerProbe = []string{codexAppServerSubcmd}

// CodexHookChecked is whether a Codex session already read whether Codex
// runs lich-plugin's PreToolUse hook: the check runs once per lich run.
func (s *Service) CodexHookChecked() bool { return s.codexHookChecked.Load() }

// ReportCodexHook files the first hook check's answer, trust being lich's
// PreToolUse hook's status when Codex skips it and "" when it runs, and tells
// the window when it is skipped. Later reports are dropped.
func (s *Service) ReportCodexHook(id, trust string) {
	if !s.codexHookChecked.CompareAndSwap(false, true) || trust == "" {
		return
	}
	events.Send(s.hub.Emit, api.CodexHookUntrustedEvent, api.CodexHookUntrusted{SessionID: id, Trust: trust})
}

// checkCodexHook reads a Codex terminal session's hooks/list through a
// short-lived `codex app-server` on the session's own binary, directory and
// environment (its CODEX_HOME included), outside the sandbox: the TUI offers
// no hooks/list, and nothing here writes the person's trust. Measured on
// 0.170.0: under a second.
func (s *Service) checkCodexHook(id string, cli ptySpec) {
	if s.CodexHookChecked() {
		return
	}
	path, err := exec.LookPath(cli.bin)
	if err != nil {
		return
	}
	argv := wrapArgv(path, codexAppServerProbe)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir, cmd.Env = cli.dir, slices.Clone(cli.env)
	winexec.Hide(cmd)
	list, err := codexhooks.Probe(context.Background(), cmd, cli.dir)
	if err != nil {
		slog.Debug("terminal: read Codex hooks", "session", id, "err", err)
		return
	}
	s.ReportCodexHook(id, codexhooks.LichTrust(list))
}
