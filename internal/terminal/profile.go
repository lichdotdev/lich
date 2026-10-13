package terminal

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/omartelo/lich/internal/claudeprofile"
	"github.com/omartelo/lich/internal/codexprofile"
	"github.com/omartelo/lich/internal/providers"
)

// profileEventName carries the profile a session moved to: Continue in, or a
// resume that found its conversation in another profile.
const profileEventName = "session-profile"

type profileEvent struct {
	ID string `json:"id"`
	// Profile is the slug, "" for Personal.
	Profile string `json:"profile"`
}

// hasProfiles is whether a provider's sessions can run in a profile of their
// own (internal/claudeprofile, internal/codexprofile). The small helpers down
// to profileDir are the only places that tell the two packages apart.
func hasProfiles(kind string) bool { return kind == providers.Claude || kind == providers.Codex }

// profileConfigDir is the state directory of a profile of kind's provider.
func profileConfigDir(kind, slug string) (string, error) {
	if kind == providers.Codex {
		return codexprofile.ConfigDir(slug)
	}
	return claudeprofile.ConfigDir(slug)
}

func ensureProfileShared(kind, slug string) error {
	if kind == providers.Codex {
		return codexprofile.EnsureShared(slug)
	}
	return claudeprofile.EnsureShared(slug)
}

// conversationOwner is the profile holding a conversation, "" for Personal.
func conversationOwner(kind, conversationID string) (slug string, ok bool) {
	if kind == providers.Codex {
		owner, _, found := codexprofile.Owner(conversationID)
		return slugOf(owner.Slug, owner.Personal), found
	}
	owner, _, found := claudeprofile.Owner(conversationID)
	return slugOf(owner.Slug, owner.Personal), found
}

func slugOf(slug string, personal bool) string {
	if personal {
		return ""
	}
	return slug
}

// profileEnv is env with a profile's directory in place of the provider's own
// state directory and the variables that would override its login removed.
func profileEnv(kind string, env []string, dir string) []string {
	if kind == providers.Codex {
		return codexprofile.Env(env, dir)
	}
	return claudeprofile.Env(env, dir)
}

func moveProfileConversation(kind, conversationID, toSlug string) error {
	if kind == providers.Codex {
		return codexprofile.MoveConversation(conversationID, toSlug)
	}
	return claudeprofile.MoveConversation(conversationID, toSlug)
}

// profileDir resolves the state directory a spawn of this session runs in, ""
// for Personal, which keeps lich's own environment and for a provider with no
// profiles.
//
// The session's row says which profile it was born in. A resume asks the disk
// instead: a conversation lives in exactly one profile, and resuming it
// anywhere else would open a conversation that is not there. When the two
// disagree (a conversation adopted from the history, one that moved) the row
// follows the disk. A profile deleted since answers ErrProfileGone, which is
// how the window learns to offer a new session in the project's profile.
func (s *Service) profileDir(id, kind, resume string) (string, error) {
	if !hasProfiles(kind) {
		return "", nil
	}
	slug, err := s.store.SessionProfile(id)
	if err != nil {
		return "", err
	}
	if resume != "" {
		if slug, err = s.followConversation(id, kind, slug, resume); err != nil {
			return "", err
		}
	}
	if claudeprofile.IsPersonal(slug) {
		return "", nil
	}
	dir, err := profileConfigDir(kind, slug)
	if err != nil {
		return "", err
	}
	if err := ensureProfileShared(kind, slug); err != nil {
		slog.Warn("link shared entries into profile", "kind", kind, "profile", slug, "err", err)
	}
	return dir, nil
}

// followConversation is the row's profile, moved to the one holding the
// conversation being resumed when that is another. Start tells the window,
// once the process runs under it.
func (s *Service) followConversation(id, kind, slug, conversationID string) (string, error) {
	found, ok := conversationOwner(kind, conversationID)
	if !ok {
		return slug, nil
	}
	if claudeprofile.IsPersonal(slug) {
		slug = ""
	}
	if found == slug {
		return slug, nil
	}
	if err := s.store.SetSessionProfile(id, found); err != nil {
		return "", err
	}
	return found, nil
}

// reportProfileMove tells the window a resume followed its conversation into
// another profile. Called after the spawn, so whoever re-reads on the event
// (the quota of the session's account) finds the process under the profile.
func (s *Service) reportProfileMove(id, bornIn string) {
	now, err := s.store.SessionProfile(id)
	if err != nil || now == bornIn {
		return
	}
	s.hub.Emit(profileEventName, profileEvent{ID: id, Profile: now})
}

// profileSlug is the profile a config directory belongs to, "" for Personal's
// (an empty dir): what a live spawn records to be counted by ProfileSessions.
func profileSlug(dir string) string {
	if dir == "" {
		return ""
	}
	return filepath.Base(dir)
}

// ProfileSessions counts the live sessions running in a Claude Code profile, so
// a profile is never deleted out from under one.
func (s *Service) ProfileSessions(slug string) int { return s.profileSessions(providers.Claude, slug) }

// CodexProfileSessions is ProfileSessions for a Codex profile.
func (s *Service) CodexProfileSessions(slug string) int {
	return s.profileSessions(providers.Codex, slug)
}

func (s *Service) profileSessions(kind, slug string) int {
	n := 0
	s.spawns.Range(func(_, v any) bool {
		if started, ok := v.(spawn); ok && started.kind == kind && started.profile == slug {
			n++
		}
		return true
	})
	return n
}

// ContinueIn moves a Claude Code or Codex session's conversation to another
// profile of the same provider and carries on there, in the same card: the
// process ends, the conversation and everything its provider keyed by its id
// move (claudeprofile.MoveConversation, codexprofile.MoveConversation),
// the row takes the new profile, and a running session is spawned again
// resuming it. Moving back is the same call. Nothing else moves: not the other
// sessions in the profile, and not the project's own profile.
//
// It refuses a turn in progress: a provider-reported turn still open, which a
// permission prompt is part of. A turn a usage limit stopped has ended, so a
// session waiting out its limit moves like an idle one.
func (s *Service) ContinueIn(id, profile string) error {
	conversation, err := s.store.ProviderSession(id)
	if err != nil {
		return err
	}
	if conversation == "" {
		return errors.New("this session has no conversation to continue yet")
	}
	started := s.spawnOf(id)
	kind := sessionKind(started.kind, s.store.SessionKind(id))
	if !hasProfiles(kind) {
		return fmt.Errorf("only a Claude Code or Codex session can continue in another profile, not %s", kind)
	}
	if s.turns.busy(id) {
		return errors.New("the session is mid-turn; continue it in another profile once the turn ends")
	}
	if _, err := profileConfigDir(kind, profile); err != nil {
		return err
	}
	live := s.Live(id)
	if live {
		if err := s.Close(id); err != nil {
			return fmt.Errorf("end the session before moving it: %w", err)
		}
	}
	if err := moveProfileConversation(kind, conversation, profile); err != nil {
		return s.respawnAfter(err, live, id, started, kind, conversation)
	}
	if claudeprofile.IsPersonal(profile) {
		profile = ""
	}
	if err := s.store.SetSessionProfile(id, profile); err != nil {
		return err
	}
	err = s.respawnAfter(nil, live, id, started, kind, conversation)
	// After the respawn: a reader that re-reads on this event (the quota of
	// the session's account) must find the process already under the profile.
	s.hub.Emit(profileEventName, profileEvent{ID: id, Profile: profile})
	return err
}

// sessionKind is the provider of a session being moved: its running spawn's,
// else its row's, and Claude Code for a row that does not say, which is every
// session born before lich recorded a kind.
func sessionKind(spawned, recorded string) string {
	switch {
	case spawned != "":
		return spawned
	case recorded != "":
		return recorded
	}
	return providers.Claude
}

// respawnAfter brings a session that was running back up on its conversation,
// wherever that now lives, and reports cause: a failed move still leaves the
// card running where it was.
func (s *Service) respawnAfter(cause error, live bool, id string, started spawn, kind, conversation string) error {
	if !live {
		return cause
	}
	err := s.Start(id, started.projectID, started.cwd, kind, conversation, "", false, false, 0, 0)
	return errors.Join(cause, err)
}
