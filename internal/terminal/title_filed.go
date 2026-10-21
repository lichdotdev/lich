package terminal

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/lichdotdev/lich/internal/providers"
)

// The providers lich reads a session title for itself, rather than being sent
// one. The report every other harness sends is a hook script reading a
// transcript path off its own stdin (docs/hooks/session-title.md), and neither
// of these hands its hooks a file the title is in. Kiro passes a path on none of
// its five events, and files a `title`, derived from the first prompt, into the
// session metadata transcript.go resolves (kiroSessionPath), the same `.json` the
// context readout is read from. Cursor CLI hands the plugin a transcript that
// holds no title, and files a generated one into the `meta.json` beside the
// chat's store.db (cursorChatStore), seconds into the first turn of an
// interactive session (measured on 2026.10.16; a `-p` run never writes one). So
// the turn's end is where lich goes and reads it.

// cursorChatMeta is the file Cursor CLI keeps a chat's metadata in, beside its
// store.db.
const cursorChatMeta = "meta.json"

// metadataTitle reads the title a provider filed for one conversation. ok is
// false for every absence (no file yet, a file caught mid-write, metadata
// carrying no title), because a session whose name lich cannot read keeps the
// name lich gave it, which is silence rather than a failure.
func metadataTitle(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var meta struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return "", false
	}
	// Trimmed like the reported one is (parseSessionTitle), so both doors
	// apply the same string.
	title := strings.TrimSpace(meta.Title)
	return title, title != ""
}

// filedTitle resolves the title the provider filed for the conversation running
// in session id. ok is false when the session runs a provider that files none
// lich reads, when no conversation is linked to it yet, and on every absence
// above.
func (s *Service) filedTitle(id string) (string, bool) {
	started := s.spawnOf(id)
	if started.kind != providers.Kiro && started.kind != providers.Cursor {
		return "", false
	}
	providerSessionID, err := s.store.ProviderSession(id)
	if err != nil || providerSessionID == "" {
		return "", false
	}
	path, ok := filedTitlePath(started.kind, providerSessionID, started.cwd)
	if !ok {
		return "", false
	}
	return metadataTitle(path)
}

// filedTitlePath is the file kind keeps one conversation's title in.
func filedTitlePath(kind, providerSessionID, cwd string) (string, bool) {
	if kind == providers.Kiro {
		return kiroSessionPath(providerSessionID)
	}
	chatStore, ok := cursorChatStore(providerSessionID, cwd)
	if !ok {
		return "", false
	}
	return filepath.Join(filepath.Dir(chatStore), cursorChatMeta), true
}

// applyFiledTitle publishes that title down the same path a reported one takes
// (onTitle): the store's guarded write, which leaves a session the user renamed
// alone, and the event the card redraws on.
func (s *Service) applyFiledTitle(id string) {
	title, ok := s.filedTitle(id)
	if !ok {
		return
	}
	if err := s.onTitle(id, title); err != nil {
		slog.Warn("terminal: apply filed title", "session", id, "err", err)
	}
}
