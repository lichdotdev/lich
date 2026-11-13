//go:build !windows

package extensions

// afterExit runs op, which moves or deletes the directory of an extension
// whose process has exited; nothing holds it once Wait returns.
func afterExit(op func() error) error { return op() }
