package extensions

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/lichdotdev/lich/extension"
	"github.com/lichdotdev/lich/internal/winexec"
)

// supervise runs one extension until lich stops or gives up on it. Every exit
// lich did not ask for counts as a crash, a clean one included: an extension
// is meant to live as long as lich does.
func (s *Service) supervise(ctx context.Context, x *installed) {
	quick := 0
	for {
		run, err := s.start(ctx, x)
		if ctx.Err() != nil && err != nil {
			s.setStatus(x, statusStopped, "")
			return
		}
		if err != nil {
			s.setStatus(x, statusFailed, err.Error())
			return
		}
		started := time.Now()
		s.setStatus(x, statusRunning, "")
		exit := run.wait()
		s.detachQueue(x)
		x.renders.reset()
		if ctx.Err() != nil {
			s.setStatus(x, statusStopped, "")
			return
		}
		if time.Since(started) >= s.timing.stableRun {
			quick = 0
		}
		quick++
		reason := exitText(exit)
		slog.Warn("extensions: exited", "extension", x.id, "reason", reason, "quickCrashes", quick)
		if quick >= maxQuickCrashes {
			s.setStatus(x, statusFailed, fmt.Sprintf("crashed %d times in a row, last: %s", quick, reason))
			return
		}
		s.setStatus(x, statusRestarting, reason)
		if !s.sleep(ctx, backoff(quick, s.timing.firstBackoff, s.timing.maxBackoff)) {
			s.setStatus(x, statusStopped, "")
			return
		}
	}
}

func exitText(err error) string {
	if err == nil {
		return "exited on its own with status 0"
	}
	return err.Error()
}

// backoff is the wait before restart number quick of a crash streak.
func backoff(quick int, first, ceiling time.Duration) time.Duration {
	wait := first << (quick - 1)
	return min(wait, ceiling)
}

// sleep waits for d and reports false if ctx ended meanwhile.
func (s *Service) sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func (s *Service) detachQueue(x *installed) {
	s.mu.Lock()
	x.queue, x.subscribed = nil, nil
	s.mu.Unlock()
}

// process is one run of an extension.
type process struct {
	id         string
	cmd        *exec.Cmd
	tree       tree
	stopWriter func()
}

// start launches x. Stdout and stderr are writers rather than pipes read here
// so cmd.Wait, bounded by WaitDelay, deals with a descendant that keeps them
// open after the process exits. cmd.Cancel closes stdin and WaitDelay is the
// kill after it; what the extension started ends once it is gone. A failure to
// start is returned, not retried: a missing or non-executable file stays that
// way.
func (s *Service) start(ctx context.Context, x *installed) (*process, error) {
	data := filepath.Join(s.dataDir, x.id)
	if err := os.MkdirAll(data, 0o700); err != nil {
		return nil, fmt.Errorf("could not create its data directory: %w", err)
	}
	cmd := exec.CommandContext(ctx, x.exe)
	cmd.Dir = x.root
	cmd.Env = append(slices.Clip(s.env), extension.DataDirEnv+"="+data, extension.LanguageEnv+"="+s.startLanguage())
	winexec.Hide(cmd)
	ownGroup(cmd)
	cmd.Stdout = newFramer(maxLineBytes,
		func(line []byte) { s.handleLine(x, line) },
		func() { s.refuse(x, nil, fmt.Sprintf("line over %d bytes", maxLineBytes)) })
	cmd.Stderr = newFramer(maxLogLineBytes,
		func(line []byte) { x.stderr.note(x.id, msgStderr, string(line)) },
		func() {
			x.stderr.note(x.id, msgStderr, fmt.Sprintf("a line over %d bytes was dropped", maxLogLineBytes))
		})
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("could not start: %w", err)
	}
	queue := make(chan []byte, sendQueue)
	quit := make(chan struct{})
	stopWriter := sync.OnceFunc(func() { close(quit) })
	cmd.Cancel = func() error {
		stopWriter()
		return nil
	}
	cmd.WaitDelay = s.timing.stopGrace
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("could not start: %w", err)
	}
	tree, err := adopt(cmd.Process)
	if err != nil {
		slog.Warn("extensions: what it starts can outlive it", "extension", x.id, "err", err)
	}
	go writeLoop(stdin, queue, quit, x.id)
	s.mu.Lock()
	x.queue = queue
	s.mu.Unlock()
	return &process{id: x.id, cmd: cmd, tree: tree, stopWriter: stopWriter}, nil
}

func (p *process) wait() error {
	err := p.cmd.Wait()
	p.stopWriter()
	if endErr := p.tree.end(); endErr != nil {
		slog.Warn("extensions: end what it started", "extension", p.id, "err", endErr)
	}
	return err
}

// writeLoop owns stdin, so a Write stuck on an extension that stopped reading
// blocks only this goroutine; the kill after the grace ends it.
func writeLoop(stdin io.WriteCloser, queue <-chan []byte, quit <-chan struct{}, id string) {
	defer stdin.Close()
	for {
		select {
		case line := <-queue:
			if _, err := stdin.Write(line); err != nil {
				slog.Warn("extensions: write to stdin", "extension", id, "err", err)
				return
			}
		case <-quit:
			return
		}
	}
}
