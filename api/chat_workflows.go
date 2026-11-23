package api

// A chat session's workflows: the scripts its agent runs in the background,
// each fanning out to agents of its own, which go on after the turn that
// started them has ended. They run in the chat's process and end with it.

var (
	// ChatWorkflows is a chat session's workflows, running and ended, in the
	// order they started. A provider that reports none has none.
	ChatWorkflows = define[SessionIDOptions, []ChatWorkflow]("chat.Workflows")

	// ChatWorkflowsEvent carries a chat session's workflows whole, each time
	// one of them or one of its agents moves.
	ChatWorkflowsEvent = defineEvent[ChatWorkflowsUpdate]("chat-workflows")
)

// The states of a workflow and of its agents. An agent is queued until it
// starts, and ends done or failed; a workflow also ends stopped.
const (
	WorkflowQueued    = "queued"
	WorkflowRunning   = "running"
	WorkflowCompleted = "completed"
	WorkflowFailed    = "failed"
	WorkflowStopped   = "stopped"
)

// ChatWorkflow is one workflow. Started and Ended are in unix milliseconds,
// Ended 0 while it runs.
type ChatWorkflow struct {
	ID      string              `json:"id"`
	Name    string              `json:"name"`
	State   string              `json:"state"`
	Started int64               `json:"started"`
	Ended   int64               `json:"ended,omitempty"`
	Agents  []ChatWorkflowAgent `json:"agents"`
}

// ChatWorkflowAgent is one agent of a workflow, in the order the script
// started them. Phase is the script's phase it runs in, empty outside one;
// Tool and ToolInput the last call it made; Prompt and Result the start of
// its task and of its answer, Error why it failed. Started and Ended are in
// unix milliseconds, 0 while unknown.
type ChatWorkflowAgent struct {
	Index     int    `json:"index"`
	Label     string `json:"label"`
	Phase     string `json:"phase,omitempty"`
	Model     string `json:"model,omitempty"`
	State     string `json:"state"`
	Started   int64  `json:"started,omitempty"`
	Ended     int64  `json:"ended,omitempty"`
	Tokens    int    `json:"tokens,omitempty"`
	ToolCalls int    `json:"toolCalls,omitempty"`
	Tool      string `json:"tool,omitempty"`
	ToolInput string `json:"toolInput,omitempty"`
	Prompt    string `json:"prompt,omitempty"`
	Result    string `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ChatWorkflowsUpdate is the payload of ChatWorkflowsEvent.
type ChatWorkflowsUpdate struct {
	SessionID string         `json:"sessionId"`
	Workflows []ChatWorkflow `json:"workflows"`
}
