package api

var (
	RelayPeers         = define[PeersOptions, []Peer]("relay.Peers")
	RelaySelf          = define[SelfOptions, Peer]("relay.Self")
	RelaySend          = define[SendOptions, SendResult]("relay.Send")
	RelaySendPrivate   = define[SendOptions, SendResult]("relay.SendPrivate")
	RelaySendSubagent  = define[SendOptions, SendResult]("relay.SendSubagent")
	RelayWait          = define[WaitOptions, SendResult]("relay.Wait")
	RelayCollect       = define[CollectOptions, CollectedResults]("relay.Collect")
	RelayCollectNow    = define[CollectNowOptions, CollectedResults]("relay.CollectNow")
	RelayReply         = define[ReplyOptions, None]("relay.Reply")
	RelayInsert        = define[InsertTextOptions, InsertedText]("relay.Insert")
	RelayFocus         = define[FocusSessionOptions, FocusedSession]("relay.Focus")
	RelayFocused       = define[None, FocusedSession]("relay.Focused")
	RelaySetShown      = define[ShownSessionOptions, None]("relay.SetShown")
	RelayAnnounceMerge = define[AnnounceMergeOptions, None]("relay.AnnounceMerge")
	RelaySubmit        = define[SubmitOptions, None]("relay.Submit")
	RelayRequest       = define[RequestOptions, None]("relay.Request")
)

// The topics of a request (RequestOptions.Topic).
const (
	RequestConflicts      = "conflicts"
	RequestChecks         = "checks"
	RequestReviewThread   = "review-thread"
	RequestReviewComments = "review-comments"
	RequestBrowserNotes   = "browser-notes"
	RequestIssue          = "issue"
)

// RequestOptions is an action the person took in the window, which lich hands
// the session as a request envelope it words in the prompt language
// (docs/hooks/notification.md): the window says what was asked, the backend
// writes it. The fields a topic does not use stay empty.
type RequestOptions struct {
	SessionID string `json:"sessionId"`
	Topic     string `json:"topic"`
	// Forge is ForgeGitHub or ForgeGitLab, the words a pull request topic is
	// written in; "" reads as GitHub.
	Forge  string `json:"forge,omitempty"`
	PR     int    `json:"pr,omitempty"`
	Branch string `json:"branch,omitempty"`
	Base   string `json:"base,omitempty"`
	// URL is the page browser notes were left on or the issue's.
	URL string `json:"url,omitempty"`
	// Image is the sketch drawn on the page, a path the session can read.
	Image string `json:"image,omitempty"`
	Issue int    `json:"issue,omitempty"`
	// Title and Body are the issue's.
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
	// Failed is how many checks failed, counted when no item names one.
	Failed int           `json:"failed,omitempty"`
	Items  []RequestItem `json:"items,omitempty"`
	// Note is what the person wrote to close the request, empty for none.
	Note string `json:"note,omitempty"`
}

// RequestItem is one row of a request: a conflicting file, a failing check, a
// review thread or comment, a note on a page.
type RequestItem struct {
	Path     string `json:"path,omitempty"`
	Lines    string `json:"lines,omitempty"`
	ID       string `json:"id,omitempty"`
	Outdated bool   `json:"outdated,omitempty"`
	Check    string `json:"check,omitempty"`
	Label    string `json:"label,omitempty"`
	// URL is a check's run, a thread's discussion or the page a note is on.
	URL      string `json:"url,omitempty"`
	Selector string `json:"selector,omitempty"`
	// Component, Source and HTML are what a browser note's element is.
	Component string `json:"component,omitempty"`
	Source    string `json:"source,omitempty"`
	HTML      string `json:"html,omitempty"`
	// Text is a review comment or a browser note.
	Text string `json:"text,omitempty"`
	// Comments are a review thread's, oldest first.
	Comments []RequestComment `json:"comments,omitempty"`
}

// RequestComment is one comment of a review thread.
type RequestComment struct {
	Author string `json:"author"`
	Body   string `json:"body"`
}

// AnnounceMergeOptions is a pull request merged through lich: the checkout its
// branch was in and what the notice tells the agents working there.
type AnnounceMergeOptions struct {
	Checkout string `json:"checkout"`
	Number   int    `json:"number"`
	Title    string `json:"title"`
	Branch   string `json:"branch"`
	Base     string `json:"base"`
	// Forge is ForgeGitHub or ForgeGitLab, the words the notice names it in;
	// "" reads as GitHub.
	Forge string `json:"forge,omitempty"`
}

// SubmitOptions is a message typed into a session and submitted with Enter.
type SubmitOptions struct {
	SessionID string `json:"sessionId"`
	Message   string `json:"message"`
}

// CollectOptions is one Collect call, an object for the reason SendOptions is
// one.
type CollectOptions struct {
	From        string `json:"from"`
	WaitSeconds int    `json:"waitSeconds"`
}

// CollectedResults is what a drain returns: every outcome that was waiting for the
// caller, oldest first, and the labels of the sessions still owing one.
type CollectedResults struct {
	Results []SendResult `json:"results"`
	Open    []string     `json:"open"`
}

// CollectNowOptions is one CollectNow call, an object for the reason
// SendOptions is one.
type CollectNowOptions struct {
	From string `json:"from"`
}

// FocusSessionOptions is one Focus call, an object for the reason SendOptions is one.
type FocusSessionOptions struct {
	From    string `json:"from"`
	Target  string `json:"target"`
	Project string `json:"project"`
}

// FocusedSession says which session's card was brought up, so a caller that named it
// loosely can tell the person which one it was.
type FocusedSession struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Project string `json:"project"`
}

// ShownSessionOptions is the native window saying which session's card it has
// on screen, "" for none.
type ShownSessionOptions struct {
	SessionID string `json:"sessionId"`
}

// InsertTextOptions is one Insert call, an object for the reason SendOptions is
// one.
type InsertTextOptions struct {
	From        string `json:"from"`
	Target      string `json:"target"`
	Project     string `json:"project"`
	Text        string `json:"text"`
	WaitSeconds int    `json:"waitSeconds"`
}

// InsertedText says where the text went, so an editor can tell the person which
// session took it when the target was chosen for them.
type InsertedText struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Project string `json:"project"`
	Kind    string `json:"kind"`
	// Bytes is what was pasted, after control characters were stripped (sanitize).
	Bytes int `json:"bytes"`
}

// PeersOptions is one Peers call, an object for the reason SendOptions is one.
type PeersOptions struct {
	From string `json:"from"`
}

// Peer is one session a caller may address: the label it is addressed by, the
// name it answers to in Claude Code's peer roster, the project it belongs to
// (labels are unique within a project, not across them), what is running in it,
// and what that session last reported it was doing.
//
// Both names are published because both reach this session and an agent sees
// them in different places — the label on the card, the roster name in
// `/list-agents` and in what a mention writes at a prompt. Showing one and
// accepting the other is what made an agent treat a single session as two.
//
// State is stateBusy, stateWaiting or stateDone, and empty when the session has
// reported nothing — which is a provider whose plugin does not report state as
// much as a session that has not had a turn yet. Empty is "not known", never
// "idle": guessing the second is how a caller ends up sending work into a
// session that cannot take it.
type Peer struct {
	Label   string `json:"label"`
	Name    string `json:"name"`
	Project string `json:"project"`
	Kind    string `json:"kind"`
	State   string `json:"state"`
	// ID is the session's lich id, the LICH_SESSION_ID its process carries,
	// so a reader holding a recorded id can find its card. Last, because
	// fields are appended to this shape, never inserted.
	ID string `json:"id"`
	// Path is the directory the session runs in, the worktree's for a worktree
	// session, and ProjectPath the root of the project it belongs to. They are
	// what an editor matches its open folder against to find the session beside
	// it; the name of a project says nothing about where it is.
	Path        string `json:"path"`
	ProjectPath string `json:"projectPath"`
	// Tool and Detail are the tool a busy session is running and what it acts
	// on, Reason what a waiting one is blocked on: the line its card draws
	// under the label, in the provider's own words (SessionStatus). Empty in
	// every other state, and for a busy turn that has named no tool yet.
	Tool   string `json:"tool,omitempty"`
	Detail string `json:"detail,omitempty"`
	Reason string `json:"reason,omitempty"`
	// Focused marks the session whose card the window has on screen: the
	// active card of the project tab in front. No session has it while no
	// window is open.
	Focused bool `json:"focused,omitempty"`
	// Subagent is how many subagent levels below a session nobody opened as
	// one this session runs (store.SessionSubagentDepth), 0 for such a
	// session. lich mcp reads its own to leave a card's tool list without the
	// workspace administration tools (internal/cli, cardTools).
	Subagent int `json:"subagent,omitempty"`
}

// ReplyOptions is one Reply call, an object for the reason SendOptions is one.
// From is the session replying.
type ReplyOptions struct {
	From   string `json:"from"`
	Ticket string `json:"ticket"`
	Answer string `json:"answer"`
}

// SelfOptions is one Self call, an object for the reason SendOptions is one.
type SelfOptions struct {
	From string `json:"from"`
}

// SendOptions is one Send, SendPrivate or SendSubagent call. It travels as a
// single JSON object rather than as positional arguments because a package
// manager replaces the lich binary under a running backend: a field a newer
// lich adds is dropped by an older backend, and one an older lich does not send
// is its zero value, where a positional list refuses both on its count.
type SendOptions struct {
	From        string `json:"from"`
	Target      string `json:"target"`
	Project     string `json:"project"`
	Prompt      string `json:"prompt"`
	WaitSeconds int    `json:"waitSeconds"`
	// ExtensionID and ExtensionName say the call is an extension's, by its id
	// and its name in the prompt language, so the prompt the target receives is
	// introduced as relayed by it. A call from a session leaves them empty.
	ExtensionID   string `json:"extensionId,omitempty"`
	ExtensionName string `json:"extensionName,omitempty"`
}

// SendResult is what a caller gets back from Send or Wait. Answer is empty unless
// Status is StatusAnswered.
type SendResult struct {
	Ticket string `json:"ticket"`
	Target string `json:"target"`
	Status string `json:"status"`
	Answer string `json:"answer"`
	// Private is set on the outcome of a ticket sent with SendPrivate, which
	// only that ticket collects: no note will announce it.
	Private bool `json:"private,omitempty"`
	// TargetContext is the tokens the target's context held when the task was
	// handed to it, set only past the size where a new session would do the
	// same task for less (relay.largeContextTokens).
	TargetContext int `json:"targetContext,omitempty"`
}

// WaitOptions is one Wait call, an object for the reason SendOptions is one.
type WaitOptions struct {
	Ticket      string `json:"ticket"`
	WaitSeconds int    `json:"waitSeconds"`
}
