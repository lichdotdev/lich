package terminal

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
	"github.com/lichdotdev/lich/internal/profilestore"
)

// A turn a usage limit stopped waits for its reset (internal/relay, resume.go).
// When another logged-in profile of the same provider has quota left, the
// window is offered it, and only the person's answer moves the session: a
// profile can be another account, and which account holds a conversation is
// theirs to say.

var errLimitOfferGone = errors.New("the session no longer waits on that limit: it moved, took another turn or closed")

// SetQuotaLeft wires the answer to "can this profile of this provider take a
// turn now" (quota.Service.HasQuotaLeft). Unset, nothing is ever offered.
func (s *Service) SetQuotaLeft(fn func(kind, slug string) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.quotaLeft = fn
}

// SetResumeNow wires the continuation sent once the person moved a session off
// its limit (relay.ResumeNow).
func (s *Service) SetResumeNow(fn func(id string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resumeNow = fn
}

// offerProfile offers session id, whose turn a usage limit just ended, the first
// other profile of its provider with quota left. A profile a limit stopped a
// turn in before its reset is not offered whatever its reading says: the reading
// is cached for minutes.
func (s *Service) offerProfile(id string) {
	kind := sessionKind(s.spawnOf(id).kind, s.store.SessionKind(id))
	from, err := s.store.SessionProfile(id)
	if err != nil {
		slog.Warn("terminal: read the profile of a session a limit stopped", "session", id, "err", err)
		return
	}
	to, ok := s.spareProfile(kind, from)
	if !ok {
		return
	}
	s.mu.Lock()
	if s.limitOffers == nil {
		s.limitOffers = make(map[string]string)
	}
	s.limitOffers[id] = to
	s.mu.Unlock()
	events.Send(s.hub.Emit, api.SessionLimitOfferEvent, api.SessionLimitOffer{ID: id, Profile: to})
}

// ContinueOffLimit is the person taking the profile a session was offered: the
// session moves through Continue in and its continuation goes out at once. An
// offer no longer standing is refused: the turn that moved on, the card that
// closed or moved since took it down.
func (s *Service) ContinueOffLimit(ctx context.Context, opts api.ContinueInOptions) (api.None, error) {
	if !s.takeLimitOffer(opts.ID, opts.Profile) {
		return api.None{}, errLimitOfferGone
	}
	if _, err := s.ContinueIn(ctx, opts); err != nil {
		return api.None{}, err
	}
	s.mu.Lock()
	resume := s.resumeNow
	s.mu.Unlock()
	if resume != nil {
		resume(opts.ID)
	}
	return api.None{}, nil
}

func (s *Service) takeLimitOffer(id, profile string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.limitOffers[id] != profile || profile == "" {
		return false
	}
	delete(s.limitOffers, id)
	return true
}

func (s *Service) forgetLimitOffer(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.limitOffers, id)
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
