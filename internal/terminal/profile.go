package terminal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/claudeprofile"
	"github.com/lichdotdev/lich/internal/codexprofile"
	"github.com/lichdotdev/lich/internal/ompprofile"
	"github.com/lichdotdev/lich/internal/profilestore"
	"github.com/lichdotdev/lich/internal/providers"

	"github.com/lichdotdev/lich/internal/events"
)

type profileEvent = api.SessionProfile

// profileOps is what the service asks of one provider's profiles: the packages
// that keep them (internal/claudeprofile, internal/codexprofile,
// internal/ompprofile), which answer the same calls on their own directories.
type profileOps struct {
	configDir    func(slug string) (string, error)
	ensureShared func(slug string) error
	owner        func(conversationID string) (profilestore.Profile, string, bool)
	env          func(env []string, dir string) []string
	moveTo       func(conversationID, toSlug string) error
	// slugOf is the profile a state directory belongs to, for a provider
	// whose directory is not named after it.
	slugOf func(dir string) string
	// noMove is a provider whose conversations cannot change profile
	// (opencode's live in the database that holds the login).
	noMove bool
}

// profilestores is the set of providers whose sessions can run in a profile of
// their own, by provider id: opencode, Cursor CLI and Kiro CLI only where they
// have profiles (internal/xdgprofile, profile_xdg.go).
var profilestores = withXDGProfiles(map[string]profileOps{
	providers.Claude: {
		configDir: claudeprofile.ConfigDir, ensureShared: claudeprofile.EnsureShared,
		owner: claudeprofile.Owner, env: claudeprofile.Env, moveTo: claudeprofile.MoveConversation,
	},
	providers.Codex: {
		configDir: codexprofile.ConfigDir, ensureShared: codexprofile.EnsureShared,
		owner: codexprofile.Owner, env: codexprofile.Env, moveTo: codexprofile.MoveConversation,
	},
	providers.OMP: {
		configDir: ompprofile.ConfigDir, ensureShared: ompprofile.EnsureShared,
		owner: ompprofile.Owner, env: ompprofile.Env, moveTo: ompprofile.MoveConversation,
		// The profile's directory is its agent/ under ~/.omp/profiles/<slug>.
		slugOf: func(dir string) string { return filepath.Base(filepath.Dir(dir)) },
	},
})

func hasProfiles(kind string) bool {
	_, ok := profilestores[kind]
	return ok
}

// conversationOwner is the profile holding a conversation, "" for Personal.
func conversationOwner(kind, conversationID string) (slug string, ok bool) {
	owner, _, found := profilestores[kind].owner(conversationID)
	if owner.Personal {
		return "", found
	}
	return owner.Slug, found
}

// profileEnv is env with a profile's directory in place of the provider's own
// state directory and the variables that would override its login removed.
func profileEnv(kind string, env []string, dir string) []string {
	return profilestores[kind].env(env, dir)
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
	if profilestore.IsPersonal(slug) {
		return "", nil
	}
	dir, err := profilestores[kind].configDir(slug)
	if err != nil {
		return "", err
	}
	if err := profilestores[kind].ensureShared(slug); err != nil {
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
	if profilestore.IsPersonal(slug) {
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
	events.Send(s.hub.Emit, api.SessionProfileEvent, profileEvent{ID: id, Profile: now})
}

// profileSlug is the profile a state directory of kind's provider belongs to,
// "" for Personal's (an empty dir): what a live spawn records to be counted by
// ProfileSessions.
func profileSlug(kind, dir string) string {
	if dir == "" {
		return ""
	}
	if slugOf := profilestores[kind].slugOf; slugOf != nil {
		return slugOf(dir)
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

// OMPProfileSessions is ProfileSessions for an oh-my-pi profile.
func (s *Service) OMPProfileSessions(slug string) int {
	return s.profileSessions(providers.OMP, slug)
}

// XDGProfileSessions is ProfileSessions for an opencode, Cursor CLI or Kiro CLI
// profile.
func (s *Service) XDGProfileSessions(kind, slug string) int {
	return s.profileSessions(kind, slug)
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

// ContinueIn moves a session's conversation to another profile of the same
// provider and carries on there, in the same card: the process ends, the
// conversation and everything its provider keyed by its id move
// (claudeprofile.MoveConversation and its siblings; a Kiro CLI conversation is
// every profile's and stays, an opencode one cannot move),
// the row takes the new profile, and a running session is spawned again
// resuming it. Moving back is the same call. Nothing else moves: not the other
// sessions in the profile, and not the project's own profile. A session that
// has its id but no transcript yet, before its first prompt, has nothing to
// move: it starts afresh in the target profile.
//
// It refuses a turn in progress: a provider-reported turn still open, which a
// permission prompt is part of. A turn a usage limit stopped has ended, so a
// session waiting out its limit moves like an idle one.
func (s *Service) ContinueIn(ctx context.Context, opts api.ContinueInOptions) (api.None, error) {
	conversation, err := s.store.ProviderSession(opts.ID)
	if err != nil {
		return api.None{}, err
	}
	if conversation == "" {
		return api.None{}, errors.New("this session has no conversation to continue yet")
	}
	started := s.spawnOf(opts.ID)
	kind := sessionKind(started.kind, s.store.SessionKind(opts.ID))
	if !hasProfiles(kind) {
		return api.None{}, fmt.Errorf("a %s session has no profiles to continue in", kind)
	}
	if profilestores[kind].noMove {
		return api.None{}, fmt.Errorf("a %s conversation cannot move to another profile; start a new session there", kind)
	}
	if s.turns.busy(opts.ID) {
		return api.None{}, errors.New("the session is mid-turn; continue it in another profile once the turn ends")
	}
	if _, err := profilestores[kind].configDir(opts.Profile); err != nil {
		return api.None{}, err
	}
	live := s.live(opts.ID)
	if live {
		if err := s.endProcess(opts.ID, ""); err != nil {
			return api.None{}, fmt.Errorf("end the session before moving it: %w", err)
		}
	}
	err = profilestores[kind].moveTo(conversation, opts.Profile)
	if errors.Is(err, profilestore.ErrNoConversation) {
		// Nothing written yet to carry over: the card starts afresh there.
		if err := s.setConversation(opts.ID, ""); err != nil {
			return api.None{}, err
		}
		conversation, err = "", nil
	}
	if err != nil {
		return api.None{}, s.respawnAfter(err, live, opts.ID, started, kind, conversation)
	}
	if profilestore.IsPersonal(opts.Profile) {
		opts.Profile = ""
	}
	if err := s.store.SetSessionProfile(opts.ID, opts.Profile); err != nil {
		return api.None{}, err
	}
	err = s.respawnAfter(nil, live, opts.ID, started, kind, conversation)
	// After the respawn: a reader that re-reads on this event (the quota of
	// the session's account) must find the process already under the profile.
	events.Send(s.hub.Emit, api.SessionProfileEvent, profileEvent{ID: opts.ID, Profile: opts.Profile})
	return api.None{}, err
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
	_, err := s.Start(context.Background(), api.StartOptions{ID: id, ProjectID: started.projectID, Cwd: started.cwd, Kind: kind, Resume: conversation})
	return errors.Join(cause, err)
}
