package claudeprofile

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/omartelo/lich/internal/profilestore"
)

// Owner is the profile holding a Claude Code conversation, and the path of its
// transcript there. A conversation id is a UUID and lives in exactly one
// profile, so the first match is the only one.
func Owner(conversationID string) (Profile, string, bool) {
	return store.Owner(conversationID, transcriptIn)
}

func transcriptIn(dir, conversationID string) (string, bool) {
	// Globbed inside dir, which may hold metacharacters of its own.
	matches, _ := fs.Glob(os.DirFS(dir), path.Join("projects", "*", conversationID+".jsonl"))
	if len(matches) == 0 {
		return "", false
	}
	return filepath.Join(dir, filepath.FromSlash(matches[0])), true
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
func MoveConversation(conversationID, toSlug string) error {
	return store.MoveConversation(conversationID, toSlug, transcriptIn, conversationEntries)
}

func conversationEntries(id, transcript, fromDir, toDir string) ([]profilestore.Pair, error) {
	project, err := filepath.Rel(fromDir, filepath.Dir(transcript))
	if err != nil {
		return nil, err
	}
	beside, err := filepath.Glob(filepath.Join(filepath.Dir(transcript), id+"*"))
	if err != nil {
		return nil, err
	}
	var pairs []profilestore.Pair
	for _, path := range beside {
		pairs = append(pairs, profilestore.Pair{From: path, To: filepath.Join(toDir, project, filepath.Base(path))})
	}
	for _, dir := range sessionStateDirs {
		path := filepath.Join(fromDir, dir, id)
		if _, err := os.Lstat(path); err == nil {
			pairs = append(pairs, profilestore.Pair{From: path, To: filepath.Join(toDir, dir, id)})
		}
	}
	return pairs, nil
}
