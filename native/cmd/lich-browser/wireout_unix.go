//go:build linux || darwin

package main

import (
	"os"
	"syscall"
)

// wireOut is stdout for the wire alone: fd 1 then points at stderr, so a
// library that prints to stdout lands in the log and never in the middle of a
// line the backend reads.
func wireOut() (*os.File, error) {
	fd, err := syscall.Dup(1)
	if err != nil {
		return nil, err
	}
	if err := stderrOnto(1); err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), "wire"), nil
}
