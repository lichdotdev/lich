package api

var (
	StoreCostTotals            = define[CostTotalsOptions, CostReport]("store.CostTotals")
	StoreLoadState             = define[None, []Project]("store.LoadState")
	StoreAddProject            = define[AddProjectOptions, None]("store.AddProject")
	StoreCloseProject          = define[ProjectIDOptions, None]("store.CloseProject")
	StoreReorderProjects       = define[ReorderProjectsOptions, None]("store.ReorderProjects")
	StoreRecentProjects        = define[SearchTermOptions, []RecentProject]("store.RecentProjects")
	StoreClosedProjectCount    = define[SearchTermOptions, int]("store.ClosedProjectCount")
	StoreAddSession            = define[AddSessionOptions, None]("store.AddSession")
	StoreAddSessionInFolder    = define[AddSessionInFolderOptions, None]("store.AddSessionInFolder")
	StoreAddSessionFrom        = define[AddSessionFromOptions, None]("store.AddSessionFrom")
	StoreSetActiveSession      = define[ActiveSessionOptions, None]("store.SetActiveSession")
	StoreCloseSession          = define[ParkSessionOptions, None]("store.CloseSession")
	StoreReopenSession         = define[ReopenSessionOptions, *Session]("store.ReopenSession")
	StoreReopenWorktreeSession = define[ReopenWorktreeSessionOptions, *Session]("store.ReopenWorktreeSession")
	StorePurgeWorktreeSessions = define[WorktreeSessionsOptions, None]("store.PurgeWorktreeSessions")
	StoreClosedSessions        = define[SearchTermOptions, ClosedHistory]("store.ClosedSessions")
	StoreExternalSessions      = define[None, []ExternalSession]("store.ExternalSessions")
	StoreAdoptExternalSession  = define[AdoptExternalSessionOptions, None]("store.AdoptExternalSession")
	StoreForgetSession         = define[SessionIDOptions, None]("store.ForgetSession")
	StoreMarkForked            = define[SessionIDOptions, None]("store.MarkForked")
	StoreReorderSessions       = define[ReorderSessionsOptions, None]("store.ReorderSessions")
	StoreRenameSession         = define[LabelSessionOptions, None]("store.RenameSession")
	StoreSetSessionColor       = define[SessionColorOptions, None]("store.SetSessionColor")
	StoreSetSessionFolder      = define[SessionFolderOptions, None]("store.SetSessionFolder")
	StoreSetSessionPinned      = define[SessionPinnedOptions, None]("store.SetSessionPinned")
	StoreSetSessionUnread      = define[SessionUnreadOptions, None]("store.SetSessionUnread")
	StoreRenameFolder          = define[RenameProjectFolderOptions, []string]("store.RenameFolder")
	StoreGetSetting            = define[GetSettingOptions, string]("store.GetSetting")
	StoreSetSetting            = define[SetSettingOptions, None]("store.SetSetting")
)

// AddProjectOptions stores a project opened at Path.
type AddProjectOptions struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// ProjectIDOptions names a project by its id.
type ProjectIDOptions struct {
	ID string `json:"id"`
}

// ReorderProjectsOptions is the full order of the open projects.
type ReorderProjectsOptions struct {
	IDs []string `json:"ids"`
}

// SearchTermOptions narrows a listing to what matches Term, "" for all of it.
type SearchTermOptions struct {
	Term string `json:"term"`
}

// AddSessionOptions stores a new session of project ProjectID. NextSeq is the
// project's label counter after it; Sandbox is the answer to the sandbox
// question, "" to follow the provider's rung.
type AddSessionOptions struct {
	ProjectID string `json:"projectId"`
	SessionID string `json:"sessionId"`
	Label     string `json:"label"`
	Kind      string `json:"kind"`
	Path      string `json:"path"`
	NextSeq   int    `json:"nextSeq"`
	Sandbox   string `json:"sandbox"`
}

// AddSessionInFolderOptions stores a new session filed under Folder.
type AddSessionInFolderOptions struct {
	AddSessionOptions
	Folder string `json:"folder"`
}

// AddSessionFromOptions stores a session another session opened: OriginID
// and OriginLabel name that session.
type AddSessionFromOptions struct {
	ProjectID   string `json:"projectId"`
	SessionID   string `json:"sessionId"`
	Label       string `json:"label"`
	Kind        string `json:"kind"`
	Path        string `json:"path"`
	NextSeq     int    `json:"nextSeq"`
	OriginID    string `json:"originId"`
	OriginLabel string `json:"originLabel"`
	Folder      string `json:"folder"`
}

// ActiveSessionOptions makes SessionID project ProjectID's active session.
type ActiveSessionOptions struct {
	ProjectID string `json:"projectId"`
	SessionID string `json:"sessionId"`
}

// ParkSessionOptions parks session SessionID; ActiveID becomes the project's
// active session, "" for none.
type ParkSessionOptions struct {
	ProjectID string `json:"projectId"`
	SessionID string `json:"sessionId"`
	ActiveID  string `json:"activeId"`
}

// ReopenSessionOptions brings parked session SessionID back as NewSessionID.
type ReopenSessionOptions struct {
	SessionID    string `json:"sessionId"`
	NewSessionID string `json:"newSessionId"`
}

// ReopenWorktreeSessionOptions brings back the parked session of the checkout
// at Path as NewSessionID.
type ReopenWorktreeSessionOptions struct {
	ProjectID    string `json:"projectId"`
	Path         string `json:"path"`
	NewSessionID string `json:"newSessionId"`
}

// WorktreeSessionsOptions names the sessions of project ProjectID rooted in
// the checkout at Path.
type WorktreeSessionsOptions struct {
	ProjectID string `json:"projectId"`
	Path      string `json:"path"`
}

// AdoptExternalSessionOptions files conversation ProviderSessionID as a
// parked session of project ProjectID under SessionID.
type AdoptExternalSessionOptions struct {
	ProjectID         string `json:"projectId"`
	SessionID         string `json:"sessionId"`
	Kind              string `json:"kind"`
	Path              string `json:"path"`
	ProviderSessionID string `json:"providerSessionId"`
	Label             string `json:"label"`
}

// ReorderSessionsOptions is the full card order of project ProjectID.
type ReorderSessionsOptions struct {
	ProjectID string   `json:"projectId"`
	IDs       []string `json:"ids"`
}

// LabelSessionOptions gives session SessionID the label the user typed.
type LabelSessionOptions struct {
	SessionID string `json:"sessionId"`
	Label     string `json:"label"`
}

// SessionColorOptions tints session SessionID's card, "" for the theme's.
type SessionColorOptions struct {
	SessionID string `json:"sessionId"`
	Color     string `json:"color"`
}

// SessionFolderOptions files session SessionID under Folder, "" for none.
type SessionFolderOptions struct {
	SessionID string `json:"sessionId"`
	Folder    string `json:"folder"`
}

// SessionPinnedOptions pins session SessionID to the head of the sidebar.
type SessionPinnedOptions struct {
	SessionID string `json:"sessionId"`
	Pinned    bool   `json:"pinned"`
}

// SessionUnreadOptions records whether session SessionID's finished turn is
// still unread.
type SessionUnreadOptions struct {
	SessionID string `json:"sessionId"`
	Unread    bool   `json:"unread"`
}

// RenameProjectFolderOptions renames project ProjectID's folder From to To.
type RenameProjectFolderOptions struct {
	ProjectID string `json:"projectId"`
	From      string `json:"from"`
	To        string `json:"to"`
}

// GetSettingOptions reads setting Key of project ProjectID, "" for a global
// one.
type GetSettingOptions struct {
	Key       string `json:"key"`
	ProjectID string `json:"projectId"`
}

// SetSettingOptions writes Value as setting Key of project ProjectID, "" for
// a global one.
type SetSettingOptions struct {
	Key       string `json:"key"`
	ProjectID string `json:"projectId"`
	Value     string `json:"value"`
}

// CostTotalsOptions is one CostTotals call. It travels as a single JSON object
// rather than as positional arguments because a package manager replaces the
// lich binary under a running backend: a filter a newer `lich cost` adds is
// dropped by an older backend, and one an older `lich cost` does not send is no
// filter, where a positional list refuses both on its count.
type CostTotalsOptions struct {
	Project  string `json:"project"`
	Provider string `json:"provider"`
	Since    int64  `json:"since"`
}

// CostReport is CostTotals' answer: a row per project, and the same numbers
// summed across every row.
//
// The money is a **lower bound** whenever Unpriced is not zero. A session lich
// never priced holds no ledger row at all — its provider files its conversation
// where lich has no reader, a model in it has no price, or the readout was off
// while it ran — and nothing here can guess what it spent. Shipping that count
// beside the sum
// is the whole difference between "this is what it cost" and "this is what lich
// could add up", and a total that omitted it in silence would be the one
// failure mode that makes the number not worth reading.
//
// Why the row is missing is not in here on purpose: that is costMiss
// (internal/terminal/usage_cost.go), which is decided by the live scan and
// never persisted, so the ledger can only say that a session has no price —
// not which of the reasons it was.
//
// Priced and Reported are how many counted sessions each rung contributed, and
// they are what lets a reader weigh the total: the sum is the one place the
// difference between the two is invisible, since both arrive as dollars.
//
// Readout is whether the cost readout is on at all. Off, no transcript is ever
// summed, so every session is unpriced and the total can only be zero — which a
// caller has to be able to say instead of printing a $0.00 that reads as free.
type CostReport struct {
	Projects []CostTotal `json:"projects"`
	Sessions int         `json:"sessions"`
	Unpriced int         `json:"unpriced"`
	Priced   int         `json:"priced"`
	Reported int         `json:"reported"`
	Source   string      `json:"source"`
	CostUSD  float64     `json:"costUsd"`
	Readout  bool        `json:"readout"`
}

// CostTotal is what one project's sessions have cost. Sessions is every session
// in the unit; Unpriced is how many of them carry no price at all, so the money
// covers Sessions-Unpriced of them.
//
// Source is whose arithmetic that money is: providers.CostSourcePriced when
// lich derived every dollar of it from token counts, providers.CostSourceReported
// when the providers that spent it handed their own figures over, and
// CostSourceMixed when the row spans both. Empty is a row with nothing to
// attribute — every session in it unpriced, or its spend earned by a provider
// CLI run by hand inside a shell session, which lich never spawned a rung for.
type CostTotal struct {
	Project  string  `json:"project"`
	Sessions int     `json:"sessions"`
	Unpriced int     `json:"unpriced"`
	Source   string  `json:"source"`
	CostUSD  float64 `json:"costUsd"`
}

// Project is a persisted project together with its restorable session state.
type Project struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Path            string    `json:"path"`
	NextSeq         int       `json:"nextSeq"`
	ActiveSessionID string    `json:"activeSessionId"`
	DefaultProvider string    `json:"defaultProvider"`
	Sessions        []Session `json:"sessions"`
}

// Session is a persisted terminal session (metadata only). Kind selects what
// the PTY runs: a provider id (see internal/providers) or "shell" (the user's
// shell). Path is the session's working directory when it lives in a git
// worktree; empty means the project's own path. ProviderSessionID is the id the
// provider CLI assigns the conversation running in the PTY, reported by that
// provider's session-start hook; empty until a hook fires (or for shell
// sessions), it is the key for features that need to reach a session's
// transcript or resume it. Pinned keeps a
// session at the head of its project's list and withholds its close affordances
// until it is unpinned. OriginSessionID and OriginLabel record the session that
// asked for this one, empty for a session nobody delegated: the id names the
// parent while it is still in the workspace, the label is what that parent was
// called at the time and is all that is left once it is closed.
type Session struct {
	ID                string `json:"id"`
	Label             string `json:"label"`
	Kind              string `json:"kind"`
	Path              string `json:"path"`
	ProviderSessionID string `json:"providerSessionId"`
	// Sandbox is whether this session runs confined: "on", "off", or empty for a
	// row nothing has spawned yet. The spawn writes its own verdict here, so the
	// window can mark a confined card without re-deriving a decision that took
	// the provider's rung, the checkout and a per-session override to reach.
	Sandbox string `json:"sandbox"`
	Pinned  bool   `json:"pinned"`
	// Folder is the sidebar block this session was filed under, "" for one the
	// user has not filed. It rides the row rather than a preference because it
	// is what the list is organised by — a workspace of forty sessions that
	// came back ungrouped after a restart would be a workspace nobody files.
	Folder string `json:"folder"`
	// Color is the palette colour the user painted the card with, "" for one
	// that follows the theme.
	Color string `json:"color"`
	// Profile is the Claude Code profile the session runs in
	// (internal/claudeprofile), "" for Personal and for every other provider.
	Profile string `json:"profile"`
	// Model is the model the session was born on (SetSessionModel), "" for one
	// opened on its provider's default.
	Model           string `json:"model"`
	OriginSessionID string `json:"originSessionId"`
	OriginLabel     string `json:"originLabel"`
	// Forked is whether the session was born branching its origin's
	// conversation, rather than delegated or opened as a subagent.
	Forked bool `json:"forked"`
	// ScheduledAt is when the prompt below is due, in unix seconds, 0 for a
	// session with nothing waiting. It rides the session row so the card that
	// draws the countdown and the relay that delivers it read the same hydration
	// call, and neither needs a channel of its own.
	ScheduledAt     int64  `json:"scheduledAt"`
	ScheduledPrompt string `json:"scheduledPrompt"`
	// Unread is whether this session's last finished turn is still waiting to be
	// read. It rides the hydration call because the mark has to survive a window
	// restart: the window holds it while it is up, and this row is what it comes
	// back to (see SetSessionUnread).
	Unread bool `json:"unread"`
	// MCPServers names the MCP servers this session's provider could reach when
	// it was spawned. It rides the row for the reason Sandbox does: the spawn is
	// what resolved it — from the provider's own config documents and this
	// session's directory — and a window restart has to come back to the same
	// answer without re-deriving it. Nil for a row nothing has spawned yet.
	MCPServers []string `json:"mcpServers"`
	// SandboxSkippedLinks names the paths under the user's home this session's
	// sandbox left out for being symlinks, relative to that home. It rides the
	// row for the reason MCPServers does — the spawn is what resolved it, and a
	// window restart has to come back to the same answer. Nil for an unconfined
	// session and for one whose sandbox skipped nothing.
	SandboxSkippedLinks []string `json:"sandboxSkippedLinks"`
}

// RecentProject is a closed project offered for reopening — identity only, since the
// menu that lists them shows a name and a path and nothing else.
type RecentProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// ClosedHistory is one page of the parked-session history and the size of the
// match it was cut from. Total is what lets the list say it was cut instead of
// presenting closedSessionLimit rows as the whole answer.
type ClosedHistory struct {
	Sessions []ParkedSession `json:"sessions"`
	Total    int             `json:"total"`
	// Parked sessions whose conversation has not been indexed yet, which is what
	// the list reports while the backfill catches up on a workspace that predates
	// the index. 0 once every row has one, which is the steady state.
	Indexing int `json:"indexing"`
}

// ParkedSession is one parked session offered for resuming — what identifies it
// in a list somebody is browsing rather than one they are already looking at.
// The project rides along because history spans every project at once, closed
// ones included, and the project name is what tells two sessions of the same
// name apart.
//
// Two answers about the branch, and they are different questions: ParkedBranch
// is the snapshot this row was closed on, which is what the search matches, and
// what the window draws is read live off the checkout, because a worktree's
// directory stops agreeing with its branch the moment an agent branches inside
// it (native/cmd/lich-native/model.go, checkoutLabel). A row whose checkout is gone has
// nothing live to show — which is the same row that cannot be resumed anyway.
type ParkedSession struct {
	ID          string `json:"id"`
	ProjectID   string `json:"projectId"`
	ProjectName string `json:"projectName"`
	// The project's own directory, so resuming a session of a closed project can
	// reopen that project first without a second lookup — and can ask where it
	// went when the directory has moved.
	ProjectPath string `json:"projectPath"`
	Label       string `json:"label"`
	Kind        string `json:"kind"`
	Path        string `json:"path"`
	// The branch the checkout was on at the close, empty for a row parked before
	// lich recorded one. Searchable, not drawn.
	ParkedBranch string `json:"parkedBranch"`
	// Unix seconds, 0 for a row parked before closed_at existed — which sorts
	// last and is drawn as no date rather than as 1970.
	ClosedAt int64 `json:"closedAt"`
	// Whether the search reached this row through its conversation. It is what
	// the window keeps the row on, apart from the snippet: a hit the index found
	// by a fold the plain text does not have ("cion" against "ción"), or a term
	// the index split on punctuation, has a snippet of nothing to show.
	MatchedConversation bool `json:"matchedConversation"`
	// The stretch of conversation that matched the search, empty for a row the
	// search matched by name, for one matched by a fold (see above), and for
	// every row of an empty query, which matched nothing in particular.
	Snippet string `json:"snippet"`
	// Whether the cap on one session's index dropped the oldest of this
	// conversation. The row says so on screen: a search that reaches only part
	// of a session has to be readable as that rather than as an absence.
	Truncated bool `json:"truncated"`
}

// ExternalSession is a conversation started outside lich that one of its
// projects can adopt: it ran in that project's directory or in one of its
// checkouts. Path is what the adopted row carries, "" for the project's own
// directory, the same as a session lich opened there.
type ExternalSession struct {
	Kind              string `json:"kind"`
	ProviderSessionID string `json:"providerSessionId"`
	Title             string `json:"title"`
	Path              string `json:"path"`
	ProjectID         string `json:"projectId"`
	ProjectName       string `json:"projectName"`
	ProjectPath       string `json:"projectPath"`
	UpdatedAt         int64  `json:"updatedAt"`
	// Profile is the Claude Code or Codex profile the conversation lives in,
	// the one a resume of it runs in whatever the project's own profile is.
	Profile string `json:"profile,omitempty"`
}
