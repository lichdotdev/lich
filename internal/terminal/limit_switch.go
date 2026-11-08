package terminal

import (
	"log/slog"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/profilestore"
)

// A turn a usage limit stopped carries on in another logged-in profile of the
// same provider that still has quota, through Continue in, instead of waiting
// for the reset. The relay decides when (internal/relay, resume.go); this file
// finds the profile and moves the session.

// SetQuotaLeft wires the answer to "can this profile of this provider take a
// turn now" (quota.Service.HasQuotaLeft). Unset, no session moves off a limit.
func (s *Service) SetQuotaLeft(fn func(kind, slug string) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.quotaLeft = fn
}

// ContinueOffLimit moves session id, whose turn a usage limit just ended, to
// the first other profile of its provider with quota left, and reports whether
// it moved. A profile a limit stopped a turn in before its reset is skipped
// whatever its reading says: the reading is cached for minutes, and trusting a
// stale one would bounce the session between two spent profiles.
func (s *Service) ContinueOffLimit(id string) bool {
	kind := sessionKind(s.spawnOf(id).kind, s.store.SessionKind(id))
	from, err := s.store.SessionProfile(id)
	if err != nil {
		slog.Warn("terminal: read the profile of a session a limit stopped", "session", id, "err", err)
		return false
	}
	to, ok := s.spareProfile(kind, from)
	if !ok {
		return false
	}
	if err := s.continueIn(api.ContinueInOptions{ID: id, Profile: to}, api.ProfileMovedOffLimit); err != nil {
		slog.Warn("terminal: continue a session off its usage limit", "session", id, "profile", to, "err", err)
		return false
	}
	return true
}

// spareProfile is the first profile of kind's provider, other than from, that
// is not spent and has quota left.
func (s *Service) spareProfile(kind, from string) (string, bool) {
	list := profilestores[kind].list
	s.mu.Lock()
	quotaLeft := s.quotaLeft
	s.mu.Unlock()
	if list == nil || quotaLeft == nil {
		return "", false
	}
	profiles, err := list()
	if err != nil {
		slog.Warn("terminal: list profiles to continue a session off its limit", "kind", kind, "err", err)
		return "", false
	}
	for _, p := range profiles {
		if sameProfile(p.Slug, from) || s.spent(kind, p.Slug) {
			continue
		}
		if quotaLeft(kind, p.Slug) {
			return p.Slug, true
		}
	}
	return "", false
}

// markSpent records that the profile session id runs in is spent until
// resetsAt (unix seconds).
func (s *Service) markSpent(id string, resetsAt int64) {
	kind := sessionKind(s.spawnOf(id).kind, s.store.SessionKind(id))
	slug, err := s.store.SessionProfile(id)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.spentUntil == nil {
		s.spentUntil = make(map[string]int64)
	}
	s.spentUntil[spentKey(kind, slug)] = resetsAt
}

func (s *Service) spent(kind, slug string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.spentUntil[spentKey(kind, slug)] > time.Now().Unix()
}

func spentKey(kind, slug string) string {
	if profilestore.IsPersonal(slug) {
		slug = profilestore.Personal
	}
	return kind + "\x00" + slug
}

func sameProfile(a, b string) bool {
	return a == b || profilestore.IsPersonal(a) && profilestore.IsPersonal(b)
}
