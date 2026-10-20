// Package restart owns how a running lich ends. The backend outlives its
// window; it ends only when stopped, by `lich quit`, a signal, or a restart: a
// detached successor is started and this process is stopped, freeing the
// pinned listener port for the successor to bind. The restart is what the POST /restart endpoint drives after install.sh
// replaces the binary on disk.
package restart

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// WaitEnv marks a lich process spawned to succeed a restarting one. The
// listener bind path retries while this is set, because the outgoing process
// still holds the pinned port for a moment (see internal/terminal transport).
const WaitEnv = "LICH_RESTART_WAIT"

// ArgsEnv carries a lich's arguments, as a JSON list, to the lich Inno Setup
// launches again after an update on Windows. That launch has a fixed command
// line (build/windows/lich.iss) and the environment of the lich that started
// the installer, so its arguments ride the environment (LaunchArgs).
const ArgsEnv = "LICH_RELAUNCH_ARGS"

// NoWindowArg starts lich without its window.
const NoWindowArg = "--no-window"

// successorArgs start a successor without a window: the native window open on
// this backend reconnects to the successor, and a second one opening there
// would take its sockets.
var successorArgs = []string{NoWindowArg}

// Coordinator ends this lich: on its own (Quit), or after launching a detached
// successor (Do, Install), so this process unwinds, exits, and frees the port.
type Coordinator struct {
	mu      sync.Mutex
	stop    func()
	started bool
	exePath string
	env     []string
	// spawn is the seam for tests; it defaults to the build-tagged primitive.
	spawn func(exe string, env, args []string) error
}

// New returns a coordinator that relaunches exePath with env (plus the wait
// marker). env should be the current process environment so the successor
// pins the same listener port.
func New(exePath string, env []string) *Coordinator {
	return &Coordinator{
		exePath: exePath,
		env:     env,
		spawn:   StartDetached,
	}
}

// SetStop supplies the clean exit: stop makes main unwind its
// defers and return. Called once lich is serving; a restart before that only
// spawns the successor.
func (c *Coordinator) SetStop(stop func()) {
	c.mu.Lock()
	c.stop = stop
	c.mu.Unlock()
}

// Quit ends this lich: every session goes with it, and nothing is launched in
// its place.
func (c *Coordinator) Quit() error {
	c.mu.Lock()
	stop := c.stop
	c.mu.Unlock()
	if stop == nil {
		return errors.New("quit: lich is still starting")
	}
	stop()
	return nil
}

// Relaunching reports whether a successor was launched to take this lich's
// place, which is what tells a restart from a quit once lich is stopping.
func (c *Coordinator) Relaunching() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.started
}

// Do launches the successor and stops this lich. Order matters: the successor
// starts first and blocks retrying the pinned port; then this process exits,
// and the freed port lets the successor bind, where the native window reconnects.
func (c *Coordinator) Do() error {
	return c.launch(c.exePath, successorArgs, successorEnv(c.env))
}

// Install hands replacement and relaunch to Inno Setup. It waits for this PID
// to exit before touching files, so the window and main's defers finish first.
func (c *Coordinator) Install(installer string) error {
	env, err := relaunchEnv(c.env, successorArgs)
	if err != nil {
		return err
	}
	return c.launch(installer, installerArgs(filepath.Dir(c.exePath), os.Getpid()), env)
}

// relaunchEnv is the installer's environment: the successor's, plus args in
// ArgsEnv for the lich it launches again.
func relaunchEnv(env, args []string) ([]string, error) {
	out := successorEnv(env)
	encoded, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("restart: encode the relaunch arguments: %w", err)
	}
	return append(out, ArgsEnv+"="+string(encoded)), nil
}

// LaunchArgs are the arguments this lich runs with: its own argv when it has
// one, and otherwise the ones a lich updating on Windows handed it through
// ArgsEnv. An ArgsEnv that is not a JSON list of strings is an error.
func LaunchArgs(argv []string, getenv func(string) string) ([]string, error) {
	if len(argv) > 0 {
		return argv, nil
	}
	handed := getenv(ArgsEnv)
	if handed == "" {
		return nil, nil
	}
	var args []string
	if err := json.Unmarshal([]byte(handed), &args); err != nil {
		return nil, fmt.Errorf("read %s: %w", ArgsEnv, err)
	}
	return args, nil
}

// installerArgs is half of a contract with build/windows/lich.iss. /UPDATEPID is
// no Inno switch: the script reads it back as {param:UPDATEPID|0} to tell an
// update from a fresh install and to wait for this process to release its exe.
// Rename it on one side only and the installer stops waiting, silently.
func installerArgs(dir string, pid int) []string {
	return []string{"/SILENT", "/NORESTART", "/CLOSEAPPLICATIONS", "/NORESTARTAPPLICATIONS",
		"/DIR=" + dir, "/UPDATEPID=" + strconv.Itoa(pid),
		"/LOG=" + filepath.Join(dir, ".lich-update-setup.log")}
}

func (c *Coordinator) launch(exe string, args, env []string) error {
	if c.exePath == "" {
		return errors.New("restart: executable path unknown")
	}
	// Once only: a second /restart (two install.sh runs) must not spawn a second
	// successor that would then lose the port race and burn the bind timeout.
	// The latch is set only after a successful spawn — a failed launch (say,
	// the exe mid-swap by the package manager) must leave /restart retryable,
	// not silently dead. The lock spans the spawn so concurrent calls cannot
	// both slip past the check.
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started {
		return nil
	}
	if err := c.spawn(exe, env, args); err != nil {
		return fmt.Errorf("restart: launch successor: %w", err)
	}
	c.started = true
	if c.stop != nil {
		c.stop()
	}
	return nil
}

// successorEnv is env plus the wait marker, on a fresh slice.
func successorEnv(env []string) []string {
	return append(append([]string(nil), env...), WaitEnv+"=1")
}

// WithoutMarker is env minus the wait marker and ArgsEnv, on a fresh slice.
// Both are for this process's own startup alone: what it spawns must not read
// as a restart successor, or a lich launched from one of its sessions never
// looks for the running one and dies on the busy port, and it must not take
// this lich's arguments for its own.
func WithoutMarker(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if !strings.HasPrefix(kv, WaitEnv+"=") && !strings.HasPrefix(kv, ArgsEnv+"=") {
			out = append(out, kv)
		}
	}
	return out
}
