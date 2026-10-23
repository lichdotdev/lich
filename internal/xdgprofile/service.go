package xdgprofile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/profilestore"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/winexec"

	"github.com/lichdotdev/lich/internal/events"
)

const (
	// statusTimeout bounds a status call, which must not hold the Settings
	// screen behind a CLI that never answers.
	statusTimeout = 15 * time.Second
	waitDelay     = 2 * time.Second
)

// BinResolver supplies the provider's binary: the configured one, or "" for the
// default name on PATH. The store implements it.
type BinResolver interface {
	ProviderBin(providerID, projectID string) string
}

// Service is the window's door to one provider's profiles: listing them with
// their logins, creating, renaming and deleting them, and the command that logs
// one in. Every login of the three is an interactive picker or a browser flow
// that prints what to do, so a login is only ever offered in a terminal card,
// as oh-my-pi's is.
type Service struct {
	provider *Provider
	bins     BinResolver
	emit     func(name string, data any)

	// inUse counts the live sessions running in a profile and forget clears the
	// settings that name a deleted one. Startup wiring.
	inUse  func(slug string) int
	forget func(slug string) error
}

// New returns a Service for provider that runs the binary bins resolves and
// reports a profile's update through emit.
func New(provider *Provider, bins BinResolver, emit func(name string, data any)) *Service {
	return &Service{provider: provider, bins: bins, emit: emit}
}

// SetSessions wires the two questions a delete asks: how many live sessions run
// in a profile, and how to forget the settings that name it. Startup wiring.
func (s *Service) SetSessions(inUse func(slug string) int, forget func(slug string) error) {
	s.inUse, s.forget = inUse, forget
}

type Info = api.ProfileInfo

// Profiles lists every profile with its login. The statuses are asked for at
// once, one CLI each.
func (s *Service) Profiles(_ context.Context, _ api.None) ([]Info, error) {
	profiles, err := s.provider.List()
	if err != nil {
		return nil, err
	}
	out := make([]Info, len(profiles))
	var wg sync.WaitGroup
	for i, p := range profiles {
		wg.Go(func() { out[i] = s.info(p) })
	}
	wg.Wait()
	return out, nil
}

func (s *Service) info(p Profile) Info {
	status := s.provider.status
	out, runErr := s.run(p, status.args...)
	found, err := status.read(out)
	if err != nil {
		return Info{Profile: p, StatusError: fmt.Sprintf("%s %s: %v: %s",
			s.bin(), strings.Join(status.args, " "), errors.Join(runErr, err), strings.TrimSpace(out))}
	}
	return Info{Profile: p, LoggedIn: found.loggedIn, Email: found.email}
}

// CreateProfile makes a profile and lays its link home out. The window logs it
// in next, in a terminal card (LoginCommand).
func (s *Service) CreateProfile(_ context.Context, opts api.CreateProfileOptions) (Profile, error) {
	return s.provider.Create(opts.Name)
}

// UpdateProfile changes a profile's name, color or image and returns it; its
// directory never moves.
func (s *Service) UpdateProfile(_ context.Context, opts api.UpdateProfileOptions) (Profile, error) {
	p, err := s.provider.Update(opts.Slug, opts.Update)
	if err != nil {
		return p, err
	}
	events.Send(s.emit, api.ProfileChangedEvent, profilestore.Changed{Provider: s.provider.ID, Profile: p})
	return p, nil
}

// DeleteProfile removes a profile and its login, and clears every setting that
// named it. It refuses while a session runs in it.
func (s *Service) DeleteProfile(_ context.Context, opts api.ProfileSlugOptions) (api.None, error) {
	if s.inUse != nil {
		if n := s.inUse(opts.Slug); n > 0 {
			return api.None{}, fmt.Errorf("profile %q has %d running session(s); close them before deleting it", opts.Slug, n)
		}
	}
	if err := s.provider.Delete(opts.Slug); err != nil {
		return api.None{}, err
	}
	if s.forget != nil {
		return api.None{}, s.forget(opts.Slug)
	}
	return api.None{}, nil
}

// LoginCommand is the command line that logs a profile in from a terminal.
func (s *Service) LoginCommand(_ context.Context, opts api.ProfileSlugOptions) (string, error) {
	dir, err := s.provider.ConfigDir(opts.Slug)
	if err != nil {
		return "", err
	}
	value := ""
	if !IsPersonal(opts.Slug) {
		value = dir
	}
	return s.provider.store.LoginLine(s.bin(), s.provider.login, value), nil
}

// IsPersonal reports whether slug names the default profile. Empty does too.
func IsPersonal(slug string) bool { return profilestore.IsPersonal(slug) }

// run executes the CLI as a profile and returns its combined output.
func (s *Service) run(p Profile, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), statusTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.bin(), args...)
	winexec.Hide(cmd)
	cmd.WaitDelay = waitDelay
	if !p.Personal {
		cmd.Env = s.provider.Env(os.Environ(), p.Dir)
	}
	out, err := cmd.CombinedOutput()
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		err = fmt.Errorf("timed out after %s", statusTimeout)
	}
	return string(out), err
}

func (s *Service) bin() string {
	if bin := s.bins.ProviderBin(s.provider.ID, ""); bin != "" {
		return bin
	}
	return providers.DefaultBinary(s.provider.ID)
}
