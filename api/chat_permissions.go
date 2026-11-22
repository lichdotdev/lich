package api

// A chat session's permission rules and hooks, as its provider reports them,
// for the window's /permissions and /hooks panels: the screens Claude Code's
// and Codex's TUIs open for those commands, which their structured modes do
// not draw.

var (
	// ChatPermissions is a chat session's permission rules and the folders
	// it may work in. A provider that reports none leaves Reported false.
	ChatPermissions = define[SessionIDOptions, ChatPermissionList]("chat.Permissions")
	// ChatPermissionAct adds a rule to a settings file or removes one from
	// it; the running chat applies it before the call answers.
	ChatPermissionAct = define[ChatPermissionActOptions, ChatPermissionActResult]("chat.PermissionAct")
	// ChatHooks is a chat session's hooks. A provider that reports none
	// leaves Reported false.
	ChatHooks = define[SessionIDOptions, ChatHookList]("chat.Hooks")
	// ChatHookAct trusts a hook or turns it on or off.
	ChatHookAct = define[ChatHookActOptions, None]("chat.HookAct")
)

// What a permission rule does to a tool call it matches.
const (
	PermissionAllow = "allow"
	PermissionAsk   = "ask"
	PermissionDeny  = "deny"
)

// ChatPermissionAct actions.
const (
	PermissionAdd    = "add"
	PermissionRemove = "remove"
)

// Where a rule, a folder or a hook comes from, as lich names the providers'
// sources: a settings file for the person, the project's shared one, the
// checkout's own, the organization's, a launch flag, this session alone, or a
// plugin.
const (
	SettingsUser    = "user"
	SettingsProject = "project"
	SettingsLocal   = "local"
	SettingsManaged = "managed"
	SettingsFlag    = "flag"
	SettingsSession = "session"
	SettingsPlugin  = "plugin"
	SettingsOther   = "other"
)

// ChatPermissionList is a chat session's permission rules. Folders are those
// it may work in besides Cwd, the one it started in. Destinations are where
// a rule is added, nil when none can be, Locked set when the organization's
// settings allow only their own rules. Errors are the settings files skipped
// for being broken, whose rules are not listed.
type ChatPermissionList struct {
	Reported     bool                 `json:"reported"`
	Rules        []ChatPermissionRule `json:"rules,omitempty"`
	Folders      []ChatFolder         `json:"folders,omitempty"`
	Cwd          string               `json:"cwd,omitempty"`
	Destinations []string             `json:"destinations,omitempty"`
	Locked       bool                 `json:"locked,omitempty"`
	Errors       []string             `json:"errors,omitempty"`
}

// ChatPermissionRule is one rule. Rule is the text as the settings file
// holds it, what a removal names; Description its plain reading where the
// provider gives one. Removable is set on one a settings file holds that
// lich may take out, Ignored on one the organization's settings keep from
// applying.
type ChatPermissionRule struct {
	Behavior    string `json:"behavior"`
	Rule        string `json:"rule"`
	Source      string `json:"source"`
	Description string `json:"description,omitempty"`
	Removable   bool   `json:"removable,omitempty"`
	Ignored     bool   `json:"ignored,omitempty"`
}

// ChatFolder is one folder a session may work in and where it was granted.
type ChatFolder struct {
	Path   string `json:"path"`
	Source string `json:"source"`
}

// ChatPermissionActOptions adds Rule as a Behavior rule to the settings file
// of Source, or removes it from there.
type ChatPermissionActOptions struct {
	ID       string `json:"id"`
	Action   string `json:"action"`
	Behavior string `json:"behavior"`
	Rule     string `json:"rule"`
	Source   string `json:"source"`
}

// ChatPermissionActResult is what the provider said of a rule it saved,
// such as it matching every use of its tool.
type ChatPermissionActResult struct {
	Warnings []string `json:"warnings,omitempty"`
}

// Hook trust, as ChatHook.Trust names it: Codex runs a hook only once the
// person trusted it, and again only once they trust it after it changed.
const (
	HookTrusted   = "trusted"
	HookUntrusted = "untrusted"
	HookModified  = "modified"
)

// ChatHookAct actions.
const (
	HookTrust   = "trust"
	HookEnable  = "enable"
	HookDisable = "disable"
)

// Why no hook of a session runs, as ChatHookList.Blocked names it.
const (
	HooksOffByPolicy = "policy"
	HooksManagedOnly = "managed-only"
	HooksOff         = "off"
	HooksSafeMode    = "safe-mode"
	HooksBareMode    = "bare-mode"
)

// ChatHookList is a chat session's hooks, in the order its provider runs
// their events. Actions are the ChatHookAct actions it takes, nil when the
// list is to be read only. Blocked says why its hooks do not run, "" when
// they do; Errors are the files skipped for being broken.
type ChatHookList struct {
	Reported bool       `json:"reported"`
	Hooks    []ChatHook `json:"hooks,omitempty"`
	Actions  []string   `json:"actions,omitempty"`
	Blocked  string     `json:"blocked,omitempty"`
	Errors   []string   `json:"errors,omitempty"`
}

// ChatHook is one hook: what it runs (Command, a command, prompt or URL as
// Type says) on Event, for the tools Matcher names, "" for all. Label is
// what its row shows. Off is set on one that does not run. Key and Hash name
// it and its content to ChatHookAct, Trust and Managed say whether Codex runs
// it and whether the person may turn it off.
type ChatHook struct {
	Event     string `json:"event"`
	Summary   string `json:"summary,omitempty"`
	Matcher   string `json:"matcher,omitempty"`
	Source    string `json:"source"`
	Plugin    string `json:"plugin,omitempty"`
	Type      string `json:"type"`
	Command   string `json:"command"`
	Label     string `json:"label,omitempty"`
	Condition string `json:"condition,omitempty"`
	Timeout   int    `json:"timeout,omitempty"`
	Off       bool   `json:"off,omitempty"`
	Key       string `json:"key,omitempty"`
	Hash      string `json:"hash,omitempty"`
	Trust     string `json:"trust,omitempty"`
	Managed   bool   `json:"managed,omitempty"`
}

// ChatHookActOptions runs Action on the hook Key names; a trust trusts the
// content Hash names, the one the person saw.
type ChatHookActOptions struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Action string `json:"action"`
	Hash   string `json:"hash,omitempty"`
}
