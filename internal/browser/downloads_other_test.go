//go:build !windows

package browser

import "syscall"

// errCrossDevice is what rename answers for a file moving across filesystems.
var errCrossDevice error = syscall.EXDEV
