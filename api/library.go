package api

var (
	LibraryList   = define[LibraryListOptions, Library]("library.List")
	LibraryMoveIn = define[LibraryItemOptions, None]("library.MoveIn")
	LibrarySet    = define[LibrarySwitchOptions, None]("library.Set")
	LibrarySeen   = define[LibrarySeenOptions, None]("library.Seen")
)

// The kinds of item lich's library holds.
const (
	LibrarySkill = "skill"
	LibraryMCP   = "mcp"
	// LibraryPluginKind is a provider's own plugin, which lich lists and
	// switches but does not hold.
	LibraryPluginKind = "plugin"
)

// LibraryListOptions names the project whose scope the screens show, by its
// path; "" is the user's scope alone.
type LibraryListOptions struct {
	Project string `json:"project,omitempty"`
}

// Library is lich's own library of skills and MCP servers, what each provider
// has installed that is not in it yet, and lich's own plugin.
type Library struct {
	Skills []LibraryItem  `json:"skills"`
	MCP    []LibraryItem  `json:"mcp"`
	Found  []LibraryFound `json:"found"`
	// Project are the MCP servers the project's own files declare, which
	// lich shows and never moves (Paths relative to the project, or under ~).
	Project []LibraryFound `json:"project"`
	// Plugins are the providers' own plugins loaded in the scope, lich's
	// left out.
	Plugins   []LibraryItem     `json:"plugins"`
	Plugin    LibraryPlugin     `json:"plugin"`
	Providers []LibraryProvider `json:"providers"`
}

// LibraryItem is one skill or MCP server in the library. Switches are the
// ones stored; a provider or profile with none is off.
type LibraryItem struct {
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Origin is where it was moved in from, "" for one added in lich.
	Origin string `json:"origin"`
	// Command is an MCP server's command line, or its URL.
	Command   string          `json:"command"`
	Transport string          `json:"transport"`
	Switches  []LibrarySwitch `json:"switches"`
}

// LibrarySwitch is whether an item reaches a provider's sessions, in one of
// its profiles; Profile "" is the provider's own switch, above its profiles.
// Reason is why it cannot be pressed (a Library* reason code), "" when it can.
type LibrarySwitch struct {
	Provider string `json:"provider"`
	Profile  string `json:"profile"`
	On       bool   `json:"on"`
	Reason   string `json:"reason,omitempty"`
}

// LibraryFound is an item a provider has installed in its own config, one per
// name whichever providers have it; Providers are their ids. New is whether it
// appeared since the last visit (LibrarySeen).
type LibraryFound struct {
	Kind        string   `json:"kind"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Transport   string   `json:"transport"`
	Providers   []string `json:"providers"`
	Paths       []string `json:"paths"`
	New         bool     `json:"new"`
}

// LibraryPlugin is lich's own plugin: the release sessions run and what it
// brings.
type LibraryPlugin struct {
	Version string `json:"version"`
	Hooks   int    `json:"hooks"`
	Skills  int    `json:"skills"`
	MCP     int    `json:"mcp"`
}

// Why a provider cannot take a kind of item, for the window to say in its own
// words; "" when it can.
const (
	// LibraryNotWritten is a provider lich does not yet write that kind into.
	LibraryNotWritten = "notWritten"
	// LibrarySharedFile is a provider whose profiles read that kind from one
	// file or folder, so it cannot be split between them.
	LibrarySharedFile = "sharedFile"
	// LibraryCommitted is a switch only a file the repository commits holds,
	// which lich never writes.
	LibraryCommitted = "committed"
	// LibraryNoSwitch is a provider whose CLI cannot turn a plugin off
	// without removing it.
	LibraryNoSwitch = "noSwitch"
)

// LibraryProvider is one provider as the library's screens list it: its
// profiles, and why it cannot take a skill or an MCP server (a Library* reason
// code, "" when it can).
type LibraryProvider struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Profiles []Profile `json:"profiles"`
	NoSkills string    `json:"noSkills"`
	NoMCP    string    `json:"noMCP"`
	// OneSkills and OneMCP say why the provider's profiles share one switch
	// for that kind (a Library* reason code), "" when each profile has its own.
	OneSkills string `json:"oneSkills"`
	OneMCP    string `json:"oneMCP"`
}

// LibraryItemOptions names one found item to move into the library.
type LibraryItemOptions struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// LibrarySwitchOptions turns an item on or off for a provider, or one of its
// profiles; a plugin's, in Project's scope when it names one.
type LibrarySwitchOptions struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Profile  string `json:"profile"`
	On       bool   `json:"on"`
	Project  string `json:"project,omitempty"`
}

// LibrarySeenOptions records every found item of Kind as seen.
type LibrarySeenOptions struct {
	Kind string `json:"kind"`
}
