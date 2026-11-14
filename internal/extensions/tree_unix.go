//go:build !windows

package extensions

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// tree is the process group an extension leads; what it starts joins it,
// unless that puts itself in a group of its own.
type tree struct{ pgid int }

func ownGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

func adopt(p *os.Process) (tree, error) { return tree{pgid: p.Pid}, nil }

// end runs after the leader is reaped: a group keeps its id while any member
// lives, so the id still names this group.
func (t tree) end() error {
	err := syscall.Kill(-t.pgid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
