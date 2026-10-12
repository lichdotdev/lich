package terminal

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/omartelo/lich/internal/claudeprofile"
	"github.com/omartelo/lich/internal/providers"
)

// profileEventName carries the Claude Code profile a session moved to: Continue
// in, or a resume that found its conversation in another profile.
const profileEventName = "session-profile"

type profileEvent struct {
	ID string `json:"id"`
	// Profile is the slug, "" for Personal.
	Profile string `json:"profile"`
}

// claudeProfileDir resolves the config directory a Claude Code spawn of this
// session runs in, "" for Personal, which keeps lich's own environment.
//
// The session's row says which profile it was born in. A resume asks the disk
// instead: a conversation lives in exactly one profile, and resuming it
// anywhere else would open a conversation that is not there. When the two
// disagree (a conversation adopted from the history, one that moved) the row
// follows the disk. A profile deleted since answers ErrProfileGone, which is
// how the window learns to offer a new session in the project's profile.
func (s *Service) claudeProfileDir(id, kind, resume string) (string, error) {
	if kind != providers.Claude {
		return "", nil
	}
	slug, err := s.store.SessionProfile(id)
	if err != nil {
		return "", err
	}
	if resume != "" {
		if slug, err = s.followConversation(id, slug, resume); err != nil {
			return "", err
		}
	}
	if claudeprofile.IsPersonal(slug) {
		return "", nil
	}
	dir, err := claudeprofile.ConfigDir(slug)
	if err != nil {
		return "", err
	}
	if err := claudeprofile.EnsureShared(slug); err != nil {
		slog.Warn("link shared claude entries into profile", "profile", slug, "err", err)
	}
	return dir, nil
}

// followConversation is the row's profile, moved to the one holding the
// conversation being resumed when that is another.
func (s *Service) followConversation(id, slug, conversationID string) (string, error) {
	owner, _, ok := claudeprofile.Owner(conversationID)
	if !ok {
		return slug, nil
	}
	found := owner.Slug
	if owner.Personal {
		found = ""
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
	s.hub.Emit(profileEventName, profileEvent{ID: id, Profile: found})
	return found, nil
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
func (s *Service) ProfileSessions(slug string) int {
	n := 0
	s.spawns.Range(func(_, v any) bool {
		if started, ok := v.(spawn); ok && started.kind == providers.Claude && started.profile == slug {
			n++
		}
		return true
	})
	return n
}

// ContinueIn moves a Claude Code session's conversation to another profile and
// carries on there, in the same card: the process ends, the conversation and
// everything Claude Code keyed by its id move (claudeprofile.MoveConversation),
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
		return errors.New("this session has no Claude Code conversation to continue yet")
	}
	started := s.spawnOf(id)
	if started.kind != "" && started.kind != providers.Claude {
		return fmt.Errorf("only a Claude Code session can continue in another profile, not %s", started.kind)
	}
	if s.turns.busy(id) {
		return errors.New("the session is mid-turn; continue it in another profile once the turn ends")
	}
	if _, err := claudeprofile.ConfigDir(profile); err != nil {
		return err
	}
	live := s.Live(id)
	if live {
		if err := s.Close(id); err != nil {
			return fmt.Errorf("end the session before moving it: %w", err)
		}
	}
	if err := claudeprofile.MoveConversation(conversation, profile); err != nil {
		return s.respawnAfter(err, live, id, started, conversation)
	}
	if claudeprofile.IsPersonal(profile) {
		profile = ""
	}
	if err := s.store.SetSessionProfile(id, profile); err != nil {
		return err
	}
	s.hub.Emit(profileEventName, profileEvent{ID: id, Profile: profile})
	return s.respawnAfter(nil, live, id, started, conversation)
}

// respawnAfter brings a session that was running back up on its conversation,
// wherever that now lives, and reports cause: a failed move still leaves the
// card running where it was.
func (s *Service) respawnAfter(cause error, live bool, id string, started spawn, conversation string) error {
	if !live {
		return cause
	}
	err := s.Start(id, started.projectID, started.cwd, providers.Claude, conversation, "", false, false, 0, 0)
	return errors.Join(cause, err)
}
