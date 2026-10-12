package claudeprofile

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// Owner is the profile holding a Claude Code conversation, and the path of its
// transcript there. A conversation id is a UUID and lives in exactly one
// profile, so the first match is the only one.
func Owner(conversationID string) (Profile, string, bool) {
	for _, p := range ConfigDirs() {
		matches, _ := filepath.Glob(filepath.Join(p.Dir, "projects", "*", conversationID+".jsonl"))
		if len(matches) > 0 {
			return p, matches[0], true
		}
	}
	return Profile{}, "", false
}

// sessionStateDirs are the directories Claude Code keeps per-conversation state
// in, each holding one entry named after the conversation id (measured on
// 2.1.296): file-history/<id>/ for the edit checkpoints, session-env/<id>/,
// and tasks/<id>/ for its task list.
var sessionStateDirs = []string{"file-history", "session-env", "tasks"}

// MoveConversation moves everything Claude Code keyed by a conversation id from
// one profile to another: the transcript, its subagent directory and any file a
// hook filed beside it under the same name, and the per-conversation state
// directories. The conversation resumes in the target with its history and its
// checkpoints (measured round trip on 2.1.296).
//
// What stays behind is what is not keyed by the id: the prompt history
// (history.jsonl) and the project's lastSessionId, which `claude --continue` in
// the source profile then points at a conversation it no longer holds.
//
// A failure part way moves back whatever had already moved, so the
// conversation is never split across two profiles.
func MoveConversation(conversationID, toSlug string) error {
	from, transcript, ok := Owner(conversationID)
	if !ok {
		return fmt.Errorf("no Claude Code conversation %q in any profile", conversationID)
	}
	toDir, err := ConfigDir(toSlug)
	if err != nil {
		return err
	}
	if filepath.Clean(from.Dir) == filepath.Clean(toDir) {
		return fmt.Errorf("conversation %q is already in profile %q", conversationID, toSlug)
	}
	pairs, err := conversationEntries(conversationID, transcript, from.Dir, toDir)
	if err != nil {
		return err
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

func conversationEntries(id, transcript, fromDir, toDir string) ([]movePair, error) {
	project, err := filepath.Rel(fromDir, filepath.Dir(transcript))
	if err != nil {
		return nil, err
	}
	beside, err := filepath.Glob(filepath.Join(filepath.Dir(transcript), id+"*"))
	if err != nil {
		return nil, err
	}
	var pairs []movePair
	for _, path := range beside {
		pairs = append(pairs, movePair{path, filepath.Join(toDir, project, filepath.Base(path))})
	}
	for _, dir := range sessionStateDirs {
		path := filepath.Join(fromDir, dir, id)
		if _, err := os.Lstat(path); err == nil {
			pairs = append(pairs, movePair{path, filepath.Join(toDir, dir, id)})
		}
	}
	return pairs, nil
}

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
