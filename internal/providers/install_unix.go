//go:build !windows

package providers

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// tree is the session an installer leads: Setsid takes it off any terminal,
// so sudo has none to ask on, and what it starts stays in its process group.
type tree struct{}

func ownSession(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
}

func adopt(*os.Process) tree { return tree{} }

// end kills the group; a group keeps its id while any member lives, so after
// the leader is reaped it still names what the installer left behind.
func (tree) end(p *os.Process) error {
	if p == nil {
		return nil
	}
	err := syscall.Kill(-p.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

func asRoot() bool { return os.Geteuid() == 0 }
