package codexprofile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/winexec"
)

const (
	// statusTimeout bounds `codex login status`, which reads local state and
	// must not hold the Settings screen behind a CLI that never answers.
	statusTimeout = 15 * time.Second
	waitDelay     = 2 * time.Second
	// notLoggedIn is what `codex login status` prints, in Codex's own words,
	// when nobody is logged in there (measured on 0.162.1, exit status 1).
	notLoggedIn = "not logged in"
)

// BinResolver supplies the Codex binary: the configured one, or "" for the
// default name on PATH. The store implements it.
type BinResolver interface {
	ProviderBin(providerID, projectID string) string
}

// Service is the window's door to the Codex profiles: listing them with their
// login state, creating, renaming and deleting them, and logging one in.
type Service struct {
	bins BinResolver
	emit func(name string, data any)

	// inUse counts the live sessions running in a profile, forget clears the
	// settings that name a deleted one and account names the login of a
	// profile. Startup wiring.
	inUse   func(slug string) int
	forget  func(slug string) error
	account func(slug string) (email, plan string)

	mu     sync.Mutex
	logins map[string]*login
}

// New returns a Service that runs the Codex binary bins resolves and reports a
// login's progress through emit.
func New(bins BinResolver, emit func(name string, data any)) *Service {
	return &Service{bins: bins, emit: emit, logins: map[string]*login{}}
}

// SetSessions wires the two questions a delete asks: how many live sessions run
// in a profile, and how to forget the settings that name it. Startup wiring.
func (s *Service) SetSessions(inUse func(slug string) int, forget func(slug string) error) {
	s.inUse, s.forget = inUse, forget
}

// SetAccount wires the question of whose login a profile holds and on which
// plan: the quota service's reading of the profile's own directory
// (quota.CodexProfilePlan), which asks upstream and is cached there. Startup
// wiring; without it a logged-in profile carries no account.
func (s *Service) SetAccount(account func(slug string) (email, plan string)) {
	s.account = account
}

// Info is a profile as the Settings screen shows it: whether anybody is logged
// in there, and the account and plan when they could be read.
type Info struct {
	Profile
	LoggedIn bool   `json:"loggedIn"`
	Email    string `json:"email,omitempty"`
	Plan     string `json:"plan,omitempty"`
	// StatusError is why `codex login status` gave no answer, in its own words;
	// LoggedIn is then false for want of one.
	StatusError string `json:"statusError,omitempty"`
}

// Profiles lists every profile with its login state. The statuses are asked for
// at once, one CLI each.
func (s *Service) Profiles() ([]Info, error) {
	profiles, err := List()
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
	loggedIn, err := s.loggedIn(p)
	if err != nil {
		return Info{Profile: p, StatusError: err.Error()}
	}
	info := Info{Profile: p, LoggedIn: loggedIn}
	if loggedIn && s.account != nil {
		info.Email, info.Plan = s.account(p.Slug)
	}
	return info
}

// loggedIn asks `codex login status`: exit status 0 is a login, and a refusal
// that says nobody is logged in is the answer "no". Any other failure is an
// error, which keeps a broken config.toml from reading as a logged-out profile.
func (s *Service) loggedIn(p Profile) (bool, error) {
	out, err := s.run(p, statusTimeout, "login", "status")
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && strings.Contains(strings.ToLower(out), notLoggedIn) {
		return false, nil
	}
	return false, fmt.Errorf("%s login status: %w: %s", s.bin(), err, strings.TrimSpace(out))
}

// CreateProfile makes a profile and lays it out against Personal. The window
// logs it in next (Login).
func (s *Service) CreateProfile(name string) (Profile, error) { return Create(name) }

// RenameProfile changes a profile's display name; its directory never moves.
func (s *Service) RenameProfile(slug, name string) error { return Rename(slug, name) }

// DeleteProfile removes a profile and its login, and clears every setting that
// named it, so a project set to it falls back to the global profile. It refuses
// while a session runs in it. A parked session born in it fails to resume with
// ErrProfileGone.
func (s *Service) DeleteProfile(slug string) error {
	if s.inUse != nil {
		if n := s.inUse(slug); n > 0 {
			return fmt.Errorf("profile %q has %d running session(s); close them before deleting it", slug, n)
		}
	}
	s.CancelLogin(slug)
	if err := Delete(slug); err != nil {
		return err
	}
	if s.forget != nil {
		return s.forget(slug)
	}
	return nil
}

// run executes the Codex CLI as a profile and returns its combined output.
func (s *Service) run(p Profile, timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := s.command(ctx, p, args...)
	out, err := cmd.CombinedOutput()
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		err = fmt.Errorf("timed out after %s", timeout)
	}
	return string(out), err
}

// command is the CLI run as a profile: Personal inherits lich's environment
// untouched, the way its sessions do; any other profile gets its own directory
// and none of the variables that would override its login.
func (s *Service) command(ctx context.Context, p Profile, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, s.bin(), args...)
	winexec.Hide(cmd)
	cmd.WaitDelay = waitDelay
	if !p.Personal {
		cmd.Env = Env(os.Environ(), p.Dir)
	}
	return cmd
}

func (s *Service) bin() string {
	if bin := s.bins.ProviderBin(providers.Codex, ""); bin != "" {
		return bin
	}
	return providers.DefaultBinary(providers.Codex)
}
