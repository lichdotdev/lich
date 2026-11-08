package terminal

import (
	"context"
	"errors"
	"fmt"
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
// theirs to say. An agent handing its work off can suggest a profile for its
// successor, which reaches the window as the same offer once a turn has ended
// there.

var errOfferGone = errors.New("that offer no longer stands: the session moved on, changed profile or closed")

// profileOffer is a profile a session was offered and has not answered. from
// is the profile it was offered from: a session no longer there has moved.
// handoff is a successor's: a turn starting does not take it down, and
// accepting it sends no continuation, since nothing was stopped. held is one
// not on screen, waiting for a turn to end so that its button works when it
// appears: Continue in refuses a turn in progress and a session with no
// conversation yet.
type profileOffer struct {
	profile, from string
	handoff, held bool
}

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
	offer := profileOffer{profile: to, from: from}
	s.storeOffer(id, offer)
	s.announceOffer(id, offer)
}

// SuggestProfile holds the profile an agent handing off suggested for its
// successor, for the person once a turn has ended there (showHeldOffer). It
// moves nothing. The error is why the suggestion cannot apply, which the handoff
// reports and does not fail over.
func (s *Service) SuggestProfile(id, slug string) error {
	kind := sessionKind(s.spawnOf(id).kind, s.store.SessionKind(id))
	ops, ok := profilestores[kind]
	if !ok {
		return fmt.Errorf("%s has no profiles", kind)
	}
	if ops.noMove {
		return fmt.Errorf("a %s conversation cannot move to another profile", kind)
	}
	if _, err := ops.configDir(slug); err != nil {
		return fmt.Errorf("no %s profile is named %q", kind, slug)
	}
	from, err := s.store.SessionProfile(id)
	if err != nil {
		return err
	}
	if sameProfile(slug, from) {
		return fmt.Errorf("the session already runs in %q", slug)
	}
	s.storeOffer(id, profileOffer{profile: slug, from: from, handoff: true, held: true})
	return nil
}

func (s *Service) storeOffer(id string, offer profileOffer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.profileOffers == nil {
		s.profileOffers = make(map[string]profileOffer)
	}
	s.profileOffers[id] = offer
}

func (s *Service) announceOffer(id string, offer profileOffer) {
	events.Send(s.hub.Emit, api.SessionProfileOfferEvent, api.SessionProfileOffer{ID: id, Profile: offer.profile, Handoff: offer.handoff})
}

// showHeldOffer puts on screen the offer session id holds, now a turn has ended
// there.
func (s *Service) showHeldOffer(id string) {
	s.mu.Lock()
	offer := s.profileOffers[id]
	held := offer.held
	if held {
		offer.held = false
		s.profileOffers[id] = offer
	}
	s.mu.Unlock()
	if held {
		s.announceOffer(id, offer)
	}
}

// TakeProfileOffer is the person taking the profile a session was offered: the
// session moves through Continue in and, off a limit, its continuation goes out
// at once. An offer no longer standing is refused: the turn that moved on, the
// card that closed or moved since took it down. A successor mid-turn is refused
// by Continue in itself and says so. Its offer goes back to held rather than
// being spent, so it is shown again when that turn ends: the click took the
// toast down.
func (s *Service) TakeProfileOffer(ctx context.Context, opts api.ContinueInOptions) (api.None, error) {
	current, err := s.store.SessionProfile(opts.ID)
	if err != nil {
		return api.None{}, err
	}
	offer, ok := s.takeOffer(opts.ID, opts.Profile, current)
	if !ok {
		return api.None{}, errOfferGone
	}
	_, err = s.ContinueIn(ctx, opts)
	if errors.Is(err, errMidTurn) && offer.handoff {
		offer.held = true
		s.storeOffer(opts.ID, offer)
	}
	if err != nil || offer.handoff {
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

func (s *Service) takeOffer(id, profile, current string) (profileOffer, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	offer, ok := s.profileOffers[id]
	if !ok || offer.profile != profile || profile == "" {
		return profileOffer{}, false
	}
	delete(s.profileOffers, id)
	return offer, sameProfile(offer.from, current)
}

func (s *Service) forgetProfileOffer(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.profileOffers, id)
}

// withdrawLimitOffer takes down what a turn starting makes moot: the session is
// no longer stopped. A handoff's offer stands through the turn.
func (s *Service) withdrawLimitOffer(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.profileOffers[id].handoff {
		delete(s.profileOffers, id)
	}
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
