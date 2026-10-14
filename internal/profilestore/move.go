package profilestore

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ErrNoConversation is MoveConversation finding the conversation in no
// profile: a session that has its id but has not written a transcript yet.
var ErrNoConversation = errors.New("conversation not found in any profile")

// Owner is the profile holding a conversation, and the path of its transcript
// there. locate finds the transcript under one state directory. A conversation
// id is unique to its provider and lives in exactly one profile, so the first
// match is the only one.
func (s Spec) Owner(conversationID string, locate func(dir, conversationID string) (string, bool)) (Profile, string, bool) {
	for _, p := range s.All() {
		if path, ok := locate(p.Dir, conversationID); ok {
			return p, path, true
		}
	}
	return Profile{}, "", false
}

// Pair is one entry of a conversation moving between profiles: where it is and
// where it goes.
type Pair struct{ From, To string }

// MoveConversation moves a conversation from the profile that holds it to
// toSlug, and back by calling it again. locate finds its transcript under one
// state directory; entries lists everything the provider keyed by the
// conversation's id, given that transcript and the two directories. The move
// refuses a target that already holds any entry.
//
// A failure part way moves back whatever had already moved, so the conversation
// is never split across two profiles.
func (s Spec) MoveConversation(
	conversationID, toSlug string,
	locate func(dir, conversationID string) (string, bool),
	entries func(conversationID, transcript, fromDir, toDir string) ([]Pair, error),
) error {
	from, transcript, ok := s.Owner(conversationID, locate)
	if !ok {
		return fmt.Errorf("no %s conversation %q: %w", s.Label, conversationID, ErrNoConversation)
	}
	toDir, err := s.ConfigDir(toSlug)
	if err != nil {
		return err
	}
	if filepath.Clean(from.Dir) == filepath.Clean(toDir) {
		return fmt.Errorf("conversation %q is already in profile %q", conversationID, toSlug)
	}
	pairs, err := entries(conversationID, transcript, from.Dir, toDir)
	if err != nil {
		return err
	}
	for _, p := range pairs {
		if _, err := os.Lstat(p.To); err == nil {
			return fmt.Errorf("profile %q already holds %s", toSlug, p.To)
		}
	}
	for i, p := range pairs {
		if err := move(p.From, p.To); err != nil {
			for _, done := range pairs[:i] {
				_ = move(done.To, done.From)
			}
			return fmt.Errorf("move %s: %w", p.From, err)
		}
	}
	return nil
}

// move renames from to to, creating to's parent, and copies across devices,
// where a rename cannot go: Personal's directory and lich's own may sit on
// different filesystems, or volumes on Windows.
func move(from, to string) error {
	if err := os.MkdirAll(filepath.Dir(to), 0o700); err != nil {
		return err
	}
	err := os.Rename(from, to)
	var linkErr *os.LinkError
	if err == nil || !errors.As(err, &linkErr) || !crossDevice(linkErr.Err) {
		return err
	}
	if err := copyTree(from, to); err != nil {
		_ = os.RemoveAll(to)
		return err
	}
	return os.RemoveAll(from)
}

// copyTree copies a tree keeping each entry's permission bits, and recreates a
// link as a link to the same target rather than copying what it points at.
func copyTree(from, to string) error {
	type dirMode struct {
		path string
		perm os.FileMode
	}
	var dirs []dirMode
	err := filepath.WalkDir(from, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(to, rel)
		if IsLink(path) {
			return copyLink(path, dst)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			dirs = append(dirs, dirMode{dst, info.Mode().Perm()})
			return os.MkdirAll(dst, 0o700)
		}
		if err := copyFile(path, dst); err != nil {
			return err
		}
		return os.Chmod(dst, info.Mode().Perm())
	})
	if err != nil {
		return err
	}
	// After the walk: a read-only directory would refuse its own children.
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := os.Chmod(dirs[i].path, dirs[i].perm); err != nil {
			return err
		}
	}
	return nil
}

func copyLink(src, dst string) error {
	target, err := os.Readlink(src)
	if err != nil {
		return err
	}
	if info, err := os.Stat(src); err == nil && info.IsDir() {
		return LinkDir(target, dst)
	}
	_, err = linkFile(target, dst)
	return err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
