package spawn

import (
	"context"
	"errors"
	"fmt"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/profilestore"
	"github.com/lichdotdev/lich/internal/store"
)

type HandoffOptions = api.HandoffSessionOptions

// Handoff opens the successor of the session From runs in: a fresh session in
// the caller's checkout and folder, on the caller's provider, model, effort,
// ultracode and profile unless opts names others, so an agent at a clean break
// can pass its work on instead of carrying a degraded context. The caller is
// left open; closing it is the user's call.
func (s *Service) Handoff(_ context.Context, opts HandoffOptions) (Session, error) {
	return s.openSession(request{
		fromID: opts.From, kind: opts.Kind, model: opts.Model, effort: opts.Effort,
		profile: opts.Profile, handoff: true,
	})
}

// refuseOrphan refuses a subagent or a handoff asked for outside a session lich
// holds: both are placed beside a caller, and there is none.
func refuseOrphan(req request, originID string) error {
	switch {
	case originID != "":
		return nil
	case req.subagent:
		return errors.New(
			"a subagent is opened for the session asking for it, and this is not running " +
				"in a session lich holds — open it without --subagent, or from inside a session",
		)
	case req.handoff:
		return errors.New(
			"a handoff passes a session's work to a fresh one, and this is not running in " +
				"a session lich holds — run it from inside the session handing off",
		)
	}
	return nil
}

// inheritCaller fills what a handoff left empty from the caller, when the
// successor runs the caller's provider: a model, effort or profile belongs to
// one provider and means nothing on another. An inherited Personal is spelled
// out, because an empty profile is the project's, and the caller may have been
// moved off it (Continue in).
func (s *Service) inheritCaller(req request, projects []store.Project, kind string) (request, error) {
	if req.profile != "" {
		if err := s.term.CheckProfile(kind, req.profile); err != nil {
			return req, err
		}
	}
	if callerKind, _ := resolveKind(projects, req.fromID, ""); kind != callerKind {
		return req, nil
	}
	if req.model == "" {
		req.model = s.sessions.SessionModel(req.fromID)
	}
	if req.effort == "" {
		req.effort = s.sessions.SessionEffort(req.fromID)
	}
	req.ultracode = s.sessions.SessionUltracode(req.fromID)
	if req.profile != "" {
		return req, nil
	}
	profile, err := s.sessions.SessionProfile(req.fromID)
	if err != nil {
		return req, err
	}
	if profile == "" {
		profile = profilestore.Personal
	}
	req.profile = profile
	return req, nil
}

// recordProfile moves a session off the profile it was born in (its project's)
// before its terminal starts, which reads it back. A write that fails is
// reported once the session is running, for the reason recordOverrides gives.
func (s *Service) recordProfile(id, label, profile string) error {
	if profile == "" {
		return nil
	}
	if err := s.sessions.SetSessionProfile(id, profile); err != nil {
		return fmt.Errorf(
			"session %q is open, but its profile could not be recorded, so it runs in its "+
				"project's profile: %w",
			label, err,
		)
	}
	return nil
}
