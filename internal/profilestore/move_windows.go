package profilestore

import (
	"errors"

	"golang.org/x/sys/windows"
)

// crossDevice reports a rename refused for crossing volumes. os.Rename is
// MoveFileEx without MOVEFILE_COPY_ALLOWED, which answers
// ERROR_NOT_SAME_DEVICE there; syscall.EXDEV on Windows is an errno Go
// invented that no call returns.
func crossDevice(err error) bool { return errors.Is(err, windows.ERROR_NOT_SAME_DEVICE) }
