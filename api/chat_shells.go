package api

// A chat session's background shells: the commands its agent started to run
// on while it carries on, which Claude Code's TUI counts in its footer and
// lists in its /tasks dialog. They run in the chat's process and end with it.

var (
	// ChatShells is a chat session's background shells, running and ended,
	// in the order they started. A provider that reports none has none.
	ChatShells = define[SessionIDOptions, []ChatShell]("chat.Shells")
	// ChatShellOutput is the end of one shell's output so far, plain text.
	ChatShellOutput = define[ChatShellOptions, string]("chat.ShellOutput")
	// ChatStopShell stops one shell that runs.
	ChatStopShell = define[ChatShellOptions, None]("chat.StopShell")

	// ChatShellsEvent carries a chat session's shells whole, each time one
	// starts or ends.
	ChatShellsEvent = defineEvent[ChatShellsUpdate]("chat-shells")
)

// The states of a background shell.
const (
	ShellRunning   = "running"
	ShellCompleted = "completed"
	ShellFailed    = "failed"
	ShellStopped   = "stopped"
)

// ChatShell is one background shell. Command is what it runs, Description
// the agent's words for it; Started and Ended are in unix milliseconds,
// Ended 0 while it runs. Monitor marks a command the agent watches, each line
// it prints reaching the agent, which Claude Code counts apart from shells.
type ChatShell struct {
	ID          string `json:"id"`
	Command     string `json:"command"`
	Description string `json:"description,omitempty"`
	Monitor     bool   `json:"monitor,omitempty"`
	State       string `json:"state"`
	Started     int64  `json:"started"`
	Ended       int64  `json:"ended,omitempty"`
}

// ChatShellOptions names shell Shell of session ID.
type ChatShellOptions struct {
	ID    string `json:"id"`
	Shell string `json:"shell"`
}

// ChatShellsUpdate is the payload of ChatShellsEvent.
type ChatShellsUpdate struct {
	SessionID string      `json:"sessionId"`
	Shells    []ChatShell `json:"shells"`
}
