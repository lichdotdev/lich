package api

// ProfileMethods are the calls every provider's profile service answers.
type ProfileMethods struct {
	Profiles      Method[None, []ProfileInfo]
	CreateProfile Method[CreateProfileOptions, Profile]
	UpdateProfile Method[UpdateProfileOptions, Profile]
	DeleteProfile Method[ProfileSlugOptions, None]
	LoginCommand  Method[ProfileSlugOptions, string]
}

// ProfileLoginMethods run a provider's login from lich, where its CLI has one
// that is not a terminal card's: Claude Code's and Codex's.
type ProfileLoginMethods struct {
	Login       Method[ProfileSlugOptions, None]
	CancelLogin Method[ProfileSlugOptions, None]
}

// ProfilePluginMethods carry plugins between profiles: Claude Code's and
// Codex's.
type ProfilePluginMethods struct {
	Plugins        Method[ProfileSlugOptions, []ProfilePlugin]
	InstallPlugins Method[InstallPluginsOptions, []ProfilePluginResult]
}

var (
	ClaudeProfiles         = profileMethods("claudeprofile")
	ClaudeProfileLogin     = profileLoginMethods("claudeprofile")
	ClaudeProfilePlugins   = profilePluginMethods("claudeprofile")
	ClaudeProfileLoginCode = define[LoginCodeOptions, None]("claudeprofile.LoginCode")
	CodexProfiles          = profileMethods("codexprofile")
	CodexProfileLogin      = profileLoginMethods("codexprofile")
	CodexProfilePlugins    = profilePluginMethods("codexprofile")
	OMPProfiles            = profileMethods("ompprofile")
	// XDGProfiles are the profile services of the providers whose logins a
	// variable moves (internal/xdgprofile), by provider id.
	XDGProfiles = map[string]ProfileMethods{
		"opencode": profileMethods("opencodeprofile"),
		"cursor":   profileMethods("cursorprofile"),
		"kiro":     profileMethods("kiroprofile"),
	}
)

func profileMethods(service string) ProfileMethods {
	return ProfileMethods{
		Profiles:      define[None, []ProfileInfo](service + ".Profiles"),
		CreateProfile: define[CreateProfileOptions, Profile](service + ".CreateProfile"),
		UpdateProfile: define[UpdateProfileOptions, Profile](service + ".UpdateProfile"),
		DeleteProfile: define[ProfileSlugOptions, None](service + ".DeleteProfile"),
		LoginCommand:  define[ProfileSlugOptions, string](service + ".LoginCommand"),
	}
}

func profileLoginMethods(service string) ProfileLoginMethods {
	return ProfileLoginMethods{
		Login:       define[ProfileSlugOptions, None](service + ".Login"),
		CancelLogin: define[ProfileSlugOptions, None](service + ".CancelLogin"),
	}
}

func profilePluginMethods(service string) ProfilePluginMethods {
	return ProfilePluginMethods{
		Plugins:        define[ProfileSlugOptions, []ProfilePlugin](service + ".Plugins"),
		InstallPlugins: define[InstallPluginsOptions, []ProfilePluginResult](service + ".InstallPlugins"),
	}
}

// CreateProfileOptions names a new profile.
type CreateProfileOptions struct {
	Name string `json:"name"`
}

// UpdateProfileOptions changes profile Slug as Update says.
type UpdateProfileOptions struct {
	Slug   string        `json:"slug"`
	Update ProfileUpdate `json:"update"`
}

// ProfileSlugOptions names a profile by its slug.
type ProfileSlugOptions struct {
	Slug string `json:"slug"`
}

// InstallPluginsOptions installs Keys (plugin@marketplace) into profile Slug.
type InstallPluginsOptions struct {
	Slug string   `json:"slug"`
	Keys []string `json:"keys"`
}

// LoginCodeOptions hands a running login of profile Slug the code its
// sign-in page showed.
type LoginCodeOptions struct {
	Slug string `json:"slug"`
	Code string `json:"code"`
}

// Profile is one login lich knows about.
type Profile struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Dir      string `json:"dir"`
	Personal bool   `json:"personal"`
	// Copied names the shared entries (a settings file, say) this profile holds
	// as a copy, so edits made in one account no longer reach the other.
	Copied []string `json:"copied,omitempty"`
	// Color is the card color the profile is shown in, "" for the one the
	// window derives from the slug.
	Color string `json:"color,omitempty"`
	// Image is the profile's picture as a PNG, avatarEdge square.
	Image []byte `json:"image,omitempty"`
}

// ProfileUpdate is a change to how a profile is shown. A field left nil keeps what the
// profile has; Name keeps it when empty.
type ProfileUpdate struct {
	Name string `json:"name,omitempty"`
	// Color is a card color's name; "" goes back to the color the window
	// derives from the slug. The window draws a name it does not know the same
	// way, so it is stored as given.
	Color *string `json:"color,omitempty"`
	// Image is a file to make the profile's image from; "" removes the image.
	Image *string `json:"image,omitempty"`
}

// ProfileInfo is a profile with its login state, as the profile services of
// every provider list it; a field a provider has no reading for stays empty.
type ProfileInfo struct {
	Profile
	LoggedIn bool   `json:"loggedIn"`
	Email    string `json:"email,omitempty"`
	OrgName  string `json:"orgName,omitempty"`
	// Plan is the subscription the login is on: Claude Code's own name ("pro",
	// "max", "team", …), or Codex's from its plan reading.
	Plan string `json:"plan,omitempty"`
	// AuthMethod is how a Claude Code profile authenticates: "claude.ai",
	// "console", or "none" when nobody is logged in.
	AuthMethod string `json:"authMethod,omitempty"`
	// Plugins counts a Claude Code or Codex profile's plugins.
	Plugins int `json:"plugins"`
	// Providers are the providers an oh-my-pi profile holds an OAuth login for.
	Providers []string `json:"providers,omitempty"`
	// StatusError is why the CLI's status gave no answer, in its own words;
	// LoggedIn is then false for want of one.
	StatusError string `json:"statusError,omitempty"`
}

// ProfilePlugin is one plugin a profile has installed at user scope.
type ProfilePlugin struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Marketplace string `json:"marketplace"`
	Version     string `json:"version,omitempty"`
}

// ProfilePluginResult is one plugin's install, with Claude Code's own words when it
// refused: an organization's policy, a marketplace the profile cannot reach.
type ProfilePluginResult struct {
	Key     string `json:"key"`
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}
