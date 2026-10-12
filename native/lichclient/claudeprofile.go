package lichclient

import (
	"context"
	"strings"
)

// ClaudeProfileLoginEvent is the event a profile login reports its progress on
// (claudeprofile.LoginEventName).
const ClaudeProfileLoginEvent = "claude-profile-login"

// ClaudeProfileEvent is the event a session moving to another profile emits
// (terminal profileEventName).
const ClaudeProfileEvent = "session-profile"

// profileGoneText is internal/claudeprofile.ErrProfileGone, which a spawn in a
// profile deleted since answers.
const profileGoneText = "claude profile no longer exists"

// ClaudeProfile mirrors claudeprofile.Info: a Claude Code login lich keeps a
// config directory for, with the account `claude auth status` found there.
// Slug is "personal" for the default profile.
type ClaudeProfile struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Personal bool   `json:"personal"`
	// Copied names the shared entries the profile holds as copies rather than
	// links (Windows without Developer Mode).
	Copied      []string `json:"copied,omitempty"`
	LoggedIn    bool     `json:"loggedIn"`
	Email       string   `json:"email,omitempty"`
	OrgName     string   `json:"orgName,omitempty"`
	Plan        string   `json:"plan,omitempty"`
	Plugins     int      `json:"plugins"`
	StatusError string   `json:"statusError,omitempty"`
}

// ClaudeProfilePlugin mirrors claudeprofile.Plugin.
type ClaudeProfilePlugin struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Marketplace string `json:"marketplace"`
}

// ClaudeProfilePluginResult mirrors claudeprofile.PluginResult: Message is
// Claude Code's own words when it refused one.
type ClaudeProfilePluginResult struct {
	Key     string `json:"key"`
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

// ClaudeProfileLogin mirrors claudeprofile.LoginEvent: Output (and URL, when
// that text holds the sign-in address) while the CLI talks, then one event
// with Done set.
type ClaudeProfileLogin struct {
	Profile     string `json:"profile"`
	Output      string `json:"output,omitempty"`
	URL         string `json:"url,omitempty"`
	Done        bool   `json:"done,omitempty"`
	LoggedIn    bool   `json:"loggedIn,omitempty"`
	Cancelled   bool   `json:"cancelled,omitempty"`
	Error       string `json:"error,omitempty"`
	PluginError string `json:"pluginError,omitempty"`
}

// ClaudeProfileMoved is the session-profile payload: Profile is the slug the
// session now runs in, "" for Personal.
type ClaudeProfileMoved struct {
	ID      string `json:"id"`
	Profile string `json:"profile"`
}

// ClaudeProfiles lists every profile with its account; the backend runs one
// `claude auth status` per profile to answer.
func (c *Client) ClaudeProfiles(ctx context.Context) ([]ClaudeProfile, error) {
	var list []ClaudeProfile
	return list, c.Call(ctx, "claudeprofile.Profiles", &list)
}

// CreateClaudeProfile makes a profile named name and returns it; logging it in
// is Login's.
func (c *Client) CreateClaudeProfile(ctx context.Context, name string) (ClaudeProfile, error) {
	var p ClaudeProfile
	return p, c.Call(ctx, "claudeprofile.CreateProfile", &p, name)
}

// RenameClaudeProfile changes a profile's display name.
func (c *Client) RenameClaudeProfile(ctx context.Context, slug, name string) error {
	return c.Call(ctx, "claudeprofile.RenameProfile", nil, slug, name)
}

// DeleteClaudeProfile removes a profile and its login; the backend refuses
// while a session runs in it.
func (c *Client) DeleteClaudeProfile(ctx context.Context, slug string) error {
	return c.Call(ctx, "claudeprofile.DeleteProfile", nil, slug)
}

// ClaudeProfilePlugins lists the plugins a profile has at user scope, the
// ones a port can carry to another profile.
func (c *Client) ClaudeProfilePlugins(ctx context.Context, slug string) ([]ClaudeProfilePlugin, error) {
	var list []ClaudeProfilePlugin
	return list, c.Call(ctx, "claudeprofile.Plugins", &list, slug)
}

// InstallClaudeProfilePlugins installs keys (plugin@marketplace) into a
// logged-in profile, one result per key.
func (c *Client) InstallClaudeProfilePlugins(ctx context.Context, slug string, keys []string) ([]ClaudeProfilePluginResult, error) {
	var results []ClaudeProfilePluginResult
	return results, c.Call(ctx, "claudeprofile.InstallPlugins", &results, slug, keys)
}

// LoginClaudeProfile starts `claude auth login` as a profile and returns once
// the CLI runs; its progress arrives as ClaudeProfileLoginEvent events.
func (c *Client) LoginClaudeProfile(ctx context.Context, slug string) error {
	return c.Call(ctx, "claudeprofile.Login", nil, slug)
}

// ClaudeLoginCode hands a running login the code the sign-in page showed.
func (c *Client) ClaudeLoginCode(ctx context.Context, slug, code string) error {
	return c.Call(ctx, "claudeprofile.LoginCode", nil, slug, code)
}

// CancelClaudeLogin ends a login lich started.
func (c *Client) CancelClaudeLogin(ctx context.Context, slug string) error {
	return c.Call(ctx, "claudeprofile.CancelLogin", nil, slug)
}

// ClaudeLoginCommand is the command line that logs a profile in from a
// terminal.
func (c *Client) ClaudeLoginCommand(ctx context.Context, slug string) (string, error) {
	var command string
	return command, c.Call(ctx, "claudeprofile.LoginCommand", &command, slug)
}

// ClaudeProfilePlan reads one profile's plan; "personal" is the machine's own
// reading.
func (c *Client) ClaudeProfilePlan(ctx context.Context, slug string) (QuotaPlan, error) {
	var plan QuotaPlan
	return plan, c.Call(ctx, "quota.ProfilePlan", &plan, slug)
}

// ContinueIn moves a Claude Code session's conversation to another profile
// and carries on there. The backend refuses a turn in progress.
func (c *Client) ContinueIn(ctx context.Context, id, slug string) error {
	return c.Call(ctx, "terminal.ContinueIn", nil, id, slug)
}

// ProfileGone reports whether err is a spawn refused for a profile deleted
// since the session was born in it.
func ProfileGone(err error) bool {
	return err != nil && strings.Contains(ErrorText(err), profileGoneText)
}
