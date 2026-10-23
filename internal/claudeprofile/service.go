package claudeprofile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
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
	// statusTimeout bounds `claude auth status`, which reads local state (and the
	// Keychain on macOS) and must not hold the Settings screen behind a CLI that
	// never answers.
	statusTimeout = 15 * time.Second
	// installTimeout bounds one `claude plugin install`, the cap the lich plugin's
	// own install uses (internal/agentplugin): a marketplace refresh clones.
	installTimeout = 130 * time.Second
	waitDelay      = 2 * time.Second
)

// pluginKeyPattern is a plugin@marketplace key as Claude Code spells one. It is
// checked before the key reaches a command line, so nothing that arrives over
// RPC can be read as a flag.
var pluginKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*@[A-Za-z0-9][A-Za-z0-9._-]*$`)

// BinResolver supplies the Claude Code binary: the configured one, or "" for
// the default name on PATH. The store implements it.
type BinResolver interface {
	ProviderBin(providerID, projectID string) string
}

// Service is the window's door to the profiles: listing them with their
// accounts, creating, renaming and deleting them, logging one in, and porting
// plugins between them.
type Service struct {
	bins BinResolver
	emit func(name string, data any)

	// installLich puts the lich plugin into a profile once it is logged in
	// (internal/agentplugin), inUse counts the live sessions running in one, and
	// forget clears the settings that name a deleted one. Startup wiring.
	installLich func(slug string) error
	inUse       func(slug string) int
	forget      func(slug string) error
	// lichPlugin is the lich plugin's key, left out of a port: every profile
	// gets it from installLich, pinned to the release this lich speaks.
	lichPlugin string

	mu     sync.Mutex
	logins map[string]*login
}

// New returns a Service that runs the Claude Code binary bins resolves and
// reports a login's progress through emit.
func New(bins BinResolver, emit func(name string, data any)) *Service {
	return &Service{bins: bins, emit: emit, logins: map[string]*login{}}
}

// SetLichPlugin wires the lich plugin: install puts it into a profile and key is
// the plugin@marketplace it installs. Startup wiring.
func (s *Service) SetLichPlugin(install func(slug string) error, key string) {
	s.installLich, s.lichPlugin = install, key
}

// SetSessions wires the two questions a delete asks: how many live sessions run
// in a profile, and how to forget the settings that name it. Startup wiring.
func (s *Service) SetSessions(inUse func(slug string) int, forget func(slug string) error) {
	s.inUse, s.forget = inUse, forget
}

type Info = api.ProfileInfo

// authStatus is the JSON `claude auth status` prints on stdout (measured on
// 2.1.296), on a logged-out profile too, exiting 1.
type authStatus struct {
	LoggedIn         bool   `json:"loggedIn"`
	AuthMethod       string `json:"authMethod"`
	Email            string `json:"email"`
	OrgName          string `json:"orgName"`
	SubscriptionType string `json:"subscriptionType"`
}

// Profiles lists every profile with its account. The statuses are asked for at
// once, one CLI each.
func (s *Service) Profiles(_ context.Context, _ api.None) ([]Info, error) {
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
	info := Info{Profile: p, Plugins: len(installedPlugins(PluginsDir(p.Slug, p.Dir)))}
	st, err := s.authStatus(p)
	if err != nil {
		info.StatusError = err.Error()
		return info
	}
	info.LoggedIn, info.AuthMethod = st.LoggedIn, st.AuthMethod
	info.Email, info.OrgName, info.Plan = st.Email, st.OrgName, st.SubscriptionType
	return info
}

func (s *Service) authStatus(p Profile) (authStatus, error) {
	var st authStatus
	ctx, cancel := context.WithTimeout(context.Background(), statusTimeout)
	defer cancel()
	cmd := s.command(ctx, p, "auth", "status")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	err = explainTimeout(ctx, err, statusTimeout)
	if jsonErr := json.Unmarshal(out, &st); jsonErr != nil {
		if err == nil {
			err = jsonErr
		}
		words := strings.TrimSpace(string(out) + "\n" + stderr.String())
		return st, fmt.Errorf("%s auth status: %w: %s", s.bin(), err, words)
	}
	return st, nil
}

// CreateProfile makes a profile and lays it out against Personal. The window
// logs it in next (Login).
func (s *Service) CreateProfile(_ context.Context, opts api.CreateProfileOptions) (Profile, error) {
	return Create(opts.Name)
}

// UpdateProfile changes a profile's name, color or image and returns it; its
// directory never moves.
func (s *Service) UpdateProfile(_ context.Context, opts api.UpdateProfileOptions) (Profile, error) {
	p, err := Update(opts.Slug, opts.Update)
	if err != nil {
		return p, err
	}
	events.Send(s.emit, api.ProfileChangedEvent, profilestore.Changed{Provider: store.Provider, Profile: p})
	return p, nil
}

// DeleteProfile removes a profile and its login, and clears every setting that
// named it, so a project set to it falls back to the global profile. It refuses
// while a session runs in it. A parked session born in it fails to resume with
// ErrProfileGone.
func (s *Service) DeleteProfile(ctx context.Context, opts api.ProfileSlugOptions) (api.None, error) {
	if s.inUse != nil {
		if n := s.inUse(opts.Slug); n > 0 {
			return api.None{}, fmt.Errorf("profile %q has %d running session(s); close them before deleting it", opts.Slug, n)
		}
	}
	s.cancelLogin(opts.Slug)
	if err := Delete(opts.Slug); err != nil {
		return api.None{}, err
	}
	if s.forget != nil {
		return api.None{}, s.forget(opts.Slug)
	}
	return api.None{}, nil
}

type Plugin = api.ProfilePlugin

// Plugins lists the plugins a profile has installed at user scope, the ones a
// port can carry to another profile. The lich plugin is not among them.
func (s *Service) Plugins(_ context.Context, opts api.ProfileSlugOptions) ([]Plugin, error) {
	dir, err := ConfigDir(opts.Slug)
	if err != nil {
		return nil, err
	}
	out := []Plugin{}
	for _, p := range installedPlugins(PluginsDir(opts.Slug, dir)) {
		if p.Key != s.lichPlugin {
			out = append(out, p)
		}
	}
	return out, nil
}

// installedPlugins reads the installed_plugins.json in a profile's plugins
// directory for its user-scope plugins, sorted by key. A file that is missing or
// does not parse lists none.
func installedPlugins(plugins string) []Plugin {
	data, err := os.ReadFile(filepath.Join(plugins, "installed_plugins.json"))
	if err != nil {
		return nil
	}
	var doc struct {
		Plugins map[string][]struct {
			Scope   string `json:"scope"`
			Version string `json:"version"`
		} `json:"plugins"`
	}
	if json.Unmarshal(data, &doc) != nil {
		return nil
	}
	var out []Plugin
	for key, installs := range doc.Plugins {
		for _, in := range installs {
			if in.Scope != "user" {
				continue
			}
			name, market, _ := strings.Cut(key, "@")
			out = append(out, Plugin{Key: key, Name: name, Marketplace: market, Version: in.Version})
			break
		}
	}
	slices.SortFunc(out, func(a, b Plugin) int { return strings.Compare(a.Key, b.Key) })
	return out
}

type PluginResult = api.ProfilePluginResult

// InstallPlugins installs each plugin@marketplace into a profile through Claude
// Code's own `plugin install`, run as that profile so its account's policy
// applies and Claude Code writes its own paths. One result per key, in order;
// one refusal does not stop the rest. Run it after the profile is logged in.
//
// A plugin installed by running a command its marketplace declares is refused
// rather than confirmed: lich never answers that question for the user.
func (s *Service) InstallPlugins(_ context.Context, opts api.InstallPluginsOptions) ([]PluginResult, error) {
	dir, err := ConfigDir(opts.Slug)
	if err != nil {
		return nil, err
	}
	p := Profile{Slug: opts.Slug, Dir: dir, Personal: IsPersonal(opts.Slug)}
	results := make([]PluginResult, 0, len(opts.Keys))
	for _, key := range opts.Keys {
		if !pluginKeyPattern.MatchString(key) {
			results = append(results, PluginResult{Key: key, Message: "not a plugin@marketplace name"})
			continue
		}
		out, err := s.run(p, installTimeout, "plugin", "install", key)
		r := PluginResult{Key: key, OK: err == nil}
		if err != nil {
			r.Message = strings.TrimSpace(out)
			if r.Message == "" {
				r.Message = err.Error()
			}
		}
		results = append(results, r)
	}
	return results, nil
}

// run executes the Claude Code CLI as a profile and returns its combined output.
func (s *Service) run(p Profile, timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := s.command(ctx, p, args...)
	out, err := cmd.CombinedOutput()
	return string(out), explainTimeout(ctx, err, timeout)
}

func explainTimeout(ctx context.Context, err error, timeout time.Duration) error {
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("timed out after %s", timeout)
	}
	return err
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
	if bin := s.bins.ProviderBin(providers.Claude, ""); bin != "" {
		return bin
	}
	return providers.DefaultBinary(providers.Claude)
}
