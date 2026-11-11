package extension

// The methods and events mirror the tools lich's MCP server gives agents
// (list_sessions, list_worktrees, open_session, send_to_session) and mean what
// they mean there. A call comes from no session, as the lich command line run
// outside one does: nothing defaults to "your own" project or provider.
var (
	// ListProjects is every project open in lich's window.
	ListProjects = method[NoParams, []Project]("projects.list", PermissionProjectsRead)
	// ListSessions is every live session, as list_sessions returns them.
	ListSessions = method[NoParams, []Session]("sessions.list", PermissionSessionsRead)
	// ListWorktrees is a project's git worktrees, as list_worktrees returns them.
	ListWorktrees = method[WorktreesParams, []Worktree]("worktrees.list", PermissionProjectsRead)
	// OpenSession opens and starts a session, as open_session does.
	OpenSession = method[OpenSessionParams, OpenedSession]("sessions.open", PermissionSessionsWrite)
	// SendToSession gives a session a task, as send_to_session does.
	SendToSession = method[SendParams, SendResult]("sessions.send", PermissionSessionsWrite)
	// Subscribe starts the events named. Each needs its own permission, and
	// the call is refused whole when one is missing.
	Subscribe = method[SubscribeParams, NoParams]("events.subscribe", "")

	// SessionStatus fires when a session reports a new state.
	SessionStatus = event[SessionStatusData]("session.status", PermissionSessionEvents)
)

// NoParams is the params or result of a method that takes or returns nothing.
type NoParams struct{}

// Project is an open project: its lich id, the name its tab shows and the
// directory it is rooted in.
type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// Session is a live session. Label is the name on its card and how it is
// addressed, unique within its Project; Name is the second name it answers
// to, empty for Claude Code. Kind is the provider running in it (claude,
// codex, antigravity, opencode, omp, crush, cursor, kiro or shell). State is
// what it last reported: "busy" mid-turn, "done" free, "waiting" blocked on a
// person; empty when it reported nothing, which says nothing about whether
// it is free, since only some providers report. A busy session may carry Tool
// and Detail, the tool it runs and what it acts on, and a waiting one Reason.
// Path is the directory it runs in and ProjectPath its project's root.
// Focused marks the card the window has on screen.
type Session struct {
	Label       string `json:"label"`
	Name        string `json:"name"`
	Project     string `json:"project"`
	Kind        string `json:"kind"`
	State       string `json:"state"`
	ID          string `json:"id"`
	Path        string `json:"path"`
	ProjectPath string `json:"projectPath"`
	Tool        string `json:"tool,omitempty"`
	Detail      string `json:"detail,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Focused     bool   `json:"focused,omitempty"`
}

// WorktreesParams names the project, by name or directory path. It is
// required: a call comes from no session to take a project from.
type WorktreesParams struct {
	Project string `json:"project"`
}

// Worktree is one git worktree of a project other than its own directory:
// its branch, where it is, whether it holds uncommitted work and the labels
// of the sessions open in it.
type Worktree struct {
	Name     string   `json:"name"`
	Path     string   `json:"path"`
	Dirty    bool     `json:"dirty"`
	Sessions []string `json:"sessions"`
}

// OpenSessionParams opens a session in Project, an open project's name or a
// directory path, which is opened as a project first; it is required.
// Worktree, when set, is a branch to root the session in a git worktree of:
// one already checked out is opened, one that does not exist is created from
// the project's current branch. Provider is what runs in it, spelled as Kind
// is on a Session; empty is claude, as for any caller that is not a session.
// Prompt, when set, is the first task,
// handed over as soon as the agent is up and capped at 8 KB like a send.
type OpenSessionParams struct {
	Project  string `json:"project"`
	Worktree string `json:"worktree,omitempty"`
	Provider string `json:"provider,omitempty"`
	Prompt   string `json:"prompt,omitempty"`
}

// OpenedSession is the session opened: its id, its project, the Label and
// Name it is addressed by, its provider, the worktree Path it runs in (empty
// in the project's own directory) and whether it runs Confined in the
// sandbox. When a prompt came with it, Task is that send's outcome, or
// TaskError why it did not reach the session, which is open either way.
type OpenedSession struct {
	ID        string      `json:"id"`
	ProjectID string      `json:"projectId"`
	Project   string      `json:"project"`
	Label     string      `json:"label"`
	Name      string      `json:"name"`
	Kind      string      `json:"kind"`
	Path      string      `json:"path"`
	Confined  bool        `json:"confined"`
	Task      *SendResult `json:"task,omitempty"`
	TaskError string      `json:"taskError,omitempty"`
}

// SendParams types Prompt at the prompt of Session, its label or id as
// ListSessions returns them, or holds it until the session reaches one.
// Project narrows the search when two projects share the label. Prompt is
// capped at 8 KB: name the paths to read rather than pasting them.
// WaitSeconds is how long the call holds for a quick answer, at most 90,
// which is also what 0 means.
type SendParams struct {
	Session     string `json:"session"`
	Project     string `json:"project,omitempty"`
	Prompt      string `json:"prompt"`
	WaitSeconds int    `json:"waitSeconds,omitempty"`
}

// SendResult is a send's outcome. Status is "answered", with the session's
// Answer; "pending", still working when the wait ran out, under Ticket;
// "unread", "unanswered", "undelivered", "stopped" or "expired", with no
// answer. Target is the label of the session it went to.
type SendResult struct {
	Ticket string `json:"ticket"`
	Target string `json:"target"`
	Status string `json:"status"`
	Answer string `json:"answer"`
}

// SubscribeParams names the events to start, adding to any already started.
type SubscribeParams struct {
	Names []string `json:"names"`
}

// SessionStatusData is a session's new state: ID is the Session's ID, State
// one of "busy", "done", "waiting", "idle", "compacting" or "interrupted",
// with Tool, Detail and Reason as on a Session.
type SessionStatusData struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Tool   string `json:"tool,omitempty"`
	Detail string `json:"detail,omitempty"`
	Reason string `json:"reason,omitempty"`
}
