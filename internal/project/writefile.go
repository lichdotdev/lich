package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"unicode/utf8"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/relpath"
)

// WriteFile writes the editor's text over a file of the checkout. It resolves
// the path as ReadFile does, so a symlink out of the tree is refused rather
// than followed, and it only overwrites a regular file that is UTF-8 on disk:
// ReadFile hands any other text over decoded as Windows-1252, and writing that
// back would re-encode the whole file.
func (s *Service) WriteFile(_ context.Context, opts api.WriteFileOptions) (api.None, error) {
	full, err := relpath.Resolve(opts.Path, opts.Rel)
	if err != nil {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			return api.None{}, fmt.Errorf("stat %s: %w", opts.Rel, pathErr.Err)
		}
		return api.None{}, err
	}
	info, err := os.Stat(full)
	if err != nil {
		return api.None{}, fmt.Errorf("stat %s: %w", opts.Rel, err)
	}
	if !info.Mode().IsRegular() {
		return api.None{}, fmt.Errorf("%s is not a regular file", opts.Rel)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return api.None{}, fmt.Errorf("read %s: %w", opts.Rel, err)
	}
	if isBinary(data) || !utf8.Valid(data) {
		return api.None{}, fmt.Errorf("%s is not UTF-8 text, so lich does not write it", opts.Rel)
	}
	if err := os.WriteFile(full, []byte(opts.Content), info.Mode().Perm()); err != nil {
		return api.None{}, fmt.Errorf("write %s: %w", opts.Rel, err)
	}
	return api.None{}, nil
}
