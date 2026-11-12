package extension

// The methods and events mirror the tools lich's MCP server gives agents
// (list_sessions, list_worktrees, open_session, send_to_session, ask_session,
// wait_for_answer, the service tools, show_code, browser_open) and mean what
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
	// AskSession puts a side question to a Claude Code session and waits for
	// its answer, as ask_session does.
	AskSession = method[AskParams, AskResult]("sessions.ask", PermissionSessionsAsk)
	// CollectAnswer waits on a ticket this extension's own sessions.send or
	// sessions.open returned pending, as wait_for_answer does with a ticket.
	CollectAnswer = method[AnswerParams, SendResult]("sessions.answer", PermissionSessionsWrite)
	// ListServices is a project's services, as list_services returns them.
	ListServices = method[ServicesParams, []Service]("services.list", PermissionServicesRead)
	// StartService starts a service, as start_service does.
	StartService = method[ServiceParams, Service]("services.start", PermissionServicesWrite)
	// StopService asks a service to stop, as stop_service does.
	StopService = method[ServiceParams, Service]("services.stop", PermissionServicesWrite)
	// RestartService restarts a service, as restart_service does.
	RestartService = method[ServiceParams, Service]("services.restart", PermissionServicesWrite)
	// ServiceOutput reads a service's console, as read_service_output does.
	ServiceOutput = method[ServiceParams, ServiceOutputResult]("services.output", PermissionServicesRead)
	// GitStatus is a checkout's branch, changes and standing against its base,
	// as the window's cards read them.
	GitStatus = method[GitParams, GitStatusResult]("git.status", PermissionGitRead)
	// GitPullRequest is the open pull request of a checkout's branch, as its
	// badge shows it.
	GitPullRequest = method[GitParams, PullRequestResult]("git.pullRequest", PermissionGitRead)
	// Notify raises a desktop notification through lich's own.
	Notify = method[NotifyParams, NoParams]("notify", PermissionNotify)
	// ShowCode opens a file of a session's checkout in the window's dock, as
	// show_code does.
	ShowCode = method[ShowCodeParams, ShownCode]("window.showCode", PermissionWindow)
	// OpenURL loads a page in a session's browser tab, as browser_open does.
	OpenURL = method[OpenURLParams, OpenedURL]("window.openURL", PermissionWindow)
	// Subscribe starts the events named. Each needs its own permission, and
	// the call is refused whole when one is missing.
	Subscribe = method[SubscribeParams, NoParams]("events.subscribe", "")

	// SessionStatus fires when a session reports a new state.
	SessionStatus = event[SessionStatusData]("session.status", PermissionSessionEvents)
	// ServiceStatus fires when a service starts, stops or fails.
	ServiceStatus = event[ServiceStatusData]("service.status", PermissionServicesRead)
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

// AskParams asks Session, its label, name or id, Question. Project narrows
// the search when two projects share the label. Only a Claude Code session
// whose lich-plugin mod is loaded answers; every other is refused.
type AskParams struct {
	Session  string `json:"session"`
	Project  string `json:"project,omitempty"`
	Question string `json:"question"`
}

// AskResult is the session's answer: its id, project and label, and what it
// said. A session that does not answer within 90 seconds is an error, and an
// answer that comes later is dropped: an ask has no ticket.
type AskResult struct {
	ID      string `json:"id"`
	Project string `json:"project"`
	Label   string `json:"label"`
	Answer  string `json:"answer"`
}

// AnswerParams waits on Ticket for WaitSeconds, at most 90, which is also
// what 0 means. The ticket must be one this extension's own sessions.send or
// sessions.open returned pending.
type AnswerParams struct {
	Ticket      string `json:"ticket"`
	WaitSeconds int    `json:"waitSeconds,omitempty"`
}

// ServicesParams names the project, by name or directory path. It is required.
type ServicesParams struct {
	Project string `json:"project"`
}

// ServiceParams names a service of Project by its name, matched ignoring case.
type ServiceParams struct {
	Project string `json:"project"`
	Service string `json:"service"`
}

// Service is a project service: State is "running", "stopping", "stopped"
// (exited 0 or asked to stop) or "failed" (exited on its own with ExitCode).
// StartedAt and StoppedAt are unix milliseconds, 0 for never.
type Service struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Command       string `json:"command"`
	StartWithLich bool   `json:"startWithLich"`
	State         string `json:"state"`
	ExitCode      *int   `json:"exitCode,omitempty"`
	StartedAt     int64  `json:"startedAt"`
	StoppedAt     int64  `json:"stoppedAt"`
}

// ServiceOutputResult is the service's console as it is now, terminal escapes
// included: what a running one printed so far, or what a stopped one printed
// in its last run. It is one read: call again for what came since.
type ServiceOutputResult struct {
	Output string `json:"output"`
}

// GitParams names a checkout by its directory path.
type GitParams struct {
	Path string `json:"path"`
}

// GitStatusResult is what a card shows of a checkout: its Branch and Head
// commit, its uncommitted Files, Added and Deleted lines; the commits it is
// Ahead and Behind its Upstream ("origin/feat" form, empty when it tracks
// none), counted from the refs on disk with no fetch, as `git status` does;
// and against Base (the branch it merges into, "origin/main" form, empty when
// it has none) the commits it is BehindBase and the files that would conflict.
type GitStatusResult struct {
	Branch     string   `json:"branch"`
	Head       string   `json:"head"`
	Files      int      `json:"files"`
	Added      int      `json:"added"`
	Deleted    int      `json:"deleted"`
	Upstream   string   `json:"upstream"`
	Ahead      int      `json:"ahead"`
	Behind     int      `json:"behind"`
	Base       string   `json:"base"`
	BehindBase int      `json:"behindBase"`
	Conflicts  []string `json:"conflicts"`
}

// PullRequestResult carries the open pull request of the checkout's branch,
// nil when it has none open.
type PullRequestResult struct {
	PullRequest *PullRequest `json:"pullRequest"`
}

// PullRequest is a GitHub pull request as gh reports it.
type PullRequest struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
	Title  string `json:"title"`
	Head   string `json:"head"`
	Base   string `json:"base"`
}

// NotifyParams is a desktop notification: Title, then Body under it, each cut
// at 120 characters.
type NotifyParams struct {
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
}

// ShowCodeParams opens Path, relative to the checkout of Session (its id as
// ListSessions returns it) or absolute inside it, at Line through EndLine,
// 1-based and inclusive, 0 leaving them out. Comment is plain text shown
// under the lines, at most 360 characters, and needs Line.
type ShowCodeParams struct {
	Session string `json:"session"`
	Path    string `json:"path"`
	Line    int    `json:"line,omitempty"`
	EndLine int    `json:"endLine,omitempty"`
	Comment string `json:"comment,omitempty"`
}

// ShownCode is what the dock shows: Path relative to the session's checkout.
type ShownCode struct {
	Session string `json:"session"`
	Path    string `json:"path"`
	Line    int    `json:"line,omitempty"`
	EndLine int    `json:"endLine,omitempty"`
}

// OpenURLParams loads URL, http or https, in the browser of Session, its id
// as ListSessions returns it, which the person sees in a tab beside it.
type OpenURLParams struct {
	Session string `json:"session"`
	URL     string `json:"url"`
}

// OpenedURL is the page the browser landed on.
type OpenedURL struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

// ServiceStatusData is a service's new State, as on a Service, with the id
// and name of the service and the project it belongs to.
type ServiceStatusData struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ProjectID string `json:"projectId"`
	State     string `json:"state"`
	ExitCode  *int   `json:"exitCode,omitempty"`
}
