package codexprofile

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	// statusTimeout bounds `codex login status`, which reads local state and
	// must not hold the Settings screen behind a CLI that never answers.
	statusTimeout = 15 * time.Second
	waitDelay     = 2 * time.Second
	// notLoggedIn is what `codex login status` prints, in Codex's own words,
	// when nobody is logged in there (measured on 0.162.1, exit status 1).
	notLoggedIn = "not logged in"
	// installTimeout bounds one `codex plugin` call of a port, the cap the lich
	// plugin's own install uses (internal/agentplugin): a marketplace upgrade
	// clones.
	installTimeout = 130 * time.Second
)

// pluginKeyPattern is a plugin@marketplace key. It is checked before the key
// reaches a command line, so nothing that arrives over RPC can be read as a
// flag.
var pluginKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*@[A-Za-z0-9][A-Za-z0-9._-]*$`)

// BinResolver supplies the Codex binary: the configured one, or "" for the
// default name on PATH. The store implements it.
type BinResolver interface {
	ProviderBin(providerID, projectID string) string
}

// Service is the window's door to the Codex profiles: listing them with their
// login state, creating, renaming and deleting them, logging one in, and
// porting plugins between them.
type Service struct {
	bins BinResolver
	emit func(name string, data any)

	// installLich puts the lich plugin into a profile once it is logged in
	// (internal/agentplugin), inUse counts the live sessions running in one,
	// forget clears the settings that name a deleted one and account names the
	// login of a profile. Startup wiring.
	installLich func(slug string) error
	inUse       func(slug string) int
	forget      func(slug string) error
	account     func(slug string) (email, plan string)
	// lichPlugin is the lich plugin's key, left out of a port: every profile
	// gets it from installLich, pinned to the release this lich speaks.
	lichPlugin string

	mu     sync.Mutex
	logins map[string]*login
}

// New returns a Service that runs the Codex binary bins resolves and reports a
// login's progress through emit.
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

// SetAccount wires the question of whose login a profile holds and on which
// plan: the quota service's reading of the profile's own directory
// (quota.CodexProfilePlan), which asks upstream and is cached there. Startup
// wiring; without it a logged-in profile carries no account.
func (s *Service) SetAccount(account func(slug string) (email, plan string)) {
	s.account = account
}

type Info = api.ProfileInfo

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
	info := Info{Profile: p}
	if config, err := readOptional(filepath.Join(p.Dir, configFile)); err == nil {
		info.Plugins = len(installedPlugins(config))
	}
	loggedIn, err := s.loggedIn(p)
	if err != nil {
		info.StatusError = err.Error()
		return info
	}
	info.LoggedIn = loggedIn
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

// UpdateProfile changes a profile's name, color or image and returns it; its
// directory never moves.
func (s *Service) UpdateProfile(slug string, u profilestore.Update) (Profile, error) {
	p, err := Update(slug, u)
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

type Plugin = api.ProfilePlugin

// Plugins lists the plugins a profile has installed, the ones a port can carry
// to another profile, in its config.toml's order. The lich plugin is not among
// them.
func (s *Service) Plugins(slug string) ([]Plugin, error) {
	dir, err := ConfigDir(slug)
	if err != nil {
		return nil, err
	}
	config, err := readOptional(filepath.Join(dir, configFile))
	if err != nil {
		return nil, err
	}
	out := []Plugin{}
	for _, key := range installedPlugins(config) {
		if key == s.lichPlugin {
			continue
		}
		name, market, _ := strings.Cut(key, "@")
		out = append(out, Plugin{Key: key, Name: name, Marketplace: market})
	}
	return out, nil
}

type PluginResult = api.ProfilePluginResult

// InstallPlugins installs each plugin@marketplace into a profile through
// Codex's own `plugin add`, run as that profile so Codex writes its own paths.
// A marketplace the profile has not declared is declared as Personal declares
// it, its ref included, and fetched first. One result per key, in order; one
// failure does not stop the rest.
func (s *Service) InstallPlugins(slug string, keys []string) ([]PluginResult, error) {
	dir, err := ConfigDir(slug)
	if err != nil {
		return nil, err
	}
	personal, err := PersonalDir()
	if err != nil {
		return nil, err
	}
	p := Profile{Slug: slug, Dir: dir, Personal: IsPersonal(slug)}
	results := make([]PluginResult, 0, len(keys))
	for _, key := range keys {
		if !pluginKeyPattern.MatchString(key) {
			results = append(results, PluginResult{Key: key, Message: "not a plugin@marketplace name"})
			continue
		}
		results = append(results, s.installPlugin(p, personal, key))
	}
	return results, nil
}

func (s *Service) installPlugin(p Profile, personal, key string) PluginResult {
	_, market, _ := strings.Cut(key, "@")
	declared, err := declareMarketplace(personal, p.Dir, market)
	if err != nil {
		return PluginResult{Key: key, Message: err.Error()}
	}
	var failures []string
	if declared {
		// A declared marketplace has no snapshot until it is fetched, and
		// `plugin add` refuses one without (measured on 0.162.1). A local
		// marketplace has nothing to fetch and refuses the upgrade, which only
		// matters if the add fails too.
		if out, err := s.run(p, installTimeout, "plugin", "marketplace", "upgrade", market); err != nil {
			failures = append(failures, failureText(out, err))
		}
	}
	out, err := s.run(p, installTimeout, "plugin", "add", key)
	if err == nil {
		return PluginResult{Key: key, OK: true}
	}
	return PluginResult{Key: key, Message: strings.Join(append(failures, failureText(out, err)), "\n")}
}

// declareMarketplace copies Personal's declaration of the marketplace name into
// a profile's config.toml that has none, and reports whether it did. A
// marketplace Personal does not declare is left to Codex, which knows its own.
func declareMarketplace(personal, dir, name string) (bool, error) {
	path := filepath.Join(dir, configFile)
	own, err := readOptional(path)
	if err != nil || marketplaceTables(own, name) != "" {
		return false, err
	}
	shared, err := readOptional(filepath.Join(personal, configFile))
	if err != nil {
		return false, err
	}
	declaration := marketplaceTables(shared, name)
	if declaration == "" {
		return false, nil
	}
	var out bytes.Buffer
	out.Write(own)
	writeTable(&out, declaration)
	return true, profilestore.WriteFileAtomic(path, out.Bytes(), 0o600)
}

// failureText is a CLI's own words for a failure, or the error when it said
// nothing.
func failureText(out string, err error) string {
	if text := strings.TrimSpace(out); text != "" {
		return text
	}
	return err.Error()
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
