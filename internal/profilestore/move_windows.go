package profilestore

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// crossDevice reports a rename refused for crossing volumes. os.Rename is
// MoveFileEx without MOVEFILE_COPY_ALLOWED, which answers
// ERROR_NOT_SAME_DEVICE there; syscall.EXDEV on Windows is an errno Go
// invented that no call returns.
func crossDevice(err error) bool { return errors.Is(err, windows.ERROR_NOT_SAME_DEVICE) }

// replaceRetryFor bounds how long replaceFile retries, the 2s Go's own
// cmd/go/internal/robustio gives the same errors.
const replaceRetryFor = 2 * time.Second

// replaceFile puts from in place of to. MoveFileEx answers ACCESS_DENIED or
// SHARING_VIOLATION while another rename onto to, or a reader, still holds it,
// which two spawns writing one profile's files at once do; it is retried
// until that passes, the way robustio retries it.
func replaceFile(from, to string) error {
	deadline := time.Now().Add(replaceRetryFor)
	for wait := time.Millisecond; ; wait = min(2*wait, 100*time.Millisecond) {
		err := os.Rename(from, to)
		if err == nil || !replaceBusy(err) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(wait)
	}
}

func replaceBusy(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}
