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
	// ChatOptions is what a chat session can be set to now: its model, its
	// effort and its mode, each as the provider offers them.
	ChatOptions = define[SessionIDOptions, []ChatOption]("chat.Options")
	// ChatSetOption sets one of them. A value the provider only takes at spawn
	// is applied by resuming the conversation once the agent is idle; while a
	// turn runs it is refused.
	ChatSetOption = define[ChatSetOptionOptions, None]("chat.SetOption")

	// ChatItemEvent carries one item of a chat, whole, each time it changes:
	// the window replaces the item with the same id, or appends a new one.
	ChatItemEvent = defineEvent[ChatItemUpdate]("chat-item")
	// ChatTurnEvent carries a chat session's turn starting and ending.
	ChatTurnEvent = defineEvent[ChatTurn]("chat-turn")
	// ChatOptionsEvent carries a chat session's options whole, each time one
	// changes, whoever changed it.
	ChatOptionsEvent = defineEvent[ChatOptionsUpdate]("chat-options")
	// SessionViewEvent carries a session's view once the switch is done: the
	// new process is up, not merely asked for. A chat resumed in a new process
	// for an option sends it too, and its history is read again.
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
	// Model and Effort are what produced the item, as the person reads them:
	// the model's name and the effort level in force when the item began. Each
	// is "" where the provider does not say, as for most of a conversation read
	// back from a transcript, which holds no effort.
	Model  string `json:"model,omitempty"`
	Effort string `json:"effort,omitempty"`
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
	// Changes is what the turn changed in the session's checkout, on its end
	// alone; nil where lich could not read the checkout.
	Changes *ChatTurnChanges `json:"changes,omitempty"`
}

// ChatTurnChanges is the checkout's diff against HEAD at a turn's end less the
// one at its start: what the turn changed by any means, a tool or a shell
// command alike, and whatever else changed the checkout meanwhile.
type ChatTurnChanges struct {
	Files   int `json:"files"`
	Added   int `json:"added"`
	Removed int `json:"removed"`
}

// SessionView is the payload of SessionViewEvent.
type SessionView struct {
	ID   string `json:"id"`
	View string `json:"view"`
}

// The categories of chat option, ACP's session config option categories.
const (
	OptionModel  = "model"
	OptionEffort = "thought_level"
	OptionMode   = "mode"
)

// ChatOption is one setting of a chat session, as the provider offers it:
// Current is the ID of the choice in force.
type ChatOption struct {
	ID       string       `json:"id"`
	Category string       `json:"category"`
	Label    string       `json:"label"`
	Current  string       `json:"current"`
	Choices  []ChatChoice `json:"choices"`
}

// ChatChoice is one value a ChatOption can take, in the provider's words.
// Danger marks a mode that runs tools without asking (Claude Code's
// bypassPermissions), which the window sets apart.
type ChatChoice struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Danger      bool   `json:"danger,omitempty"`
}

// ChatSetOptionOptions sets option OptionID of session ID to the choice
// Value.
type ChatSetOptionOptions struct {
	ID       string `json:"id"`
	OptionID string `json:"optionId"`
	Value    string `json:"value"`
}

// ChatOptionsUpdate is the payload of ChatOptionsEvent.
type ChatOptionsUpdate struct {
	SessionID string       `json:"sessionId"`
	Options   []ChatOption `json:"options"`
}
