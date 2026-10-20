package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lichdotdev/lich/internal/restart"
)

// nativeStartWait bounds waiting for a lich started here to answer. Most of it
// is the workspace database opening and the login shell resolving.
const nativeStartWait = 15 * time.Second

// native opens the native window on the running lich, starting one without a
// window when none runs. Both endpoints the window lives on take one client at
// a time, so a native window already open gives way to the newest.
func (c *client) native(args []string) error {
	flags := newFlagSet("native")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return usageError("native")
	}
	window, err := c.nativeWindow()
	if err != nil {
		return err
	}
	port, token, err := c.backend()
	if err != nil {
		return err
	}
	env := append(os.Environ(), "LICH_PORT="+port, "LICH_TOKEN="+token)
	if err := c.startNative(window, env); err != nil {
		return fmt.Errorf("launch %s: %w", window, err)
	}
	fmt.Fprintf(c.stdout, "Opening the native window on lich (port %s).\n", port)
	return nil
}

// nativeWindow is the native window to launch: the pinned one, else the one
// beside this lich binary. A missing one is reported before anything starts.
func (c *client) nativeWindow() (string, error) {
	return restart.NativeExecutable(c.env)
}

// backend is the running lich's coordinates, starting lich in the background
// with no window when nothing answers on them.
func (c *client) backend() (string, string, error) {
	port, token, err := c.coordinates()
	if err == nil && listening(port) {
		return port, token, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", "", fmt.Errorf("find the lich binary: %w", err)
	}
	if err := c.start(exe, os.Environ(), []string{restart.NoWindowArg}); err != nil {
		return "", "", fmt.Errorf("start lich: %w", err)
	}
	return c.awaitBackend(nativeStartWait)
}

// awaitBackend waits up to wait for a lich just started to write its runtime
// file and listen on the port it names.
func (c *client) awaitBackend(wait time.Duration) (string, string, error) {
	deadline := time.Now().Add(wait)
	for {
		port, token, err := c.coordinates()
		if err == nil && listening(port) {
			return port, token, nil
		}
		if time.Now().After(deadline) {
			return "", "", errors.New("lich was started and did not answer within " + wait.String() +
				"; its log says why (lich doctor walks the boot)")
		}
		time.Sleep(quitPoll)
	}
}

// start launches exe detached, so it outlives this command; nil c.launch takes
// the real one.
func (c *client) start(exe string, env, args []string) error {
	if c.launch != nil {
		return c.launch(exe, env, args)
	}
	return restart.StartDetached(exe, env, args)
}

// startNative launches the native window detached, printing into the native
// log beside lich's; nil c.launch takes the real one. Nothing here waits on it:
// this command exits right after.
func (c *client) startNative(exe string, env []string) error {
	if c.launch != nil {
		return c.launch(exe, env, nil)
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("resolve config directory: %w", err)
	}
	_, err = restart.StartNative(exe, env, filepath.Join(dir, "lich"))
	return err
}
