// Package system holds the few OS integrations the window needs that are
// not tied to any domain service — opening a URL in the user's default browser,
// opening a work-tree file in their editor, raising a desktop notification, and
// the handful of facts a bug report needs (version, platform, where the log
// lives).
package system

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/relpath"
	"github.com/lichdotdev/lich/internal/sandbox"
	"github.com/ncruces/zenity"
)

type Service struct {
	// mu guards env alone: a re-read of the login shell replaces it while the
	// window is asking to open a file (SetEnv).
	mu sync.RWMutex
	// env is the resolved login-shell environment (see terminal.ResolveShellEnv),
	// the source of $VISUAL/$EDITOR — a GUI launch never sourced the user's rc.
	env []string
	// logPath is the running build's log file (see logging.Path). The window
	// cannot derive it: the config dir is resolved by this process, not by the
	// window.
	logPath string
	// version is the running build's version, "dev" outside a release.
	version string
	// uncleanExit records that the previous run never reached its clean exit
	// (see singleton.UncleanExit). Consumed by the first reader, so it is
	// atomic: a window opened again on the same backend calls in again.
	uncleanExit atomic.Bool
	// run launches a detached process; injected in tests, exec in production.
	run func(name string, args ...string) error
	// notify posts one desktop notification; injected in tests, zenity in
	// production (see Notify).
	notify func(text, title string) error
	// quit is the exit, wired by main (SetQuit).
	quit   func() error
	native nativeWindow
}

func New(env []string, logPath, version string, uncleanExit bool) *Service {
	s := &Service{
		env:     env,
		logPath: logPath,
		version: version,
		run: func(name string, args ...string) error {
			return exec.Command(name, args...).Start()
		},
		notify: func(text, title string) error {
			return zenity.Notify(text, zenity.Title(title))
		},
	}
	s.uncleanExit.Store(uncleanExit)
	return s
}

// SetEnv replaces the resolved environment after a re-read of the login shell
// (internal/providers.Service.RefreshPath): an $EDITOR exported by an rc file
// the user edited while lich was open is found from the same re-check that
// finds a newly installed agent, rather than from the next launch.
func (s *Service) SetEnv(env []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.env = env
}

// TakeUncleanExit answers takeUncleanExit over RPC.
func (s *Service) TakeUncleanExit(_ context.Context, _ api.None) (bool, error) {
	return s.takeUncleanExit(), nil
}

// takeUncleanExit reports whether the run before this one ended without
// quitting cleanly, and clears the flag. It is consumed rather than read because the
// notice belongs to the launch that followed the bad exit, not to every window
// that opens on it: each window runs its gate again, the process does not
// start again with it.
func (s *Service) takeUncleanExit() bool {
	return s.uncleanExit.Swap(false)
}

type Diagnostics = api.Diagnostics

// Diagnostics answers diagnostics over RPC.
func (s *Service) Diagnostics(_ context.Context, _ api.None) (Diagnostics, error) {
	return s.diagnostics(), nil
}

func (s *Service) diagnostics() Diagnostics {
	return Diagnostics{
		Version:  s.version,
		Platform: runtime.GOOS + "/" + runtime.GOARCH,
		LogPath:  s.logPath,
	}
}

// SandboxBackend answers sandboxBackend over RPC.
func (s *Service) SandboxBackend(_ context.Context, _ api.None) (string, error) {
	return s.sandboxBackend(), nil
}

// sandboxBackend names what confines a session on this machine — "bubblewrap",
// "sandbox-exec" — or "" when nothing can (internal/sandbox). It answers here
// rather than beside the provider list because it is a fact about the machine,
// and the same answer for every provider on it.
//
// The name rather than a yes: the two backends have different holes, and the
// Sandbox pane says which one is in play so a report about a confined session
// starts a round ahead. "" is what the pane draws its "cannot confine" state
// from, and what the new-session dialog reads as "do not offer the choice".
func (s *Service) sandboxBackend() string {
	return sandbox.Backend()
}

// RevealLog opens the log's directory with the platform's file manager, not the
// file itself: the rotated generation (*.old) is often the one holding the bug,
// and a log file handed to its default handler opens in an editor, not beside
// the sibling the reporter also needs.
func (s *Service) RevealLog(_ context.Context, _ api.None) (api.None, error) {
	if s.logPath == "" {
		return api.None{}, fmt.Errorf("no log file: lich is logging to stderr only")
	}
	return api.None{}, s.openFolder(filepath.Dir(s.logPath))
}

// notifyTitle is the app name carried by every notification lich raises. macOS
// shows it as the notification's title and Windows as the toast's source; Linux
// ignores it, taking the text's first line as the summary — which is why the
// text, not the title, has to carry what happened.
const notifyTitle = "lich"

// notifyLimit bounds each line of a notification, in runes. Both lines are
// labels a user or an agent wrote (a session's name, a project's), and a
// desktop notification is not the place to render an essay.
const notifyLimit = 120

// Notify raises a desktop notification through the OS's own notification
// service — the one channel that still reaches the user after they leave the
// lich window. summary is the headline, detail the second line (empty omits
// it).
//
// Delivery only: the window decides when a notification is warranted, because it
// is the side that knows whether the window has focus and which session the
// user is looking at.
func (s *Service) Notify(_ context.Context, opts api.NotifyOptions) (api.None, error) {
	text := truncate(opts.Summary, notifyLimit)
	if opts.Detail != "" {
		text += "\n" + truncate(opts.Detail, notifyLimit)
	}
	if strings.TrimSpace(text) == "" {
		return api.None{}, errors.New("refusing to raise an empty notification")
	}
	return api.None{}, s.notify(text, notifyTitle)
}

// truncate caps s at limit runes — never bytes, which would cut a multi-byte
// character in half and hand the notifier invalid UTF-8.
func truncate(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "…"
}

// OpenExternal opens an http(s) URL in the default browser. Scheme-gated so a
// crafted terminal escape can never turn a click into a file:// or custom
// scheme launch. The launcher itself is per-OS (see the open_* files).
func (s *Service) OpenExternal(_ context.Context, opts api.OpenExternalOptions) (api.None, error) {
	if err := ValidateExternalURL(opts.URL); err != nil {
		return api.None{}, err
	}
	argv := urlOpenArgv(opts.URL)
	return api.None{}, s.run(argv[0], argv[1:]...)
}

// OpenFolderInEditor opens a session's checkout in the user's editor. It prefers
// $VISUAL, then $EDITOR, resolved from the login shell, so a GUI launch still
// sees rc exports. A terminal editor (vim, nvim, nano, …) is launched nothing:
// the shell command line to run in a lich terminal session comes back instead,
// since a detached launch would give it no controlling terminal. A GUI editor,
// or the file manager when none is set, is launched detached, and "" comes
// back.
func (s *Service) OpenFolderInEditor(_ context.Context, opts api.OpenFolderOptions) (string, error) {
	if err := requireDir(opts.Dir); err != nil {
		return "", err
	}
	return s.openInEditor(opts.Dir)
}

// OpenFolder shows a checkout in the platform's file manager. It never consults
// $VISUAL/$EDITOR: this is the way out of lich to the OS's own view of the
// folder, which is the one thing opening it in an editor does not give.
func (s *Service) OpenFolder(_ context.Context, opts api.OpenFolderOptions) (api.None, error) {
	if err := requireDir(opts.Dir); err != nil {
		return api.None{}, err
	}
	return api.None{}, s.openFolder(opts.Dir)
}

// OpenFile hands one work-tree file to the application the desktop opens its
// type with: an image viewer, a PDF reader. Unlike OpenFolderInEditor it never
// consults $VISUAL/$EDITOR, which would open a PNG in vim. opts.Rel goes through
// relpath.Resolve, as project.ReadFile does, so a symlink out of the checkout
// is refused rather than opened. The launcher is openFolder's: xdg-open, open
// and explorer each hand any path, file or folder, to its default handler.
func (s *Service) OpenFile(_ context.Context, opts api.OpenFileOptions) (api.None, error) {
	full, err := relpath.Resolve(opts.Dir, opts.Rel)
	if err != nil {
		return api.None{}, err
	}
	info, err := os.Stat(full)
	if err != nil {
		return api.None{}, fmt.Errorf("stat %s: %w", opts.Rel, err)
	}
	if !info.Mode().IsRegular() {
		return api.None{}, fmt.Errorf("%s is not a regular file", opts.Rel)
	}
	return api.None{}, s.openFolder(full)
}

// requireDir refuses a path that is not a directory any more, naming it. lich
// watches no checkout: a worktree removed from a terminal, or a project folder
// moved, still has its card, and a launcher handed the dead path either does
// nothing or fails somewhere nobody is looking.
func requireDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("this folder is gone: %s", dir)
	}
	return nil
}

// openInEditor resolves the editor for one absolute directory and either
// launches it or hands back the command line to run; with no editor set, the
// file manager shows the directory.
func (s *Service) openInEditor(full string) (string, error) {
	editor := s.getenv("VISUAL")
	if editor == "" {
		editor = s.getenv("EDITOR")
	}
	if editor != "" && isTerminalEditor(editor) {
		// The command runs in the session's shell, so the file path — the
		// caller-influenced part — must survive spaces and metacharacters. The
		// quoting rule is the shell's, and the session's shell is per-OS (see
		// the open_* files).
		return editor + " " + s.quoteForShell(full), nil
	}
	if editor != "" {
		return "", s.runEditor(editor, full)
	}
	return "", s.openFolder(full)
}

// runEditor launches a GUI $EDITOR value that may carry flags ("code --wait"),
// appending the target as the final argument. An all-whitespace value degrades
// to the file manager rather than launching an empty command.
func (s *Service) runEditor(editor, full string) error {
	fields := strings.Fields(editor)
	if len(fields) == 0 {
		return s.openFolder(full)
	}
	args := append(fields[1:], full)
	return s.run(fields[0], args...)
}

// terminalEditors run inside the terminal; keyed by binary basename. A GUI
// launch gives them no controlling terminal, so lich runs them in a session.
// Ceiling: a fixed list — an unlisted terminal editor is treated as GUI and
// launched detached (and silently fails to open); add it here.
var terminalEditors = map[string]bool{
	"vi": true, "vim": true, "nvim": true, "neovim": true, "nano": true,
	"emacs": true, "emacsclient": true, "helix": true, "hx": true, "kak": true,
	"kakoune": true, "micro": true, "vis": true, "joe": true, "ne": true,
}

// isTerminalEditor reports whether the editor command (which may carry flags,
// e.g. "nvim -p") names a terminal editor, matched on the binary's basename.
func isTerminalEditor(editor string) bool {
	fields := strings.Fields(editor)
	if len(fields) == 0 {
		return false
	}
	return terminalEditors[filepath.Base(fields[0])]
}

// getenv reads a key from the resolved shell env, "" when absent.
func (s *Service) getenv(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	prefix := key + "="
	for _, kv := range s.env {
		if strings.HasPrefix(kv, prefix) {
			return kv[len(prefix):]
		}
	}
	return ""
}

// ValidateExternalURL accepts absolute http/https URLs only.
func ValidateExternalURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("refusing to open scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return fmt.Errorf("refusing to open url without host")
	}
	return nil
}
