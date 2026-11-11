package extensions

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	// maxUnpackedBytes and maxArchiveEntries bound what an archive expands to,
	// so a small download cannot fill the disk.
	maxUnpackedBytes  = 1 << 30
	maxArchiveEntries = 1000
)

// extract unpacks a .tar.gz into dest, which it creates. Only directories and
// regular files are taken, and every name must stay inside dest: with no link
// ever created, the lexical check is the whole of it.
func extract(r io.Reader, dest string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("not a gzip archive: %w", err)
	}
	if err := os.Mkdir(dest, 0o755); err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	budget := int64(maxUnpackedBytes)
	for entries := 0; ; entries++ {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("not a tar archive: %w", err)
		}
		if entries >= maxArchiveEntries {
			return fmt.Errorf("more than %d entries", maxArchiveEntries)
		}
		if budget, err = extractEntry(tr, h, dest, budget); err != nil {
			return err
		}
	}
}

// extractEntry writes one entry and returns what is left of the size budget.
func extractEntry(tr *tar.Reader, h *tar.Header, dest string, budget int64) (int64, error) {
	if h.Typeflag == tar.TypeXGlobalHeader {
		return budget, nil
	}
	local := filepath.FromSlash(h.Name)
	if !filepath.IsLocal(local) {
		return budget, fmt.Errorf("entry %q leaves the extension's directory", h.Name)
	}
	target := filepath.Join(dest, local)
	switch h.Typeflag {
	case tar.TypeDir:
		return budget, os.MkdirAll(target, 0o755)
	case tar.TypeReg:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return budget, err
		}
		return writeEntry(tr, h.Name, target, budget)
	default:
		return budget, fmt.Errorf("entry %q is not a regular file or a directory; links are not accepted", h.Name)
	}
}

func writeEntry(r io.Reader, name, target string, budget int64) (int64, error) {
	f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return budget, fmt.Errorf("entry %q: %w", name, err)
	}
	n, copyErr := io.Copy(f, io.LimitReader(r, budget+1))
	if err := errors.Join(copyErr, f.Close()); err != nil {
		return budget, fmt.Errorf("entry %q: %w", name, err)
	}
	if n > budget {
		return budget, fmt.Errorf("unpacks to more than %d MiB", maxUnpackedBytes>>20)
	}
	return budget - n, nil
}
