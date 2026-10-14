//go:build !windows

package profilestore

import (
	"errors"
	"syscall"
)

// crossDevice reports a rename refused for crossing filesystems.
func crossDevice(err error) bool { return errors.Is(err, syscall.EXDEV) }
