package ompprofile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/lichdotdev/lich/internal/profilestore"
	"github.com/lichdotdev/lich/internal/providers"
	"github.com/lichdotdev/lich/internal/winexec"
)

const (
	// statusTimeout bounds `omp usage accounts`, which reads local state and
	// must not hold the Settings screen behind a CLI that never answers.
	statusTimeout = 15 * time.Second
	waitDelay     = 2 * time.Second
)

// BinResolver supplies the omp binary: the configured one, or "" for the
// default name on PATH. The store implements it.
type BinResolver interface {
	ProviderBin(providerID, projectID string) string
}

// Service is the window's door to the oh-my-pi profiles: listing them with
// their logins, creating, renaming and deleting them, and the command that logs
// one in. There is no login lich runs itself: `omp login` is an interactive
// picker of providers, so a login is only ever offered in a terminal card.
type Service struct {
	bins BinResolver
	emit func(name string, data any)

	// inUse counts the live sessions running in a profile and forget clears the
	// settings that name a deleted one. Startup wiring.
	inUse  func(slug string) int
	forget func(slug string) error
}

// New returns a Service that runs the omp binary bins resolves and reports a
// profile's update through emit.
func New(bins BinResolver, emit func(name string, data any)) *Service {
	return &Service{bins: bins, emit: emit}
}

// SetSessions wires the two questions a delete asks: how many live sessions run
// in a profile, and how to forget the settings that name it. Startup wiring.
func (s *Service) SetSessions(inUse func(slug string) int, forget func(slug string) error) {
	s.inUse, s.forget = inUse, forget
}

// Info is a profile as the Settings screen shows it: whether any account is
// logged in there, and which.
type Info struct {
	Profile
	LoggedIn bool `json:"loggedIn"`
	// Email is the address of each OAuth account, comma separated. omp keeps
	// several logins per profile, one per provider or more, so there is no one
	// account to name.
	Email string `json:"email,omitempty"`
	// Providers are the providers omp holds an OAuth login for here.
	Providers []string `json:"providers,omitempty"`
	// StatusError is why `omp usage accounts` gave no answer, in its own words;
	// LoggedIn is then false for want of one.
	StatusError string `json:"statusError,omitempty"`
}

// accounts is the JSON `omp usage accounts --json` prints (measured on 18.8.9):
// one row per OAuth login in the profile, API keys left out. identityKey reads
// "email:<address>|org:<id>" for an org-scoped provider, and may be null.
type accounts struct {
	Accounts []struct {
		Provider    string  `json:"provider"`
		IdentityKey *string `json:"identityKey"`
	} `json:"accounts"`
}

// Profiles lists every profile with its logins. The statuses are asked for at
// once, one CLI each.
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
	out, stderr, err := s.run(p, statusTimeout, "usage", "accounts", "--json")
	var found accounts
	if err == nil {
		err = json.Unmarshal([]byte(out), &found)
	}
	if err != nil {
		words := strings.TrimSpace(out + "\n" + stderr)
		return Info{Profile: p, StatusError: fmt.Sprintf("%s usage accounts: %v: %s", s.bin(), err, words)}
	}
	info := Info{Profile: p, LoggedIn: len(found.Accounts) > 0}
	var emails []string
	for _, a := range found.Accounts {
		if !slices.Contains(info.Providers, a.Provider) {
			info.Providers = append(info.Providers, a.Provider)
		}
		if a.IdentityKey == nil {
			continue
		}
		if email := emailOf(*a.IdentityKey); email != "" && !slices.Contains(emails, email) {
			emails = append(emails, email)
		}
	}
	info.Email = strings.Join(emails, ", ")
	return info
}

// emailOf is the address in an identity key: its "email:" part.
func emailOf(identityKey string) string {
	for part := range strings.SplitSeq(identityKey, "|") {
		if email, ok := strings.CutPrefix(part, "email:"); ok {
			return email
		}
	}
	return ""
}

// CreateProfile makes a profile and lays it out against Personal. The window
// logs it in next, in a terminal card (LoginCommand).
func (s *Service) CreateProfile(name string) (Profile, error) { return Create(name) }

// UpdateProfile changes a profile's name, color or image and returns it; its
// directory never moves.
func (s *Service) UpdateProfile(slug string, u profilestore.Update) (Profile, error) {
	p, err := Update(slug, u)
	if err != nil {
		return p, err
	}
	s.emit(profilestore.ChangedEvent, profilestore.Changed{Provider: store.Provider, Profile: p})
	return p, nil
}

// DeleteProfile removes a profile and its logins, and clears every setting that
// named it, so a project set to it falls back to the global profile. It refuses
// while a session runs in it. A parked session born in it fails to resume with
// ErrProfileGone.
func (s *Service) DeleteProfile(slug string) error {
	if s.inUse != nil {
		if n := s.inUse(slug); n > 0 {
			return fmt.Errorf("profile %q has %d running session(s); close them before deleting it", slug, n)
		}
	}
	if err := Delete(slug); err != nil {
		return err
	}
	if s.forget != nil {
		return s.forget(slug)
	}
	return nil
}

// LoginCommand is the command line that logs a profile in from a terminal: it
// opens omp's own picker of providers.
func (s *Service) LoginCommand(slug string) (string, error) {
	dir, err := ConfigDir(slug)
	if err != nil {
		return "", err
	}
	value := ""
	if !IsPersonal(slug) {
		value = slugOf(dir)
	}
	return store.LoginLine(s.bin(), "login", value), nil
}

// run executes the omp CLI as a profile and returns its stdout and stderr.
func (s *Service) run(p Profile, timeout time.Duration, args ...string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.bin(), args...)
	winexec.Hide(cmd)
	cmd.WaitDelay = waitDelay
	if !p.Personal {
		cmd.Env = Env(os.Environ(), p.Dir)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		err = fmt.Errorf("timed out after %s", timeout)
	}
	return string(out), stderr.String(), err
}

func (s *Service) bin() string {
	if bin := s.bins.ProviderBin(providers.OMP, ""); bin != "" {
		return bin
	}
	return providers.DefaultBinary(providers.OMP)
}
