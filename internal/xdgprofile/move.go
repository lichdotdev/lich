package xdgprofile

import (
	"fmt"
	"path/filepath"

	"github.com/lichdotdev/lich/internal/profilestore"
)

// MoveConversation carries a conversation to the profile toSlug, and back by
// calling it again, so a session can continue in another account.
//
// Cursor CLI resumes a chat only from the active config dir, so its chat
// directory moves. Kiro CLI keeps its conversations under ~/.kiro, which is
// every profile's, so there is nothing to move. opencode keeps a conversation
// in the database that holds the login, so it cannot move and this answers
// ErrNoMove.
func (p *Provider) MoveConversation(conversationID, toSlug string) error {
	switch p {
	case Kiro:
		_, err := p.store.ConfigDir(toSlug)
		return err
	case Cursor:
		return p.store.MoveConversation(conversationID, toSlug, p.inState(CursorChat), p.cursorEntries)
	}
	return fmt.Errorf("%s: %w", p.store.Label, ErrNoMove)
}

// CursorChat is the directory Cursor CLI keeps one chat in under a config dir:
// chats/<md5 of the cwd>/<id>/, found by its store.db.
func CursorChat(stateDir, conversationID string) (string, bool) {
	if conversationID == "" || filepath.Base(conversationID) != conversationID {
		return "", false
	}
	matches, _ := filepath.Glob(filepath.Join(stateDir, "chats", "*", conversationID, "store.db"))
	if len(matches) == 0 {
		return "", false
	}
	return filepath.Dir(matches[0]), true
}

// inState is locate for profilestore, which hands over a profile's base
// directory rather than its state directory.
func (p *Provider) inState(locate func(stateDir, conversationID string) (string, bool)) func(home, conversationID string) (string, bool) {
	return func(home, conversationID string) (string, bool) {
		dir, err := p.stateOf(home)
		if err != nil {
			return "", false
		}
		return locate(dir, conversationID)
	}
}

// cursorEntries is the chat directory, moved under the same workspace key.
func (p *Provider) cursorEntries(_, chat, _, toHome string) ([]profilestore.Pair, error) {
	to, err := p.stateOf(toHome)
	if err != nil {
		return nil, err
	}
	workspace := filepath.Base(filepath.Dir(chat))
	return []profilestore.Pair{{From: chat, To: filepath.Join(to, "chats", workspace, filepath.Base(chat))}}, nil
}
