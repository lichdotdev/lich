//go:build windows

package restart

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// detached prepares exe to run detached from this console and process group so
// it outlives the restarting process, including an installer that must wait for
// this process to release its executable.
func detached(exe string, env, args []string) *exec.Cmd {
	cmd := exec.Command(exe, args...)
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
	}
	return cmd
}
