package ompprofile

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/omartelo/lich/internal/profilestore"
)

// Owner is the profile holding an oh-my-pi conversation, and the path of its
// session file there. A session id is unique, so the first match is the only
// one.
func Owner(conversationID string) (Profile, string, bool) {
	return store.Owner(conversationID, SessionIn)
}

// SessionIn locates a conversation's JSONL under one agent directory: one file
// per session, in a directory named after the cwd it ran in,
// sessions/<encoded cwd>/<timestamp>_<id>.jsonl.
func SessionIn(dir, conversationID string) (string, bool) {
	matches, err := filepath.Glob(filepath.Join(dir, "sessions", "*", "*_"+conversationID+".jsonl"))
	if err != nil || len(matches) == 0 {
		return "", false
	}
	return matches[0], true
}

// MoveConversation moves an oh-my-pi conversation from the profile that holds
// it to another: its session file and the directory of artifacts beside it
// (named after the file, holding the sub-agents' sessions and tool output), and
// a copy of every image blob the session refers to. Resume finds a session only
// in the active profile's own sessions directory, so the move is what makes the
// conversation resumable there.
//
// What stays behind is what is not keyed by the id: the prompt history
// (history.db) and the blobs themselves, which other sessions of the source may
// still refer to.
func MoveConversation(conversationID, toSlug string) error {
	return store.MoveConversation(conversationID, toSlug, SessionIn, conversationEntries)
}

// blobRef is how a session file refers to an externalized image: its SHA-256,
// naming the file under the agent directory's blobs/.
var blobRef = regexp.MustCompile(`blob:sha256:([a-f0-9]{64})`)

// conversationEntries is the session file and its artifacts directory. The
// blobs are copied here, ahead of the move, because they are content-addressed
// and shared between sessions: a move that then fails leaves a few extra blobs
// in the target and the conversation where it was.
func conversationEntries(_, session, fromDir, toDir string) ([]profilestore.Pair, error) {
	rel, err := filepath.Rel(fromDir, session)
	if err != nil {
		return nil, err
	}
	pairs := []profilestore.Pair{{From: session, To: filepath.Join(toDir, rel)}}
	sessions := []string{session}
	artifacts := strings.TrimSuffix(session, ".jsonl")
	if _, err := os.Lstat(artifacts); err == nil {
		rel, err := filepath.Rel(fromDir, artifacts)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, profilestore.Pair{From: artifacts, To: filepath.Join(toDir, rel)})
		nested, err := sessionsUnder(artifacts)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, nested...)
	}
	return pairs, copyBlobs(sessions, fromDir, toDir)
}

// sessionsUnder is the session files in a session's artifacts directory: the
// sub-agents', which refer to blobs of their own.
func sessionsUnder(artifacts string) ([]string, error) {
	var found []string
	err := filepath.WalkDir(artifacts, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".jsonl" {
			found = append(found, path)
		}
		return nil
	})
	return found, err
}

// copyBlobs copies into toDir the blobs the session files refer to and toDir
// does not hold. A blob fromDir no longer has (omp's own gc removes unreferenced
// ones) is skipped: the image is gone from the source conversation too.
func copyBlobs(sessions []string, fromDir, toDir string) error {
	hashes := map[string]bool{}
	for _, file := range sessions {
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		for _, m := range blobRef.FindAllSubmatch(data, -1) {
			hashes[string(m[1])] = true
		}
	}
	for hash := range hashes {
		src, dst := filepath.Join(fromDir, "blobs", hash), filepath.Join(toDir, "blobs", hash)
		if _, err := os.Lstat(dst); err == nil {
			continue
		}
		if err := copyBlob(src, dst); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// copyBlob writes dst through a temporary file and a rename, so omp never reads
// a blob half copied.
func copyBlob(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".blob-*.tmp")
	if err != nil {
		return err
	}
	if _, err := io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), dst)
}
