package codexprofile

import (
	"os"
	"path/filepath"

	"github.com/omartelo/lich/internal/profilestore"
)

// Owner is the profile holding a Codex conversation, and the path of its
// rollout there. A conversation id is a UUID and lives in exactly one profile,
// so the first match is the only one.
func Owner(conversationID string) (Profile, string, bool) {
	return store.Owner(conversationID, RolloutIn)
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
func MoveConversation(conversationID, toSlug string) error {
	return store.MoveConversation(conversationID, toSlug, RolloutIn, conversationEntries)
}

func conversationEntries(id, rollout, fromDir, toDir string) ([]profilestore.Pair, error) {
	rel, err := filepath.Rel(fromDir, rollout)
	if err != nil {
		return nil, err
	}
	pairs := []profilestore.Pair{{From: rollout, To: filepath.Join(toDir, rel)}}
	images := filepath.Join("generated_images", id)
	if _, err := os.Lstat(filepath.Join(fromDir, images)); err == nil {
		pairs = append(pairs, profilestore.Pair{From: filepath.Join(fromDir, images), To: filepath.Join(toDir, images)})
	}
	return pairs, nil
}
