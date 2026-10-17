package restart

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/lichdotdev/lich/internal/logging"
)

// memorySampleEvery is how often WatchNative reads the running window's memory:
// the reading logged with an exit is at most this old, and a read of
// /proc/<pid>/status this rarely costs nothing.
const memorySampleEvery = 10 * time.Second

// StartDetached launches exe detached, so it outlives this process. stdio is
// left nil: the successor logs to its own file (main's logging.Init), like any
// lich launch.
func StartDetached(exe string, env, args []string) error {
	return detached(exe, env, args).Start()
}

// StartNative launches the native window exe detached, its stdout and stderr
// appended to the native log under logDir (logging.OpenNative): detached, it has
// no terminal, and what it prints would otherwise go nowhere.
func StartNative(exe string, env []string, logDir string) (*exec.Cmd, error) {
	out, err := logging.OpenNative(logDir)
	if err != nil {
		return nil, fmt.Errorf("open the native window's log: %w", err)
	}
	defer out.Close()
	cmd := detached(exe, env, nil)
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

// WatchNative waits for the native window cmd, already started, to exit and
// logs how it ended, an exit code or the signal that killed it, with the last
// memory reading taken while it ran (Linux only). Waiting also reaps it: an
// exited window no one waits on stays a zombie under the backend.
func WatchNative(cmd *exec.Cmd) {
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	pid := cmd.Process.Pid
	var last memory
	tick := time.NewTicker(memorySampleEvery)
	defer tick.Stop()
	for {
		select {
		case err := <-waited:
			logNativeExit(pid, cmd.ProcessState, err, last)
			return
		case <-tick.C:
			if m, ok := readMemory(pid); ok {
				last = m
			}
		}
	}
}

func logNativeExit(pid int, state *os.ProcessState, err error, last memory) {
	attrs := []any{"pid", pid, "vm_rss", last.rss, "vm_hwm", last.hwm}
	if state == nil {
		slog.Error("native window: wait failed", append(attrs, "err", err)...)
		return
	}
	attrs = append(attrs, "state", state.String(), "code", state.ExitCode())
	if state.Success() {
		slog.Info("native window exited", attrs...)
		return
	}
	slog.Warn("native window exited", attrs...)
}
