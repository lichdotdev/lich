package api

// A chat session's plugins and the marketplaces they come from, as its
// provider reports them, for the window's /plugin panel: the screen Claude
// Code's TUI opens for that command, which its structured mode does not draw.

var (
	// ChatPlugins is a chat session's plugins, installed and on offer, and its
	// marketplaces. A provider that reports none leaves Reported false.
	ChatPlugins = define[SessionIDOptions, ChatPluginList]("chat.Plugins")
	// ChatPluginAct acts on one plugin or marketplace: installs, uninstalls,
	// turns on or off or updates a plugin, adds, updates or removes a
	// marketplace.
	ChatPluginAct = define[ChatPluginActOptions, ChatPluginActResult]("chat.PluginAct")
)

// ChatPluginAct actions. The Plugin ones name a plugin by its ID, the
// Marketplace ones a marketplace by its name, MarketplaceAdd by its source.
const (
	PluginInstall     = "install"
	PluginUninstall   = "uninstall"
	PluginEnable      = "enable"
	PluginDisable     = "disable"
	PluginUpdate      = "update"
	MarketplaceAdd    = "marketplace-add"
	MarketplaceUpdate = "marketplace-update"
	MarketplaceRemove = "marketplace-remove"
)

// Where a plugin is installed, as ChatPlugin.Scope and ChatPluginList.Scopes
// name it: for the person, for everyone in the project, or in this checkout
// only.
const (
	PluginScopeUser    = "user"
	PluginScopeProject = "project"
	PluginScopeLocal   = "local"
)

// ChatPluginList is a chat session's plugins: the installed ones that apply
// in its checkout and, not installed, those its marketplaces offer. Actions
// are the ChatPluginAct actions its provider takes, Scopes where it installs
// one, the default first.
type ChatPluginList struct {
	Reported     bool              `json:"reported"`
	Plugins      []ChatPlugin      `json:"plugins,omitempty"`
	Marketplaces []ChatMarketplace `json:"marketplaces,omitempty"`
	Actions      []string          `json:"actions,omitempty"`
	Scopes       []string          `json:"scopes,omitempty"`
}

// ChatPlugin is one plugin. ID is name@marketplace. Latest is a newer version
// than Version, "" when there is none or the provider cannot tell. Installs
// is how many people installed it, 0 when not reported. Parts is what an
// installed one brings, nil when not reported. Fixed is set on lich's own
// plugin, which lich installs and pins itself.
type ChatPlugin struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Marketplace string           `json:"marketplace"`
	Description string           `json:"description,omitempty"`
	Version     string           `json:"version,omitempty"`
	Latest      string           `json:"latest,omitempty"`
	Installed   bool             `json:"installed,omitempty"`
	Enabled     bool             `json:"enabled,omitempty"`
	Scope       string           `json:"scope,omitempty"`
	Installs    int              `json:"installs,omitempty"`
	Parts       *ChatPluginParts `json:"parts,omitempty"`
	Fixed       bool             `json:"fixed,omitempty"`
}

// ChatPluginParts is what a plugin brings, and Tokens what it adds to every
// session's context, 0 when not reported.
type ChatPluginParts struct {
	Skills int `json:"skills"`
	Agents int `json:"agents"`
	Hooks  int `json:"hooks"`
	MCP    int `json:"mcp"`
	Tokens int `json:"tokens,omitempty"`
}

// ChatMarketplace is one marketplace the provider was given. Source is the
// repository, URL or path it reads, Plugins how many it offers.
type ChatMarketplace struct {
	Name    string `json:"name"`
	Source  string `json:"source,omitempty"`
	Plugins int    `json:"plugins"`
	Fixed   bool   `json:"fixed,omitempty"`
}

// ChatPluginActOptions runs Action of session ID on Target: a plugin's ID, a
// marketplace's name, or for MarketplaceAdd its source. Scope is where a
// plugin is installed, or is to be, "" for the provider's default.
type ChatPluginActOptions struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	Target string `json:"target"`
	Scope  string `json:"scope,omitempty"`
}

// ChatPluginActResult is what an update left: Restart when it installed a
// newer version the chat runs only once restarted, Current when it was already
// the latest.
type ChatPluginActResult struct {
	Restart bool `json:"restart,omitempty"`
	Current bool `json:"current,omitempty"`
}
