package providers

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/gitutil"
	"github.com/lichdotdev/lich/internal/winexec"
)

const (
	// installTimeout bounds an install: a download that stalls must not leave
	// a provider installing for good.
	installTimeout = 10 * time.Minute
	// installWaitDelay is how long the installer's pipes may stay open after it
	// exits, held by a descendant it left running.
	installWaitDelay = 5 * time.Second
	// installCeiling is where the estimate stops until the installer exits, and
	// installHalfLines the lines of output that take it halfway there: no
	// installer states a percentage, so the line count is what moves.
	installCeiling   = 0.9
	installHalfLines = 6
	// installStep is the smallest move of the estimate worth an event.
	installStep = 0.01
	// installErrorMax is how many bytes of the installer's last line an error
	// quotes.
	installErrorMax = 200
)

// recipe is how a provider installs on one OS: the programs it needs on PATH,
// and the command line, whose first word is the first of them.
type recipe struct {
	needs []string
	args  []string
}

type Install = api.ProviderInstall

// errNoInstall is a provider lich has no unattended install for on this OS;
// installGaps says why.
var errNoInstall = errors.New("lich cannot install this provider here; its install page says how")

// Install answers providers.Install: it checks what the install needs, starts
// the installer and returns, and ProviderInstallEvent carries the rest. The
// installer runs as the user, with no terminal and no stdin, so whatever would
// ask for a password or a choice fails instead of waiting.
func (s *Service) Install(_ context.Context, opts api.ProviderIDOptions) (api.None, error) {
	r, err := s.installRecipe(opts.ID)
	if err != nil {
		return api.None{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), installTimeout)
	cmd := s.installCommand(ctx, r.args)
	out, w := io.Pipe()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, w, w
	cmd.Env = installEnv(os.Environ())
	cmd.WaitDelay = installWaitDelay
	ownSession(cmd)
	winexec.Hide(cmd)
	var t tree
	cmd.Cancel = func() error { return t.end(cmd.Process) }
	if err := s.claimInstall(opts.ID); err != nil {
		cancel()
		return api.None{}, err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		s.releaseInstall(opts.ID)
		return api.None{}, fmt.Errorf("start the %s install: %w", nameOf(opts.ID), err)
	}
	t = adopt(cmd.Process)
	relayed := make(chan string, 1)
	go func() { relayed <- s.relayInstall(opts.ID, out) }()
	go func() {
		waitErr := cmd.Wait()
		_ = w.Close()
		last := <-relayed
		_ = t.end(cmd.Process)
		s.finishInstall(ctx, opts.ID, last, waitErr)
		cancel()
	}()
	return api.None{}, nil
}

// installRecipe is the recipe for id on this OS, once the machine can run it.
func (s *Service) installRecipe(id string) (recipe, error) {
	if !Known(id) {
		return recipe{}, fmt.Errorf("unknown provider %q", id)
	}
	if asRoot() {
		return recipe{}, errors.New("lich does not install providers as root")
	}
	r, ok := recipes[hostOS][id]
	if !ok {
		return recipe{}, errNoInstall
	}
	for _, need := range r.needs {
		if _, err := s.lookPath(need); err != nil {
			return recipe{}, fmt.Errorf("installing %s needs %s, which is not on PATH", nameOf(id), need)
		}
	}
	return r, nil
}

func (s *Service) installCommand(ctx context.Context, args []string) *exec.Cmd {
	if s.newInstall != nil {
		return s.newInstall(ctx, args)
	}
	return exec.CommandContext(ctx, args[0], args[1:]...)
}

func (s *Service) claimInstall(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.installing[id] {
		return fmt.Errorf("%s is already installing", nameOf(id))
	}
	if s.installing == nil {
		s.installing = map[string]bool{}
	}
	s.installing[id] = true
	return nil
}

func (s *Service) releaseInstall(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.installing, id)
}

// installEnv is the backend's own environment, whose PATH is the one it
// pinned, with every prompter that could open a dialog silenced.
func installEnv(env []string) []string {
	return append(gitutil.NoPrompt(env), "SUDO_ASKPASS=")
}

// relayInstall reads the installer's output as it comes, a line at a time (a
// progress bar's carriage return ends one too), sends the estimate as it
// moves, and returns the last line that said something.
func (s *Service) relayInstall(id string, out io.Reader) string {
	scan := bufio.NewScanner(out)
	scan.Split(scanLines)
	lines, sent, last := 0, 0.0, ""
	for scan.Scan() {
		text := strings.TrimSpace(scan.Text())
		if text == "" {
			continue
		}
		last = text
		lines++
		slog.Debug("provider install", "provider", id, "line", text)
		if p := installProgress(lines); p-sent >= installStep {
			sent = p
			s.sendInstall(Install{ID: id, Progress: p})
		}
	}
	// A line longer than the scanner's buffer stops it: drain the rest so the
	// installer never blocks on a full pipe.
	_, _ = io.Copy(io.Discard, out)
	return last
}

// installProgress is the estimate after lines of output: it rises fast, then
// slows, and never reaches installCeiling while the installer runs.
func installProgress(lines int) float64 {
	return installCeiling * float64(lines) / float64(lines+installHalfLines)
}

// scanLines is bufio.ScanLines that also ends a line at a carriage return.
func scanLines(data []byte, atEOF bool) (int, []byte, error) {
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// finishInstall sends the last event: why the install failed, or, once the
// PATH lich resolves binaries through is read again, that the provider is on it.
func (s *Service) finishInstall(ctx context.Context, id, last string, waitErr error) {
	defer s.releaseInstall(id)
	done := Install{ID: id, Progress: 1, Done: true}
	if failure := installFailure(ctx.Err(), last, waitErr); failure != "" {
		slog.Warn("provider install failed", "provider", id, "err", failure)
		done.Error = failure
		s.sendInstall(done)
		return
	}
	if _, err := s.RefreshPath(ctx, api.None{}); err != nil {
		slog.Warn("provider install: refresh PATH", "provider", id, "err", err)
	}
	p, _ := find(id)
	if d := s.resolve(Detected{ID: id}, p.Binaries); !d.Installed {
		done.Error = fmt.Sprintf("%s installed, but %s is not on PATH", p.Name, DefaultBinary(id))
	}
	s.sendInstall(done)
}

// installFailure is the one line an install that went wrong ends on, "" for
// one that went right: the installer's own last words when it printed any.
func installFailure(ended error, last string, waitErr error) string {
	switch {
	case errors.Is(ended, context.DeadlineExceeded):
		return fmt.Sprintf("install timed out after %s", installTimeout)
	case waitErr == nil:
		return ""
	case last != "":
		return clip(last, installErrorMax)
	}
	return waitErr.Error()
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.ToValidUTF8(s[:n], "") + "…"
}

func (s *Service) sendInstall(step Install) {
	s.mu.Lock()
	emit := s.emit
	s.mu.Unlock()
	if emit != nil {
		events.Send(emit, api.ProviderInstallEvent, step)
	}
}

// SetEmit wires where install progress goes: the event hub.
func (s *Service) SetEmit(emit func(name string, data any)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.emit = emit
}

func find(id string) (Provider, bool) {
	for _, p := range Registry {
		if p.ID == id {
			return p, true
		}
	}
	return Provider{}, false
}

func nameOf(id string) string {
	p, _ := find(id)
	return p.Name
}
