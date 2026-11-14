package claudeprofile

import (
	"errors"

	"golang.org/x/sys/windows"
)

// processAlive reports whether pid is a running process. Windows answers
// ERROR_INVALID_PARAMETER for a pid no process holds; any other failure to
// open it, a process of another user's say, counts as running.
func processAlive(pid int) bool {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return !errors.Is(err, windows.ERROR_INVALID_PARAMETER)
	}
	defer func() { _ = windows.CloseHandle(h) }()
	event, err := windows.WaitForSingleObject(h, 0)
	return err != nil || event == uint32(windows.WAIT_TIMEOUT)
}
