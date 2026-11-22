package api

// A session's chat is the same conversation as its terminal, drawn by the
// window instead of by the provider's TUI: lich runs the provider's CLI in its
// structured mode (Claude Code's stream-json, Codex's app-server, ACP for the
// others that speak it)
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
	// ChatSteer sends the person's message into the turn in flight, which
	// carries on with it; with no turn in flight it is ChatPrompt. A provider
	// that takes no message during a turn stops it and sends the message
	// after.
	ChatSteer = define[ChatPromptOptions, None]("chat.Steer")
	// ChatCancel stops the turn in flight.
	ChatCancel = define[SessionIDOptions, None]("chat.Cancel")
	// ChatAnswer answers a permission item with one of its options.
	ChatAnswer = define[ChatAnswerOptions, None]("chat.Answer")
	// ChatReply answers a question item, or declines it.
	ChatReply = define[ChatReplyOptions, None]("chat.Reply")
	// ChatOptions is what a chat session can be set to now: its model, its
	// effort and its mode, each as the provider offers them.
	ChatOptions = define[SessionIDOptions, []ChatOption]("chat.Options")
	// ChatSetOption sets one of them. A value the provider only takes at spawn
	// is applied by resuming the conversation once the agent is idle; while a
	// turn runs it is refused.
	ChatSetOption = define[ChatSetOptionOptions, None]("chat.SetOption")
	// ChatCommands is what a chat session's provider takes typed at its
	// prompt, its slash commands and skills, as it lists them; none until it
	// has.
	ChatCommands = define[SessionIDOptions, []ChatCommand]("chat.Commands")
	// ChatRunShell runs a command the person typed after "!" the way the
	// provider's own TUI does, with no model and no permission check; a
	// provider whose protocol has no such way refuses it.
	ChatRunShell = define[ChatRunShellOptions, None]("chat.RunShell")
	// ChatNew starts an empty conversation in a chat session, the one it had
	// left to the provider. It refuses while the agent works.
	ChatNew = define[SessionIDOptions, None]("chat.New")

	// ChatItemEvent carries one item of a chat, whole, each time it changes:
	// the window replaces the item with the same id, or appends a new one.
	ChatItemEvent = defineEvent[ChatItemUpdate]("chat-item")
	// ChatTurnEvent carries a chat session's turn starting and ending.
	ChatTurnEvent = defineEvent[ChatTurn]("chat-turn")
	// ChatOptionsEvent carries a chat session's options whole, each time one
	// changes, whoever changed it.
	ChatOptionsEvent = defineEvent[ChatOptionsUpdate]("chat-options")
	// ChatCommandsEvent carries a chat session's commands whole, each time
	// the provider lists them anew.
	ChatCommandsEvent = defineEvent[ChatCommandsUpdate]("chat-commands")
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

// ChatPromptOptions sends Text to session ID's agent, with Attachments: an
// image goes as an image where the provider takes one, any other file as its
// path under the text with its notice, as a drop at the terminal pastes it.
type ChatPromptOptions struct {
	ID          string           `json:"id"`
	Text        string           `json:"text"`
	Attachments []DropAttachment `json:"attachments,omitempty"`
}

// ChatRunShellOptions runs Command in session ID's checkout.
type ChatRunShellOptions struct {
	ID      string `json:"id"`
	Command string `json:"command"`
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
	ItemQuestion   = "question"   // the agent asking the person to choose or write
	ItemNotice     = "notice"     // something lich or the provider says about the session: an error, a resume
	ItemPlan       = "plan"       // the agent's task list as it stands, whole, drawn over the composer and not in the conversation
)

// The states of a plan entry, Claude Code's and ACP's own.
const (
	PlanPending    = "pending"
	PlanInProgress = "in_progress"
	PlanCompleted  = "completed"
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
	Turn int    `json:"turn"`
	Text string `json:"text,omitempty"`
	// Files are the paths a person's message attached, in this run of the
	// chat: a conversation read back has its text alone.
	Files      []string        `json:"files,omitempty"`
	Tool       *ChatToolCall   `json:"tool,omitempty"`
	Permission *ChatPermission `json:"permission,omitempty"`
	Question   *ChatQuestion   `json:"question,omitempty"`
	Plan       *ChatPlan       `json:"plan,omitempty"`
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
// of its result; Diff is a unified diff when the tool edits a file. Ended is
// when it finished, in unix milliseconds, 0 while it runs and where nobody
// reported its end: a conversation read back from most providers.
type ChatToolCall struct {
	Name   string `json:"name"`
	Title  string `json:"title,omitempty"`
	State  string `json:"state"`
	Output string `json:"output,omitempty"`
	Diff   string `json:"diff,omitempty"`
	Ended  int64  `json:"ended,omitempty"`
}

// ChatPlan is the agent's task list, in its order. A turn's plan item holds
// the list as it last stood in that turn; one with no entries is a list the
// agent cleared.
type ChatPlan struct {
	Entries []ChatPlanEntry `json:"entries"`
}

// ChatPlanEntry is one task. ActiveForm is what the task reads as while it
// runs ("Running the tests"), its Content where the provider gives none.
type ChatPlanEntry struct {
	Content    string `json:"content"`
	ActiveForm string `json:"activeForm,omitempty"`
	Status     string `json:"status"`
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

// ChatCommand is one command a chat's provider takes typed at its prompt.
// Text is what the composer gets, as the provider's TUI takes it: "/compact",
// or "$deploy" for a Codex skill. Hint names its arguments, and Skill sets a
// skill apart from a command where the provider tells the two apart.
type ChatCommand struct {
	Text        string `json:"text"`
	Description string `json:"description,omitempty"`
	Hint        string `json:"hint,omitempty"`
	Skill       bool   `json:"skill,omitempty"`
}

// ChatCommandsUpdate is the payload of ChatCommandsEvent.
type ChatCommandsUpdate struct {
	SessionID string        `json:"sessionId"`
	Commands  []ChatCommand `json:"commands"`
}

// ChatOptionsUpdate is the payload of ChatOptionsEvent.
type ChatOptionsUpdate struct {
	SessionID string       `json:"sessionId"`
	Options   []ChatOption `json:"options"`
}

// ChatQuestion is the agent asking the person, Claude Code's AskUserQuestion
// shape that every provider's request for input is read into: one to four
// questions. Reply is the answer once given, nil while it waits.
//
// Proposal is set where the agent asks to start on the plan it wrote: the one
// ask's options are then the modes the work can start in, each Label a choice
// id of the session's mode option, which the window names; the person's own
// words, or no answer, keep it planning.
type ChatQuestion struct {
	Questions []ChatAsk     `json:"questions"`
	Reply     *ChatQReplied `json:"reply,omitempty"`
	Proposal  *ChatProposal `json:"proposal,omitempty"`
}

// ChatProposal is the plan the agent asks to start on, in Markdown, "" where
// it gave none.
type ChatProposal struct {
	Plan string `json:"plan"`
}

// ChatAsk is one question. Header is its short label; an ask with no Options
// takes free text alone. Multi lets the person pick more than one option.
// Every ask also takes the person's own words.
type ChatAsk struct {
	Header  string      `json:"header,omitempty"`
	Text    string      `json:"text"`
	Options []ChatOffer `json:"options,omitempty"`
	Multi   bool        `json:"multi,omitempty"`
}

// ChatOffer is one option of a question; Preview is a block of text shown
// beside the options while this one is highlighted.
type ChatOffer struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Preview     string `json:"preview,omitempty"`
}

// ChatQReplied is how a question item was answered: one answer per ask, in
// order, or Declined when the person answered none.
type ChatQReplied struct {
	Answers  []ChatAskAnswer `json:"answers,omitempty"`
	Declined bool            `json:"declined,omitempty"`
}

// ChatAskAnswer is the options picked, by label, and the person's own words,
// "" for none.
type ChatAskAnswer struct {
	Picked []string `json:"picked,omitempty"`
	Other  string   `json:"other,omitempty"`
}

// ChatReplyOptions answers question item ItemID of session ID.
type ChatReplyOptions struct {
	ID     string       `json:"id"`
	ItemID string       `json:"itemId"`
	Reply  ChatQReplied `json:"reply"`
}
