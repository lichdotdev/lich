package api

// A session's chat is the same conversation as its terminal, drawn by the
// window instead of by the provider's TUI: lich runs the provider's CLI in its
// structured mode (Claude Code's stream-json, ACP for the others that speak it)
// and reports what it does as items. Switching a session between the two stops
// one process and resumes the conversation in the other, one at a time.

var (
	// ChatKinds is the providers whose sessions can be drawn as a chat.
	ChatKinds = define[None, []string]("chat.Kinds")
	// ChatSetView switches a session between its terminal and its chat. It
	// refuses while the agent is working.
	ChatSetView = define[ChatViewOptions, None]("chat.SetView")
	// ChatHistory is a chat session's conversation so far, for a window that
	// opens on it.
	ChatHistory = define[SessionIDOptions, []ChatItem]("chat.History")
	// ChatPrompt sends the person's message.
	ChatPrompt = define[ChatPromptOptions, None]("chat.Prompt")
	// ChatCancel stops the turn in flight.
	ChatCancel = define[SessionIDOptions, None]("chat.Cancel")
	// ChatAnswer answers a permission item with one of its options.
	ChatAnswer = define[ChatAnswerOptions, None]("chat.Answer")

	// ChatItemEvent carries one item of a chat, whole, each time it changes:
	// the window replaces the item with the same id, or appends a new one.
	ChatItemEvent = defineEvent[ChatItemUpdate]("chat-item")
	// ChatTurnEvent carries a chat session's turn starting and ending.
	ChatTurnEvent = defineEvent[ChatTurn]("chat-turn")
	// SessionViewEvent carries a session's view once the switch is done: the
	// new process is up, not merely asked for.
	SessionViewEvent = defineEvent[SessionView]("session-view")
)

// The views a session is drawn in.
const (
	ViewTerminal = "terminal"
	ViewChat     = "chat"
)

// ChatViewOptions asks for session ID in View.
type ChatViewOptions struct {
	ID   string `json:"id"`
	View string `json:"view"`
}

// ChatPromptOptions sends Text to session ID's agent.
type ChatPromptOptions struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// ChatAnswerOptions answers permission item ItemID of session ID with the
// option OptionID.
type ChatAnswerOptions struct {
	ID       string `json:"id"`
	ItemID   string `json:"itemId"`
	OptionID string `json:"optionId"`
}

// The kinds of chat item.
const (
	ItemUser       = "user"       // the person's message
	ItemAgent      = "agent"      // the agent's text, Markdown
	ItemThought    = "thought"    // the agent's reasoning, when the provider reports it
	ItemTool       = "tool"       // a tool call and its outcome
	ItemPermission = "permission" // the agent asking before a tool runs
	ItemNotice     = "notice"     // something lich or the provider says about the session: an error, a resume
)

// The states of a tool item.
const (
	ToolPending = "pending"
	ToolRunning = "running"
	ToolDone    = "done"
	ToolFailed  = "failed"
)

// ChatItem is one entry of a conversation. Text is the whole text so far, not
// a delta. A tool item fills Tool; a permission item fills Permission.
type ChatItem struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	// Turn counts the person's messages: every item of an answer carries the
	// turn of the message it answers.
	Turn       int             `json:"turn"`
	Text       string          `json:"text,omitempty"`
	Tool       *ChatToolCall   `json:"tool,omitempty"`
	Permission *ChatPermission `json:"permission,omitempty"`
	// At is when the item began, in unix milliseconds.
	At int64 `json:"at"`
}

// ChatToolCall is a tool the agent ran. Name is the provider's own (Read,
// Bash), Title what it acts on (a path, a command), Output a short reading
// of its result; Diff is a unified diff when the tool edits a file.
type ChatToolCall struct {
	Name   string `json:"name"`
	Title  string `json:"title,omitempty"`
	State  string `json:"state"`
	Output string `json:"output,omitempty"`
	Diff   string `json:"diff,omitempty"`
}

// ChatPermission is the agent asking to run Tool. Chosen is the option the
// person picked, "" while it waits.
type ChatPermission struct {
	Tool    ChatToolCall       `json:"tool"`
	Options []ChatPermitOption `json:"options"`
	Chosen  string             `json:"chosen,omitempty"`
}

// The kinds of permission option, ACP's own.
const (
	PermitAllowOnce   = "allow_once"
	PermitAllowAlways = "allow_always"
	PermitRejectOnce  = "reject_once"
)

// ChatPermitOption is one answer a permission item offers, in the provider's
// words.
type ChatPermitOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
}

// ChatItemUpdate is the payload of ChatItemEvent.
type ChatItemUpdate struct {
	SessionID string   `json:"sessionId"`
	Item      ChatItem `json:"item"`
}

// ChatTurn is the payload of ChatTurnEvent: Running while a turn is in flight;
// on its end, StopReason ("end_turn", "cancelled", "error") and how long it
// took.
type ChatTurn struct {
	SessionID  string `json:"sessionId"`
	Running    bool   `json:"running"`
	StopReason string `json:"stopReason,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`
}

// SessionView is the payload of SessionViewEvent.
type SessionView struct {
	ID   string `json:"id"`
	View string `json:"view"`
}
