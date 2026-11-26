// Package winretry runs a file operation that Windows can refuse for a moment
// after the process holding the file has exited.
package winretry

import (
	"errors"
	"time"

	"golang.org/x/sys/windows"
)

// retryFor bounds how long Do retries, the 2s Go's own
// cmd/go/internal/robustio gives the same errors.
const retryFor = 2 * time.Second

// Do runs op, which moves or deletes files a process may have just let go of.
// Windows can still hold them for a moment after that process exited, or while
// a helper of it lingers, and refuses op meanwhile, so op is retried the way
// robustio retries it.
func Do(op func() error) error {
	deadline := time.Now().Add(retryFor)
	for wait := time.Millisecond; ; wait = min(2*wait, 100*time.Millisecond) {
		err := op()
		if err == nil || !held(err) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(wait)
	}
}

func held(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}
