package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// maxSameName bounds the "name (n).ext" tried for one download, as Chrome
// stops at 100 (DownloadPathReservationTracker's kMaxUniqueFiles).
const maxSameName = 100

// reserve makes an empty file in dir for a download the server named
// suggested, under that name or Chrome's "name (1).ext" where it is taken,
// and answers its path. A name with a path in it keeps only its last part.
func reserve(dir, suggested string) (string, error) {
	name := filepath.Base(filepath.Clean("/" + suggested))
	if name == "/" || name == "." {
		name = "download"
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for n := range maxSameName {
		try := name
		if n > 0 {
			try = fmt.Sprintf("%s (%d)%s", stem, n, ext)
		}
		path := filepath.Join(dir, try)
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		return path, f.Close()
	}
	return "", fmt.Errorf("%s: %d files of that name already", name, maxSameName)
}
