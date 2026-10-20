// Package restart owns how a running lich ends. The backend outlives its
// window; it ends only when stopped, by `lich quit`, a signal, or a restart: a
// detached successor is started and this process is stopped, freeing the
// pinned listener port for the successor to bind. The restart is what the POST /restart endpoint drives after install.sh
// replaces the binary on disk.
package restart

import (
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
	// windowMoves is whether the open window reconnects to the successor
	// rather than closing with this lich.
	windowMoves bool
	exePath     string
	env         []string
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

// WindowMovesToSuccessor reports whether the open window stays to reconnect
// to a successor once lich is stopping, rather than closing with it: true only
// after a restart in place (Do) launched one.
func (c *Coordinator) WindowMovesToSuccessor() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.started && c.windowMoves
}

// Do launches the successor and stops this lich. Order matters: the successor
// starts first and blocks retrying the pinned port; then this process exits,
// and the freed port lets the successor bind, where the native window reconnects.
func (c *Coordinator) Do() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	spawned, err := c.spawnOnce(c.exePath, successorArgs, successorEnv(c.env))
	if spawned {
		c.windowMoves = true
		c.stopLocked()
	}
	return err
}

// Install hands replacement and relaunch to Inno Setup. It waits for this PID
// to exit before touching files, so main's defers finish first. The window
// closes with this lich instead of reconnecting: the installer replaces
// lich-native.exe too, which Windows will not overwrite while it runs, and the
// lich it launches again opens a window of its own.
func (c *Coordinator) Install(installer string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	spawned, err := c.spawnOnce(installer, installerArgs(filepath.Dir(c.exePath), os.Getpid()), successorEnv(c.env))
	if spawned {
		c.stopLocked()
	}
	return err
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

// spawnOnce launches the successor and reports whether this call did; the
// caller holds c.mu across it, so concurrent calls cannot both slip past the
// check. Once only: a second /restart (two install.sh runs) must not spawn a
// second successor that would then lose the port race and burn the bind
// timeout. The latch is set only after a successful spawn — a failed launch
// (say, the exe mid-swap by the package manager) must leave /restart
// retryable, not silently dead.
func (c *Coordinator) spawnOnce(exe string, args, env []string) (bool, error) {
	if c.exePath == "" {
		return false, errors.New("restart: executable path unknown")
	}
	if c.started {
		return false, nil
	}
	if err := c.spawn(exe, env, args); err != nil {
		return false, fmt.Errorf("restart: launch successor: %w", err)
	}
	c.started = true
	return true, nil
}

// stopLocked ends this lich once a successor is up; the caller holds c.mu.
func (c *Coordinator) stopLocked() {
	if c.stop != nil {
		c.stop()
	}
}

// successorEnv is env plus the wait marker, on a fresh slice.
func successorEnv(env []string) []string {
	return append(append([]string(nil), env...), WaitEnv+"=1")
}

// WithoutMarker is env minus the wait marker, on a fresh slice. The marker is
// for this process's own startup alone: what it spawns must not read as a
// restart successor, or a lich launched from one of its sessions never looks
// for the running one and dies on the busy port.
func WithoutMarker(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if !strings.HasPrefix(kv, WaitEnv+"=") {
			out = append(out, kv)
		}
	}
	return out
}
