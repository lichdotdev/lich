//go:build !windows

package restart

import (
	"os/exec"
	"syscall"
)

// detached prepares exe to run in its own session (setsid) so it outlives this
// process and the PTY that triggered the restart.
func detached(exe string, env, args []string) *exec.Cmd {
	cmd := exec.Command(exe, args...)
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd
}
