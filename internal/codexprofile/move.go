package codexprofile

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// Owner is the profile holding a Codex conversation, and the path of its
// rollout there. A conversation id is a UUID and lives in exactly one profile,
// so the first match is the only one.
func Owner(conversationID string) (Profile, string, bool) {
	for _, p := range ConfigDirs() {
		if path, ok := RolloutIn(p.Dir, conversationID); ok {
			return p, path, true
		}
	}
	return Profile{}, "", false
}

// RolloutIn locates a conversation's rollout under one state directory. Codex
// files a rollout by the date it started,
// sessions/<yyyy>/<mm>/<dd>/rollout-<timestamp>-<id>.jsonl, so the date parts
// are globbed rather than reconstructed.
func RolloutIn(dir, conversationID string) (string, bool) {
	matches, err := filepath.Glob(filepath.Join(dir, "sessions", "*", "*", "*", "rollout-*-"+conversationID+".jsonl"))
	if err != nil || len(matches) == 0 {
		return "", false
	}
	return matches[0], true
}

// MoveConversation moves a Codex conversation from the profile that holds it to
// another: its rollout, and the images it generated (generated_images/<id>). The
// target's own thread database learns the conversation from the rollout the
// next time Codex reads it (measured on 0.162.1: a rollout alone in an empty
// home was found by id and indexed there).
//
// What stays behind is what is not keyed by the id: the prompt history
// (history.jsonl), and the source's thread database row: that database is
// Codex's, and lich leaves it alone.
//
// A failure part way moves back whatever had already moved, so the conversation
// is never split across two profiles.
func MoveConversation(conversationID, toSlug string) error {
	from, rollout, ok := Owner(conversationID)
	if !ok {
		return fmt.Errorf("no Codex conversation %q in any profile", conversationID)
	}
	toDir, err := ConfigDir(toSlug)
	if err != nil {
		return err
	}
	if filepath.Clean(from.Dir) == filepath.Clean(toDir) {
		return fmt.Errorf("conversation %q is already in profile %q", conversationID, toSlug)
	}
	rel, err := filepath.Rel(from.Dir, rollout)
	if err != nil {
		return err
	}
	pairs := []movePair{{rollout, filepath.Join(toDir, rel)}}
	images := filepath.Join("generated_images", conversationID)
	if _, err := os.Lstat(filepath.Join(from.Dir, images)); err == nil {
		pairs = append(pairs, movePair{filepath.Join(from.Dir, images), filepath.Join(toDir, images)})
	}
	for _, p := range pairs {
		if _, err := os.Lstat(p.to); err == nil {
			return fmt.Errorf("profile %q already holds %s", toSlug, p.to)
		}
	}
	for i, p := range pairs {
		if err := move(p.from, p.to); err != nil {
			for _, done := range pairs[:i] {
				_ = move(done.to, done.from)
			}
			return fmt.Errorf("move %s: %w", p.from, err)
		}
	}
	return nil
}

type movePair struct{ from, to string }

// move renames from to to, creating to's parent, and copies across devices,
// where a rename cannot go: Personal's directory and lich's own may sit on
// different filesystems.
func move(from, to string) error {
	if err := os.MkdirAll(filepath.Dir(to), 0o700); err != nil {
		return err
	}
	err := os.Rename(from, to)
	var linkErr *os.LinkError
	if err == nil || !errors.As(err, &linkErr) || !errors.Is(linkErr.Err, syscall.EXDEV) {
		return err
	}
	if err := copyTree(from, to); err != nil {
		_ = os.RemoveAll(to)
		return err
	}
	return os.RemoveAll(from)
}

func copyTree(from, to string) error {
	return filepath.WalkDir(from, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o700)
		}
		return copyFile(path, dst)
	})
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
