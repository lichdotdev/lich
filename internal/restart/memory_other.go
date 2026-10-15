//go:build !linux

package restart

// readMemory has no /proc to read here.
func readMemory(int) (memory, bool) { return memory{}, false }
