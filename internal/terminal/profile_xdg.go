package terminal

import (
	"path/filepath"

	"github.com/omartelo/lich/internal/profilestore"
	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/xdgprofile"
)

// withXDGProfiles adds to ops the providers whose profiles are link homes
// (internal/xdgprofile), on the platforms where they have them.
func withXDGProfiles(ops map[string]profileOps) map[string]profileOps {
	for _, p := range xdgprofile.Providers() {
		op := profileOps{
			configDir: p.ConfigDir, ensureShared: p.EnsureShared, env: p.Env, moveTo: p.MoveConversation,
			// A Kiro CLI conversation is under ~/.kiro, every profile's, so no
			// profile owns one and a resume runs in the row's profile.
			owner: func(string) (profilestore.Profile, string, bool) { return profilestore.Profile{}, "", false },
		}
		switch p.ID {
		case providers.OpenCode:
			op.owner = opencodeOwner
			op.noMove = true
		case providers.Cursor:
			op.owner = func(id string) (profilestore.Profile, string, bool) { return p.Owner(id, xdgprofile.CursorChat) }
		}
		ops[p.ID] = op
	}
	return ops
}

// xdgStates is every profile of kind with the directory its provider keeps its
// conversations in, Personal first; Personal's alone, at personal, where kind
// has no profiles.
func xdgStates(kind string, personal func() (string, bool)) []xdgprofile.State {
	if p := xdgprofile.Of(kind); p != nil {
		return p.States()
	}
	dir, ok := personal()
	if !ok {
		return nil
	}
	return []xdgprofile.State{{Profile: profilestore.Profile{Slug: profilestore.Personal, Personal: true}, Dir: dir}}
}

// opencodeDataDir is Personal's opencode data directory, the one
// opencodeSessionDB's database is in.
func opencodeDataDir() (string, bool) {
	path, ok := opencodeSessionDB()
	return filepath.Dir(path), ok
}

// opencodeOwner is the opencode profile whose database holds a conversation,
// and that database.
func opencodeOwner(conversationID string) (profilestore.Profile, string, bool) {
	for _, s := range xdgStates(providers.OpenCode, opencodeDataDir) {
		db := filepath.Join(s.Dir, opencodeDBFile)
		if sessionRowExists(db, opencodeSessionTable, conversationID) {
			return s.Profile, db, true
		}
	}
	return profilestore.Profile{}, "", false
}
