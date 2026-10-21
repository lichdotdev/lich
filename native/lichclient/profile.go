package lichclient

import (
	"context"
	"strings"
)

const (
	// ClaudeProfileLoginEvent and CodexProfileLoginEvent are the events a
	// profile login reports its progress on (claudeprofile.LoginEventName,
	// codexprofile.LoginEventName).
	ClaudeProfileLoginEvent = "claude-profile-login"
	CodexProfileLoginEvent  = "codex-profile-login"

	// SessionProfileEvent is the event a session moving to another profile
	// emits (terminal profileEventName).
	SessionProfileEvent = "session-profile"

	// ProfileChangedEvent is the event a profile's update emits once it is
	// written (profilestore.ChangedEvent).
	ProfileChangedEvent = "profile-changed"
)

// profileGoneText is what internal/claudeprofile.ErrProfileGone and
// internal/codexprofile.ErrProfileGone both end in, which a spawn in a profile
// deleted since answers.
const profileGoneText = "profile no longer exists"

// Profile mirrors claudeprofile.Info, codexprofile.Info and ompprofile.Info: a
// login lich keeps a state directory for. Slug is "personal" for the default
// profile. Email, OrgName and Plan are what `claude auth status` found there;
// a Codex profile carries Email and Plan from its plan reading, and an oh-my-pi
// one Email (every account's address) and Providers. Plugins counts a Claude
// Code or Codex profile's plugins.
type Profile struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Personal bool   `json:"personal"`
	// Copied names the shared entries the profile holds as copies rather than
	// links (Windows without Developer Mode).
	Copied   []string `json:"copied,omitempty"`
	LoggedIn bool     `json:"loggedIn"`
	Email    string   `json:"email,omitempty"`
	OrgName  string   `json:"orgName,omitempty"`
	Plan     string   `json:"plan,omitempty"`
	Plugins  int      `json:"plugins"`
	// Providers are the providers an oh-my-pi profile holds a login for.
	Providers   []string `json:"providers,omitempty"`
	StatusError string   `json:"statusError,omitempty"`
	// Color is a card color's name, "" for the one the window derives from the
	// slug; Image is the profile's picture, a square PNG.
	Color string `json:"color,omitempty"`
	Image []byte `json:"image,omitempty"`
}

// ProfileUpdate mirrors profilestore.Update: a nil field keeps what the profile
// has, and an empty Name keeps its name. Color "" goes back to the slug's own
// color; Image is a file to read, "" removes the image.
type ProfileUpdate struct {
	Name  string  `json:"name,omitempty"`
	Color *string `json:"color,omitempty"`
	Image *string `json:"image,omitempty"`
}

// ProfilePlugin mirrors claudeprofile.Plugin and codexprofile.Plugin.
type ProfilePlugin struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Marketplace string `json:"marketplace"`
}

// ProfilePluginResult mirrors claudeprofile.PluginResult and
// codexprofile.PluginResult: Message is the CLI's own words when it refused one.
type ProfilePluginResult struct {
	Key     string `json:"key"`
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

// ProfileLogin mirrors claudeprofile.LoginEvent and codexprofile.LoginEvent:
// Output (and URL, when that text holds the sign-in address) while the CLI
// talks, Authenticating once it is through with the browser, then one event
// with Done set.
type ProfileLogin struct {
	Profile        string `json:"profile"`
	Output         string `json:"output,omitempty"`
	URL            string `json:"url,omitempty"`
	Authenticating bool   `json:"authenticating,omitempty"`
	Done           bool   `json:"done,omitempty"`
	LoggedIn       bool   `json:"loggedIn,omitempty"`
	Cancelled      bool   `json:"cancelled,omitempty"`
	Error          string `json:"error,omitempty"`
	PluginError    string `json:"pluginError,omitempty"`
}

// ProfileChanged mirrors profilestore.Changed: Provider is the provider id
// ("claude", "kiro"), Profile the profile as it now is, without its login
// state.
type ProfileChanged struct {
	Provider string  `json:"provider"`
	Profile  Profile `json:"profile"`
}

// ProfileMoved is the session-profile payload: Profile is the slug the session
// now runs in, "" for Personal.
type ProfileMoved struct {
	ID      string `json:"id"`
	Profile string `json:"profile"`
}

// ProfileAPI is one provider's profile service: the claudeprofile, codexprofile
// or ompprofile one, which answer the same calls. A Codex login has no sign-in
// code to hand over, so LoginCode is Claude Code's alone, and Plugins and
// InstallPlugins are Claude Code's and Codex's. oh-my-pi's login is a terminal
// card's (LoginCommand), so it has no Login, LoginCode or CancelLogin, and no
// plan.
type ProfileAPI struct {
	c       *Client
	service string
	plan    string
}

// ClaudeProfiles is the Claude Code profile service.
func (c *Client) ClaudeProfiles() ProfileAPI {
	return ProfileAPI{c: c, service: "claudeprofile", plan: "quota.ProfilePlan"}
}

// CodexProfiles is the Codex profile service.
func (c *Client) CodexProfiles() ProfileAPI {
	return ProfileAPI{c: c, service: "codexprofile", plan: "quota.CodexProfilePlan"}
}

// OMPProfiles is the oh-my-pi profile service.
func (c *Client) OMPProfiles() ProfileAPI {
	return ProfileAPI{c: c, service: "ompprofile"}
}

// XDGProfiles is the profile service of opencode, Cursor CLI or Kiro CLI, by
// provider id (xdgprofile.Service, "<id>profile").
func (c *Client) XDGProfiles(kind string) ProfileAPI {
	return ProfileAPI{c: c, service: kind + "profile"}
}

// List lists every profile with its login state; the backend runs one CLI per
// profile to answer.
func (a ProfileAPI) List(ctx context.Context) ([]Profile, error) {
	var list []Profile
	return list, a.c.Call(ctx, a.service+".Profiles", &list)
}

// Create makes a profile named name and returns it; logging it in is Login's.
func (a ProfileAPI) Create(ctx context.Context, name string) (Profile, error) {
	var p Profile
	return p, a.c.Call(ctx, a.service+".CreateProfile", &p, name)
}

// Update changes a profile's name, color or image and returns it as the backend
// now lists it, without its login state.
func (a ProfileAPI) Update(ctx context.Context, slug string, u ProfileUpdate) (Profile, error) {
	var p Profile
	return p, a.c.Call(ctx, a.service+".UpdateProfile", &p, slug, u)
}

// Delete removes a profile and its login; the backend refuses while a session
// runs in it.
func (a ProfileAPI) Delete(ctx context.Context, slug string) error {
	return a.c.Call(ctx, a.service+".DeleteProfile", nil, slug)
}

// Plugins lists the plugins a profile has at user scope, the ones a port can
// carry to another profile. Claude Code and Codex only.
func (a ProfileAPI) Plugins(ctx context.Context, slug string) ([]ProfilePlugin, error) {
	var list []ProfilePlugin
	return list, a.c.Call(ctx, a.service+".Plugins", &list, slug)
}

// InstallPlugins installs keys (plugin@marketplace) into a logged-in profile,
// one result per key. Claude Code and Codex only.
func (a ProfileAPI) InstallPlugins(ctx context.Context, slug string, keys []string) ([]ProfilePluginResult, error) {
	var results []ProfilePluginResult
	return results, a.c.Call(ctx, a.service+".InstallPlugins", &results, slug, keys)
}

// Login starts the provider's login as a profile and returns once the CLI
// runs; its progress arrives as ClaudeProfileLoginEvent or
// CodexProfileLoginEvent events.
func (a ProfileAPI) Login(ctx context.Context, slug string) error {
	return a.c.Call(ctx, a.service+".Login", nil, slug)
}

// LoginCode hands a running login the code the sign-in page showed. Claude Code
// only.
func (a ProfileAPI) LoginCode(ctx context.Context, slug, code string) error {
	return a.c.Call(ctx, a.service+".LoginCode", nil, slug, code)
}

// CancelLogin ends a login lich started.
func (a ProfileAPI) CancelLogin(ctx context.Context, slug string) error {
	return a.c.Call(ctx, a.service+".CancelLogin", nil, slug)
}

// LoginCommand is the command line that logs a profile in from a terminal.
func (a ProfileAPI) LoginCommand(ctx context.Context, slug string) (string, error) {
	var command string
	return command, a.c.Call(ctx, a.service+".LoginCommand", &command, slug)
}

// Plan reads one profile's plan; "personal" is the machine's own reading.
func (a ProfileAPI) Plan(ctx context.Context, slug string) (QuotaPlan, error) {
	var plan QuotaPlan
	return plan, a.c.Call(ctx, a.plan, &plan, slug)
}

// ContinueIn moves a session's conversation to another profile of its provider
// and carries on there. The backend refuses a turn in progress, and a provider
// whose conversations cannot move (opencode).
func (c *Client) ContinueIn(ctx context.Context, id, slug string) error {
	return c.Call(ctx, "terminal.ContinueIn", nil, id, slug)
}

// ProfileGone reports whether err is a spawn refused for a profile deleted
// since the session was born in it.
func ProfileGone(err error) bool {
	return err != nil && strings.Contains(ErrorText(err), profileGoneText)
}
