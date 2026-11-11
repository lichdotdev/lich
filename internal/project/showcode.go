package project

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/relpath"
)

// maxCodeComment keeps the block under the lines readable at a glance, without
// pushing the code off the panel.
const maxCodeComment = 360

// SetSessionCheckout wires the directory a live session runs in, "" for one
// that is gone. Set once at startup.
func (s *Service) SetSessionCheckout(checkout func(id string) string) { s.sessionCheckout = checkout }

// ShowCode refuses what the preview could not show, so the agent hears it and
// corrects itself instead of the person finding an error in the panel.
func (s *Service) ShowCode(ctx context.Context, opts api.ShowCodeOptions) (api.ShowCode, error) {
	dir := ""
	if s.sessionCheckout != nil {
		dir = s.sessionCheckout(opts.ID)
	}
	if dir == "" {
		return api.ShowCode{}, fmt.Errorf("session %q is not running", opts.ID)
	}
	rel, err := checkoutRel(dir, opts.Path)
	if err != nil {
		return api.ShowCode{}, err
	}
	text, err := s.ReadFile(ctx, api.CheckoutFileOptions{Path: dir, Rel: rel})
	if errors.Is(err, fs.ErrNotExist) {
		return api.ShowCode{}, fmt.Errorf("no such file in this checkout: %s", rel)
	}
	if err != nil {
		return api.ShowCode{}, err
	}
	if err := checkLineRange(rel, text, opts.Line, opts.EndLine); err != nil {
		return api.ShowCode{}, err
	}
	comment, err := cleanCodeComment(opts.Comment, opts.Line)
	if err != nil {
		return api.ShowCode{}, err
	}
	shown := api.ShowCode{SessionID: opts.ID, Rel: rel, Line: opts.Line, EndLine: opts.EndLine, Comment: comment}
	if s.emit != nil {
		events.Send(s.emit, api.ShowCodeEvent, shown)
	}
	return shown, nil
}

// checkoutRel is path relative to dir, with both sides' links resolved when
// path is absolute: a worktree is routinely reached through a link, and
// comparing one resolved side would call every file outside.
func checkoutRel(dir, path string) (string, error) {
	rel := path
	if filepath.IsAbs(path) {
		root, err := filepath.EvalSymlinks(dir)
		if err != nil {
			return "", fmt.Errorf("resolve checkout %s: %w", dir, err)
		}
		full, err := filepath.EvalSymlinks(path)
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("no such file in this checkout: %s", path)
		}
		if err != nil {
			return "", fmt.Errorf("resolve %s: %w", path, err)
		}
		if rel, err = filepath.Rel(root, full); err != nil {
			return "", fmt.Errorf("outside this session's checkout: %s", path)
		}
	}
	rel = filepath.Clean(rel)
	if relpath.Escapes(rel) {
		return "", fmt.Errorf("outside this session's checkout: %s", path)
	}
	return filepath.ToSlash(rel), nil
}

// cleanCodeComment trims raw, whitespace alone counting as no comment, and
// turns what the interface fonts cannot draw into what they can.
func cleanCodeComment(raw string, line int) (string, error) {
	comment := strings.TrimSpace(raw)
	if comment == "" {
		return "", nil
	}
	if line == 0 {
		return "", errors.New("comment needs line")
	}
	if n := utf8.RuneCountInString(comment); n > maxCodeComment {
		return "", fmt.Errorf("comment is %d characters; the limit is %d", n, maxCodeComment)
	}
	return strings.NewReplacer("\r\n", "\n", "\t", "    ").Replace(comment), nil
}

func checkLineRange(rel, text string, line, end int) error {
	switch {
	case line < 0 || end < 0:
		return errors.New("line numbers start at 1")
	case end > 0 && line == 0:
		return errors.New("end_line needs line")
	case end > 0 && end < line:
		return fmt.Errorf("end_line %d is before line %d", end, line)
	}
	if last, count := max(line, end), lineCount(text); last > count {
		return fmt.Errorf("line %d is past the end of %s (%d lines)", last, rel, count)
	}
	return nil
}

// lineCount counts as sliceLines does: a trailing newline ends the last line
// and does not open another.
func lineCount(text string) int {
	if text == "" {
		return 0
	}
	return strings.Count(strings.TrimSuffix(text, "\n"), "\n") + 1
}
