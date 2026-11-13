package extensions

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// renameRetryFor bounds how long renameExited retries, the 2s Go's own
// cmd/go/internal/robustio gives the same errors.
const renameRetryFor = 2 * time.Second

// renameExited moves the directory of an extension whose process has just
// exited. Windows can still hold its executable for a moment after Wait
// returns, and MoveFileEx refuses a directory with a file in it held, so the
// rename is retried the way robustio retries it.
func renameExited(from, to string) error {
	deadline := time.Now().Add(renameRetryFor)
	for wait := time.Millisecond; ; wait = min(2*wait, 100*time.Millisecond) {
		err := os.Rename(from, to)
		if err == nil || !renameBusy(err) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(wait)
	}
}

func renameBusy(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}
