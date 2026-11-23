package prompt

// Catalog is every prompt text in one language. Add a field here, then its
// text in en.go and in every other locale file; TestCatalogParity fails until
// each locale has it with the same format verbs.
//
// Untranslated in every locale, because agents and tools match them literally:
// the lich-notification tag and its attributes and values, tool and CLI names,
// ticket ids, and tool parameters.
type Catalog struct {
	isOne func(int) bool
	// isFew and isMany are nil in a language whose CLDR rule has no few or
	// many form: Pick reads Other for every count isOne leaves out.
	isFew  func(int) bool
	isMany func(int) bool

	// internal/relay: SpawnBriefing. These are passed as one argv entry and a
	// provider shipped as a .cmd is spawned through cmd.exe, so they must not
	// contain a double quote or angle brackets in any locale.
	BriefingIntro        string
	BriefingCards        string
	BriefingNative       string
	BriefingSessions     string
	BriefingCommandTools string
	BriefingCommandCLI   string
	// BriefingArtifacts follows every route, its own leading space included
	// where the language separates sentences with one; %s is how to open a
	// page, a tool name or a command, never translated.
	BriefingArtifacts string

	// BriefingEnvelope explains the notification envelope
	// (docs/hooks/notification.md) to every route, its own leading space
	// included where the language separates sentences with one. It names the
	// block without angle brackets and the attribute values without quotes.
	// %[1]s is how to reply and %[2]s how to collect, a tool name or a
	// command, never translated.
	BriefingEnvelope string

	// internal/relay: the text inside a notification envelope. Tags,
	// attributes and their enumerated values stay English in every locale.
	PingPongWarning     string
	PickTicketNudge     string
	CompactReminder     Plural
	CollectWithTool     string
	CollectWithCLI      string
	WaitingOnPermission string
	// PermissionToRun is what a blocked worker asks to run: %[1]s is the
	// command or path its permission prompt names, or the tool.
	PermissionToRun string
	// TaskReplyTool and TaskReplyCLI are a task's summary when it is answered
	// by its ticket, %[1]s: the tool for a session with lich's MCP tools, the
	// command for the rest.
	TaskReplyTool string
	TaskReplyCLI  string

	// MergeNoticeGitLab says MergeNotice of a GitLab merge request, which
	// GitLab numbers !N.
	MergeNotice       string
	MergeNoticeGitLab string
	// LateNotice and OnTimeNotice sum up a scheduled prompt come due: %[1]s is
	// when it was parked for, %[2]s how late it is.
	LateNotice   string
	OnTimeNotice string
	// LateMinutes, LateHours and LateDays word how late, %d the count.
	LateMinutes Plural
	LateHours   Plural
	LateDays    Plural
	// EditOverlap is an edit-overlap notice (docs/hooks/session-touched.md):
	// %[1]s is the other session's label, %[2]s the paths, %[3]s EditAgo or
	// EditJustNow, %[4]s how to reach it, a tool name or a command.
	EditOverlap string
	// EditAgo is %s, a LateMinutes count, said as time past.
	EditAgo     string
	EditJustNow string
	// UsageReset sums up the notice that carries ResumePrompt.
	UsageReset string
	// ResumePrompt is stored in the sessions table as a parked prompt, marked
	// as lich's (store SetLichSchedule); never recognise it by comparing text.
	ResumePrompt string
	// What a watch met, above its prompt. %[1]d is the pull request; %[2]s,
	// where it appears, the commit.
	WatchChecksFailed string
	WatchChecksPassed string
	WatchReview       string
	// A GitLab watch never meets a review: GitLab's carry no id to tell a new one by.
	WatchChecksFailedGitLab string
	WatchChecksPassedGitLab string
	// `lich schedule` and schedule_prompt's answers.
	ScheduledAtResult string // %[1]s is when
	ScheduledOnResult string // %[1]s is the condition, %[2]d the run limit
	ScheduleCleared   string
	// `lich check-log` and read_check_log's answers. %[1]d is the pull
	// request, %[2]s the commit, then the counts named in the text.
	CheckLogNoPR    string
	CheckLogClean   string // %[3]d passed, %[4]d pending
	CheckLogRunning string // %[3]d failed, %[4]d pending
	CheckLogFailed  string // %[3]d failed
	CheckLogCut     string // %[1]d is the lines left out
	CheckLogNoLog   string

	// internal/relay: a request the person made from the window
	// (relay.Request). The *Summary fields title its box; the rest are the
	// text the agent reads. GitLab variants say merge request and !N.
	RequestConflictsSummary  string
	RequestChecksSummary     string
	RequestChecksMoreSummary Plural // %d checks not named
	RequestThreadsSummary    Plural // %d threads
	RequestCommentsSummary   Plural // %d comments
	RequestNotesSummary      Plural // %d notes
	RequestSketchSummary     string
	RequestIssueSummary      string // %d is the issue
	// %[1]d is the pull request, %[2]s its branch, %[3]s the base, and
	// %[4]s the list of files.
	PRConflicts              string
	PRConflictsGitLab        string
	PRConflictsListed        string
	PRConflictsListedGitLab  string
	PRChecksUnnamed          Plural // %[1]d pull request, %[2]s branch, %[3]d count
	PRChecksUnnamedGitLab    Plural
	PRChecksListed           string // %[1]d pull request, %[2]s branch, %[3]s the checks
	PRChecksListedGitLab     string
	PRChecksOmitted          Plural // %d checks
	ReviewCommentsHeader     string
	ReviewThreadHead         string // %[1]d pull request, %[2]s branch, %[3]s path, %[4]s lines
	ReviewThreadHeadOutdated string
	ReviewThreadID           string // %s
	ReviewThreadAsk          Plural // no verb: the form follows the number of threads
	IssueHead                string // %[1]d number, %[2]s title, %[3]s URL
	BrowserNotesHead         Plural // %d notes
	BrowserNotesItem         string // %[1]d index, %[2]s label, %[3]s URL
	BrowserNotesNote         string // %s
	BrowserNotesSketch       string // %s path
	BrowserNotesSketchOnly   string // %[1]s URL, %[2]s path
	BrowserNotesSelectors    string

	// internal/subagent: the reason a /subagent deny gives the model. %[1]q is
	// the session opened, %[2]s its ticket; SubagentOpenedMany lists
	// SubagentOpenedSession for each.
	SubagentOpened        string
	SubagentOpenedSession string
	SubagentOpenedMany    string
	SubagentSealed        string
	SubagentWait          string

	// mark_thread_fixed's answer.
	ThreadMarked string

	// internal/drop: the line under a copied file's path.
	CopyNotice string
	// internal/artifact: the line under a copied artifact's path. %[1]s is its
	// name, %[2]q the session it came from.
	ArtifactCopyNotice string

	// internal/cli: the lich command line and its MCP server, read from
	// EnvVar. MCP tool descriptions, flag help and errors stay English.
	MCPInstructions        string
	MCPArtifacts           string // %s is the session's artifact directory
	HandOverFailed         string
	SendToSessionPrivate   string
	OutcomeUnread          string
	OutcomeUndelivered     string
	OutcomeUnanswered      string
	OutcomeStopped         string
	OutcomeExpired         string
	StillWorkingPrivate    string
	StillWorkingOpen       string
	StillWorkingPrivateCLI string
	StillWorkingOpenCLI    string
	CollectedNothing       string
	CollectedAnswer        string
	CollectedStillWorking  string
	OpenedProject          string
	OpenedWorktree         string
	OpenedConfined         string
	OpenedSession          string
	OpenedSessionLabelOnly string
	ClosedRemoved          string
	ClosedKept             string
	ClosedPlain            string
	ClosedProject          string
	// HandoffNote's first paragraph is the handoff notice's summary and the
	// rest its prompt (relay.HandoffEnvelope).
	HandoffNote string
	HandoffNone string
	HandoffSent string

	HandoffProfileSuggested string
	HandoffProfileDropped   string

	// internal/cli: a browser page as the browser_* tools answer with it. The
	// line tags (url:, title:, wait:, text:, interactive, downloading:, dialog
	// open:) and browser_dialog stay English in every locale: the tool
	// descriptions name them. PageListSame stands for an element list the
	// agent's last page answer already gave it.
	PageInteractive  string
	PageListSame     string
	PageMore         string
	PageFilled       string
	PageChecked      string
	PageDownloading  string
	PageWaitTimedOut string
	PageConsoleMore  Plural
	PageDialogWaits  string
	// PageDialogWaitsCLI is PageDialogWaits for `lich browser`, which names
	// its own verb instead of the MCP tool.
	PageDialogWaitsCLI string

	// internal/chat: what a chat's agent is told when the person answers its
	// permission question no.
	ChatPermissionDenied string
	// ChatQuestionDeclined is what it is told when the person answers none of
	// its questions: they pressed Esc or wrote a message instead, which
	// follows as its own.
	ChatQuestionDeclined string
	// ChatPlanDeclined is what it is told when the person answers its plan
	// with no word, and ChatPlanFeedback when they answer it with the change
	// they want (%s): either keeps it in plan mode.
	ChatPlanDeclined string
	ChatPlanFeedback string
	// ChatSuggestPrompt asks a throwaway fork of a Codex chat for the
	// person's likely next message: Claude Code's own prompt for its
	// prompt_suggestion (2.1.330), with Claude named as the assistant.
	ChatSuggestPrompt string
	// ChatTitlePrompt asks for a chat's title from its first message (%s).
	ChatTitlePrompt string
}
