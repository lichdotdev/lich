package main

import (
	"path/filepath"
	"runtime"
)

// osPath is a fixture's absolute slash path as this system writes one:
// /repo/a.go, or C:\repo\a.go on Windows, where a path needs a drive to be
// absolute and the backend reports it with backslashes.
func osPath(slash string) string {
	if runtime.GOOS == "windows" {
		return "C:" + filepath.FromSlash(slash)
	}
	return slash
}
