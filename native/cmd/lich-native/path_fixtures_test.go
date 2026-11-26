package main

import (
	"os"
	"path/filepath"
)

func nativeTestPath(path string) string {
	return filepath.VolumeName(os.TempDir()) + filepath.FromSlash(path)
}
