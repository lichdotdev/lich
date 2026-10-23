package backend

import (
	"context"
	"errors"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// profileGoneText is what internal/claudeprofile.ErrProfileGone and
// internal/codexprofile.ErrProfileGone both end in, which a spawn in a profile
// deleted since answers.
const profileGoneText = "profile no longer exists"

type Profile = api.ProfileInfo

type ProfileUpdate = api.ProfileUpdate

type ProfilePlugin = api.ProfilePlugin

type ProfilePluginResult = api.ProfilePluginResult

type ProfileLogin = api.ProfileLogin

type ProfileChanged = api.ProfileChanged

type ProfileMoved = api.SessionProfile

// ProfileAPI is one provider's profile service: the claudeprofile, codexprofile
// or ompprofile one, which answer the same calls. A Codex login has no sign-in
// code to hand over, so LoginCode is Claude Code's alone, and Plugins and
// InstallPlugins are Claude Code's and Codex's. oh-my-pi's login is a terminal
// card's (LoginCommand), so it has no Login, LoginCode or CancelLogin, and no
// plan. A call the provider has no method for fails with ErrNoProfileCall.
type ProfileAPI struct {
	c         *Client
	methods   api.ProfileMethods
	login     *api.ProfileLoginMethods
	plugins   *api.ProfilePluginMethods
	loginCode *api.Method[api.LoginCodeOptions, api.None]
	plan      *api.Method[api.ProfilePlanOptions, api.QuotaPlan]
}

// ErrNoProfileCall is a profile call the provider's service does not answer.
var ErrNoProfileCall = errors.New("this provider's profiles have no such call")

// ClaudeProfiles is the Claude Code profile service.
func (c *Client) ClaudeProfiles() ProfileAPI {
	return ProfileAPI{
		c: c, methods: api.ClaudeProfiles,
		login: &api.ClaudeProfileLogin, plugins: &api.ClaudeProfilePlugins,
		loginCode: &api.ClaudeProfileLoginCode, plan: &api.QuotaProfilePlan,
	}
}

// CodexProfiles is the Codex profile service.
func (c *Client) CodexProfiles() ProfileAPI {
	return ProfileAPI{
		c: c, methods: api.CodexProfiles,
		login: &api.CodexProfileLogin, plugins: &api.CodexProfilePlugins, plan: &api.QuotaCodexProfilePlan,
	}
}

// OMPProfiles is the oh-my-pi profile service.
func (c *Client) OMPProfiles() ProfileAPI {
	return ProfileAPI{c: c, methods: api.OMPProfiles}
}

// XDGProfiles is the profile service of opencode, Cursor CLI or Kiro CLI, by
// provider id (xdgprofile.Service, "<id>profile").
func (c *Client) XDGProfiles(kind string) ProfileAPI {
	return ProfileAPI{c: c, methods: api.XDGProfiles[kind]}
}

// List lists every profile with its login state; the backend runs one CLI per
// profile to answer.
func (a ProfileAPI) List(ctx context.Context) ([]Profile, error) {
	return callMethod(ctx, a.c, a.methods.Profiles, api.None{})
}

// Create makes a profile named name and returns it; logging it in is Login's.
func (a ProfileAPI) Create(ctx context.Context, name string) (Profile, error) {
	p, err := callMethod(ctx, a.c, a.methods.CreateProfile, api.CreateProfileOptions{Name: name})
	return Profile{Profile: p}, err
}

// Update changes a profile's name, color or image and returns it as the backend
// now lists it, without its login state.
func (a ProfileAPI) Update(ctx context.Context, slug string, u ProfileUpdate) (Profile, error) {
	p, err := callMethod(ctx, a.c, a.methods.UpdateProfile, api.UpdateProfileOptions{Slug: slug, Update: u})
	return Profile{Profile: p}, err
}

// Delete removes a profile and its login; the backend refuses while a session
// runs in it.
func (a ProfileAPI) Delete(ctx context.Context, slug string) error {
	_, err := callMethod(ctx, a.c, a.methods.DeleteProfile, api.ProfileSlugOptions{Slug: slug})
	return err
}

// Plugins lists the plugins a profile has at user scope, the ones a port can
// carry to another profile. Claude Code and Codex only.
func (a ProfileAPI) Plugins(ctx context.Context, slug string) ([]ProfilePlugin, error) {
	if a.plugins == nil {
		return nil, ErrNoProfileCall
	}
	return callMethod(ctx, a.c, a.plugins.Plugins, api.ProfileSlugOptions{Slug: slug})
}

// InstallPlugins installs keys (plugin@marketplace) into a logged-in profile,
// one result per key. Claude Code and Codex only.
func (a ProfileAPI) InstallPlugins(ctx context.Context, slug string, keys []string) ([]ProfilePluginResult, error) {
	if a.plugins == nil {
		return nil, ErrNoProfileCall
	}
	return callMethod(ctx, a.c, a.plugins.InstallPlugins, api.InstallPluginsOptions{Slug: slug, Keys: keys})
}

// Login starts the provider's login as a profile and returns once the CLI
// runs; its progress arrives as ClaudeProfileLoginEvent or
// CodexProfileLoginEvent events.
func (a ProfileAPI) Login(ctx context.Context, slug string) error {
	if a.login == nil {
		return ErrNoProfileCall
	}
	_, err := callMethod(ctx, a.c, a.login.Login, api.ProfileSlugOptions{Slug: slug})
	return err
}

// LoginCode hands a running login the code the sign-in page showed. Claude Code
// only.
func (a ProfileAPI) LoginCode(ctx context.Context, slug, code string) error {
	if a.loginCode == nil {
		return ErrNoProfileCall
	}
	_, err := callMethod(ctx, a.c, *a.loginCode, api.LoginCodeOptions{Slug: slug, Code: code})
	return err
}

// CancelLogin ends a login lich started.
func (a ProfileAPI) CancelLogin(ctx context.Context, slug string) error {
	if a.login == nil {
		return ErrNoProfileCall
	}
	_, err := callMethod(ctx, a.c, a.login.CancelLogin, api.ProfileSlugOptions{Slug: slug})
	return err
}

// LoginCommand is the command line that logs a profile in from a terminal.
func (a ProfileAPI) LoginCommand(ctx context.Context, slug string) (string, error) {
	return callMethod(ctx, a.c, a.methods.LoginCommand, api.ProfileSlugOptions{Slug: slug})
}

// Plan reads one profile's plan; "personal" is the machine's own reading.
func (a ProfileAPI) Plan(ctx context.Context, slug string) (QuotaPlan, error) {
	if a.plan == nil {
		return QuotaPlan{}, ErrNoProfileCall
	}
	return callMethod(ctx, a.c, *a.plan, api.ProfilePlanOptions{Profile: slug})
}

// ContinueIn moves a session's conversation to another profile of its provider
// and carries on there. The backend refuses a turn in progress, and a provider
// whose conversations cannot move (opencode).
func (c *Client) ContinueIn(ctx context.Context, id, slug string) error {
	_, err := callMethod(ctx, c, api.TerminalContinueIn, api.ContinueInOptions{ID: id, Profile: slug})
	return err
}

// ProfileGone reports whether err is a spawn refused for a profile deleted
// since the session was born in it.
func ProfileGone(err error) bool {
	return err != nil && strings.Contains(ErrorText(err), profileGoneText)
}
