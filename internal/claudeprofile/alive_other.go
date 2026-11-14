//go:build !windows

package claudeprofile

import (
	"errors"
	"syscall"
)

// processAlive reports whether pid is a running process. One owned by another
// user answers EPERM, and is running.
func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
