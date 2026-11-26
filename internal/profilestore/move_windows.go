package profilestore

import (
	"errors"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/windows"

	"github.com/lichdotdev/lich/internal/winretry"
)

// CrossDevice reports a rename refused for crossing volumes. os.Rename is
// MoveFileEx without MOVEFILE_COPY_ALLOWED, which answers
// ERROR_NOT_SAME_DEVICE there; syscall.EXDEV on Windows is an errno Go
// invented that no call returns.
func CrossDevice(err error) bool { return errors.Is(err, windows.ERROR_NOT_SAME_DEVICE) }

// replaceLocks holds one mutex per target path, so renames onto one file from
// this process take turns.
var replaceLocks sync.Map

// replaceFile puts from in place of to. MoveFileEx answers ACCESS_DENIED or
// SHARING_VIOLATION while another rename onto to, or a reader, still holds it.
// Renames from this process take turns per target: racing each other, sixteen
// writers on a loaded runner outlasted the shared deadline. The retry is left
// for other processes and readers.
func replaceFile(from, to string) error {
	mu, _ := replaceLocks.LoadOrStore(filepath.Clean(to), new(sync.Mutex))
	mu.(*sync.Mutex).Lock()
	defer mu.(*sync.Mutex).Unlock()
	return winretry.Do(func() error { return os.Rename(from, to) })
}
