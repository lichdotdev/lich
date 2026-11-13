package extensions

import (
	"errors"
	"time"

	"golang.org/x/sys/windows"
)

// exitedRetryFor bounds how long afterExit retries, the 2s Go's own
// cmd/go/internal/robustio gives the same errors.
const exitedRetryFor = 2 * time.Second

// afterExit runs op, which moves or deletes the directory of an extension
// whose process has just exited. Windows can still hold its executable for a
// moment after Wait returns and refuses op meanwhile, so op is retried the way
// robustio retries it.
func afterExit(op func() error) error {
	deadline := time.Now().Add(exitedRetryFor)
	for wait := time.Millisecond; ; wait = min(2*wait, 100*time.Millisecond) {
		err := op()
		if err == nil || !stillHeld(err) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(wait)
	}
}

func stillHeld(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}
