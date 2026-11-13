//go:build !windows

package extensions

import "os"

// renameExited moves the directory of an extension whose process has exited;
// nothing holds it once Wait returns.
func renameExited(from, to string) error { return os.Rename(from, to) }
