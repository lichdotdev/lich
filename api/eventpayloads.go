package api

// SessionStatus is the payload of SessionStatusEvent: the session whose processing
// state changed, the new state, and — on a pre-tool report alone — the tool it
// is about to run and what that tool acts on, or — on a waiting one alone — what
// it is blocked on. All three are the provider's own words, never translated
// here (docs/hooks/session-state.md tables them).
type SessionStatus struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Tool   string `json:"tool,omitempty"`
	Detail string `json:"detail,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// SessionTitle is the payload of SessionTitleEvent: the session whose label changed
// and its new label.
type SessionTitle struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// SessionTouched is the payload of SessionTouchedEvent: the session whose files
// likely changed.
type SessionTouched struct {
	ID string `json:"id"`
}

// SessionAgent is the payload of SessionAgentEvent: the session and the provider
// CLI now live in its PTY ("" when none is).
type SessionAgent struct {
	ID    string `json:"id"`
	Agent string `json:"agent"`
}

type SessionConversation struct {
	ID                string `json:"id"`
	ProviderSessionID string `json:"providerSessionId"`
}

// SessionSandbox is the payload of SessionSandboxEvent: the session, whether its PTY
// is confined, and the home paths that sandbox skipped for being symlinks,
// relative to the home (nil for an unconfined spawn and for one that skipped
// none).
type SessionSandbox struct {
	ID           string   `json:"id"`
	Confined     bool     `json:"confined"`
	SkippedLinks []string `json:"skippedLinks"`
}

// SessionMCP is the payload of SessionMCPEvent: the session and the MCP servers its
// provider could reach when it was spawned.
type SessionMCP struct {
	ID      string   `json:"id"`
	Servers []string `json:"servers"`
}

// SessionDuplicate is the payload of SessionDuplicateEvent.
type SessionDuplicate struct {
	ID     string `json:"id"`
	HeldBy string `json:"heldBy"`
}

// SessionCwd is the payload of SessionCwdEvent.
type SessionCwd struct {
	ID   string `json:"id"`
	Cwd  string `json:"cwd"`
	Host string `json:"host"`
}

// SessionLimit is the payload of SessionLimitEvent.
type SessionLimit struct {
	ID       string `json:"id"`
	Window   string `json:"window"`
	ResetsAt int64  `json:"resetsAt"`
}

type SessionProfile struct {
	ID string `json:"id"`
	// Profile is the slug, "" for Personal.
	Profile string `json:"profile"`
}

// SessionTodo is the payload of SessionTodoEvent. Both counts are the whole list the
// agent last wrote, so a card can decide for itself whether a list is worth
// drawing (a finished one is not, and neither is a list of one).
type SessionTodo struct {
	ID    string `json:"id"`
	Done  int    `json:"done"`
	Total int    `json:"total"`
}

type PluginIncompatible struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

// SessionExit is the payload of TerminalExitEvents: the status the session's PTY
// child exited with. Code is absent when the child left none — killed by a
// signal, or a wait that failed — so the window can say the session ended
// without reporting a clean exit nobody observed.
type SessionExit struct {
	Code *int `json:"code,omitempty"`
}

// SessionFocus is the payload of SessionFocusEvent.
type SessionFocus struct {
	ID string `json:"id"`
}

// SessionInbox is the payload of SessionInboxEvent.
type SessionInbox struct {
	ID    string `json:"id"`
	Count int    `json:"count"`
}

// SessionSchedule is the payload of SessionScheduleEvent: the session whose mark
// changed, when its prompt is now due, and the prompt. At is 0 and Prompt empty
// when nothing is waiting anymore.
type SessionSchedule struct {
	ID     string `json:"id"`
	At     int64  `json:"at"`
	Prompt string `json:"prompt,omitempty"`
}

// SessionRelay is the payload of SessionRelayEvent: the session whose mark changed,
// the label at the other end, which way the request runs, and the ticket the
// two ends share. Peer is empty when the other end is not a session at all —
// the CLI run from a script or a shell — which the card words its own way.
//
// The ticket rides along because it is otherwise written down in exactly one
// place: the message typed at the target's prompt. An agent whose context was
// compacted past that message has no way back to it, and neither had the person
// watching — the window knew a request was open and could not say which one.
// Empty when the mark is being cleared, along with the direction.
type SessionRelay struct {
	ID        string `json:"id"`
	Peer      string `json:"peer"`
	Direction string `json:"direction"`
	Ticket    string `json:"ticket"`
}

// RelayStalled is the payload of RelayStalledEvent: who asked ("" when the
// request came from the command line rather than a session), and the session
// that has whatever was produced.
type RelayStalled struct {
	ID       string `json:"id"`
	TargetID string `json:"targetId"`
	Target   string `json:"target"`
}

// SessionClosed is the payload of SessionClosedEvent: which session left, the project
// it left, and which of that project's sessions is active now — the window has
// no say in the choice, because the row that records it is already written.
type SessionClosed struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	ActiveID  string `json:"activeId"`
}

// SessionsFiled is SessionsFiledEvent's payload. Folder is "" when the cards were taken
// out of the one they were in.
type SessionsFiled struct {
	ProjectID string   `json:"projectId"`
	IDs       []string `json:"ids"`
	Folder    string   `json:"folder"`
}

// SessionsColored is SessionsColoredEvent's payload. Color is "" when the cards went
// back to the theme.
type SessionsColored struct {
	ProjectID string   `json:"projectId"`
	IDs       []string `json:"ids"`
	Color     string   `json:"color"`
}

// ProjectAdded is the payload of ProjectAddedEvent: the identity as stored.
type ProjectAdded struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// ProjectClosed is the payload of ProjectClosedEvent.
type ProjectClosed struct {
	ID string `json:"id"`
}

// ProjectsReordered is the payload of ProjectsReorderedEvent: every open
// project, in the order that was stored.
type ProjectsReordered struct {
	IDs []string `json:"ids"`
}

// ProfileLogin is one step of a login. Output carries what the CLI printed, with
// URL set when that text holds the sign-in address. Authenticating says the CLI
// is through with the browser and lich is reading whether the login took. The
// last event has Done set:
// LoggedIn is what the CLI's login status answered afterwards (`claude auth
// status`, `codex login status`), Error why the login failed or was cancelled,
// and PluginError why the lich plugin did not install.
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

// ProfileChanged is ProfileChangedEvent's payload: Provider is the provider id
// ("claude", "kiro"), Profile the profile as it now is, without its login state.
type ProfileChanged struct {
	Provider string  `json:"provider"`
	Profile  Profile `json:"profile"`
}

// PluginProfiles is what one pass did: the release it installed and the
// profiles where the install failed, in the CLI's own words. Name is the
// provider's display name.
type PluginProfiles struct {
	Provider string                 `json:"provider"`
	Name     string                 `json:"name"`
	Version  string                 `json:"version"`
	Failures []PluginProfileFailure `json:"failures"`
}

// PluginProfileFailure is one profile the plugin could not be installed into.
type PluginProfileFailure struct {
	Profile string `json:"profile"`
	Name    string `json:"name"`
	Error   string `json:"error"`
}

// AppUpdateProgress is one step of Apply: bytes received of total during the download
// (total is -1 when the asset came without a Content-Length), then the phase
// that has no percentage: "installer", where lich hands over to the Windows
// installer and closes.
type AppUpdateProgress struct {
	Phase    string `json:"phase"`
	Received int64  `json:"received"`
	Total    int64  `json:"total"`
}

// ScheduleForfeited is a scheduled prompt lost with the session it was parked
// on, which was deleted for good: the row was the only copy of what the user
// wrote.
type ScheduleForfeited struct {
	Label  string `json:"label"`
	At     int64  `json:"at"`
	Prompt string `json:"prompt"`
}
