//go:build !windows

package profilestore

import (
	"errors"
	"os"
	"syscall"
)

// crossDevice reports a rename refused for crossing filesystems.
func crossDevice(err error) bool { return errors.Is(err, syscall.EXDEV) }

// replaceFile puts from in place of to; rename(2) replaces atomically.
func replaceFile(from, to string) error { return os.Rename(from, to) }
