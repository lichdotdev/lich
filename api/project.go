package api

import "encoding/json"

var (
	ProjectOpen                     = define[None, *OpenedProject]("project.Open")
	ProjectHome                     = define[None, *OpenedProject]("project.Home")
	ProjectRelocate                 = define[ProjectIDOptions, *OpenedProject]("project.Relocate")
	ProjectExists                   = define[PathOptions, bool]("project.Exists")
	ProjectMissing                  = define[PathsOptions, []string]("project.Missing")
	ProjectPickFile                 = define[PickFileOptions, string]("project.PickFile")
	ProjectPickSaveFile             = define[PickSaveFileOptions, string]("project.PickSaveFile")
	ProjectDiff                     = define[PathOptions, DiffStats]("project.Diff")
	ProjectDiffText                 = define[PathOptions, string]("project.DiffText")
	ProjectBranchesOf               = define[PathsOptions, map[string]string]("project.BranchesOf")
	ProjectBaseStatus               = define[PathOptions, *BaseStatus]("project.BaseStatus")
	ProjectBaseFreshness            = define[PathBranchOptions, BaseFreshness]("project.BaseFreshness")
	ProjectBranchRules              = define[PathBranchOptions, *BranchRules]("project.BranchRules")
	ProjectCommitIdentity           = define[PathOptions, CommitIdentity]("project.CommitIdentity")
	ProjectTree                     = define[PathOptions, FileListing]("project.Tree")
	ProjectSearch                   = define[SearchOptions, SearchResult]("project.Search")
	ProjectReadFile                 = define[CheckoutFileOptions, string]("project.ReadFile")
	ProjectReadFileAt               = define[ReadFileAtOptions, string]("project.ReadFileAt")
	ProjectFileLines                = define[FileLinesOptions, []string]("project.FileLines")
	ProjectDiscardFile              = define[CheckoutFileOptions, None]("project.DiscardFile")
	ProjectRevertLines              = define[RevertLinesOptions, RevertResult]("project.RevertLines")
	ProjectRestoreLines             = define[RestoreLinesOptions, None]("project.RestoreLines")
	ProjectCarryUncommitted         = define[CarryUncommittedOptions, None]("project.CarryUncommitted")
	ProjectListBranches             = define[PathOptions, Branches]("project.ListBranches")
	ProjectListCheckouts            = define[PathOptions, []Worktree]("project.ListCheckouts")
	ProjectCreateWorktree           = define[CreateWorktreeOptions, *Worktree]("project.CreateWorktree")
	ProjectCreateWorktreeFromPR     = define[CreateWorktreeFromPROptions, *Worktree]("project.CreateWorktreeFromPR")
	ProjectRemoveWorktree           = define[RemoveWorktreeOptions, None]("project.RemoveWorktree")
	ProjectWorktreeDirty            = define[PathOptions, bool]("project.WorktreeDirty")
	ProjectWorktreeAdopted          = define[PathOptions, bool]("project.WorktreeAdopted")
	ProjectGitHubAccounts           = define[None, []string]("project.GitHubAccounts")
	ProjectIssue                    = define[NumberOptions, Issue]("project.Issue")
	ProjectRecheckPullRequest       = define[PathOptions, None]("project.RecheckPullRequest")
	ProjectListPullRequests         = define[ListPullRequestsOptions, []PRSummary]("project.ListPullRequests")
	ProjectPullRequestDetail        = define[NumberOptions, *PRDetail]("project.PullRequestDetail")
	ProjectPullRequestConversation  = define[NumberOptions, *PRConversation]("project.PullRequestConversation")
	ProjectPullRequestAvatar        = define[AvatarOptions, []byte]("project.PullRequestAvatar")
	ProjectPullRequestDiff          = define[NumberOptions, string]("project.PullRequestDiff")
	ProjectPullRequestMergeBase     = define[NumberOptions, string]("project.PullRequestMergeBase")
	ProjectPullRequestConflicts     = define[PRConflictsOptions, []string]("project.PullRequestConflicts")
	ProjectCreatePullRequest        = define[PathOptions, None]("project.CreatePullRequest")
	ProjectMergePullRequest         = define[MergePullRequestOptions, None]("project.MergePullRequest")
	ProjectEditPullRequestTitle     = define[PRTitleOptions, None]("project.EditPullRequestTitle")
	ProjectEditPullRequestBody      = define[PRBodyOptions, None]("project.EditPullRequestBody")
	ProjectCommentOnPullRequest     = define[PRBodyOptions, None]("project.CommentOnPullRequest")
	ProjectEditPullRequestComment   = define[PRCommentBodyOptions, None]("project.EditPullRequestComment")
	ProjectDeletePullRequestComment = define[PRCommentOptions, None]("project.DeletePullRequestComment")
	ProjectSubmitReview             = define[SubmitReviewOptions, None]("project.SubmitReview")
	ProjectEditReviewBody           = define[PRReviewBodyOptions, None]("project.EditReviewBody")
	ProjectEditReviewComment        = define[PRCommentBodyOptions, None]("project.EditReviewComment")
	ProjectDeleteReviewComment      = define[PRCommentOptions, None]("project.DeleteReviewComment")
	ProjectReplyToReviewThread      = define[PRCommentBodyOptions, None]("project.ReplyToReviewThread")
	ProjectResolveReviewThread      = define[ResolveThreadOptions, None]("project.ResolveReviewThread")
)

// PathOptions names a checkout, or a directory, by its path.
type PathOptions struct {
	Path string `json:"path"`
}

// PathsOptions names several paths.
type PathsOptions struct {
	Paths []string `json:"paths"`
}

// PickFileOptions titles the file chooser.
type PickFileOptions struct {
	Title string `json:"title"`
}

// PickSaveFileOptions titles the save dialog and names the file it offers.
type PickSaveFileOptions struct {
	Title       string `json:"title"`
	DefaultName string `json:"defaultName"`
}

// PathBranchOptions names a branch of the checkout at Path.
type PathBranchOptions struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
}

// SearchOptions searches the checkout at Path for Query.
type SearchOptions struct {
	Path  string `json:"path"`
	Query string `json:"query"`
}

// CheckoutFileOptions names file Rel of the checkout at Path.
type CheckoutFileOptions struct {
	Path string `json:"path"`
	Rel  string `json:"rel"`
}

// ReadFileAtOptions names file Rel of the checkout at Path as it was at Ref.
type ReadFileAtOptions struct {
	Path string `json:"path"`
	Rel  string `json:"rel"`
	Ref  string `json:"ref"`
}

// FileLinesOptions reads lines From to To of file Rel, at Ref ("" for the
// working tree).
type FileLinesOptions struct {
	Path string `json:"path"`
	Rel  string `json:"rel"`
	Ref  string `json:"ref"`
	From int    `json:"from"`
	To   int    `json:"to"`
}

// RevertLinesOptions puts Lines of file Rel back to HEAD.
type RevertLinesOptions struct {
	Path  string       `json:"path"`
	Rel   string       `json:"rel"`
	Lines []RevertLine `json:"lines"`
}

// RestoreLinesOptions undoes the revert Undo describes.
type RestoreLinesOptions struct {
	Path string       `json:"path"`
	Rel  string       `json:"rel"`
	Undo RevertResult `json:"undo"`
}

// CarryUncommittedOptions carries the uncommitted changes of checkout Src to
// checkout Dst.
type CarryUncommittedOptions struct {
	Src string `json:"src"`
	Dst string `json:"dst"`
}

// CreateWorktreeOptions creates worktree Name of the project at ProjectPath,
// branched from Base (a remote branch when BaseIsRemote).
type CreateWorktreeOptions struct {
	ProjectPath  string `json:"projectPath"`
	ProjectID    string `json:"projectId"`
	Name         string `json:"name"`
	Base         string `json:"base"`
	BaseIsRemote bool   `json:"baseIsRemote"`
}

// CreateWorktreeFromPROptions creates a worktree on pull request Number's
// branch.
type CreateWorktreeFromPROptions struct {
	ProjectPath string `json:"projectPath"`
	ProjectID   string `json:"projectId"`
	Number      int    `json:"number"`
}

// RemoveWorktreeOptions removes the worktree at WorktreePath. AdoptedAck is
// the user's answer to removing one lich did not create.
type RemoveWorktreeOptions struct {
	ProjectPath  string `json:"projectPath"`
	WorktreePath string `json:"worktreePath"`
	Force        bool   `json:"force"`
	AdoptedAck   bool   `json:"adoptedAck"`
}

// NumberOptions names issue or pull request Number of the checkout at Path's
// repository.
type NumberOptions struct {
	Path   string `json:"path"`
	Number int    `json:"number"`
}

// ListPullRequestsOptions lists the pull requests in State ("open", "closed",
// "merged", "all").
type ListPullRequestsOptions struct {
	Path  string `json:"path"`
	State string `json:"state"`
}

// PRConflictsOptions asks which files of pull request Number conflict with
// Base.
type PRConflictsOptions struct {
	Path   string `json:"path"`
	Number int    `json:"number"`
	Base   string `json:"base"`
	PRURL  string `json:"prUrl"`
}

// MergePullRequestOptions merges pull request Number by Method ("merge",
// "squash", "rebase"); Admin bypasses the branch's rules.
type MergePullRequestOptions struct {
	Path    string `json:"path"`
	Number  int    `json:"number"`
	Method  string `json:"method"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
	Admin   bool   `json:"admin"`
}

// PRTitleOptions retitles pull request Number.
type PRTitleOptions struct {
	Path   string `json:"path"`
	Number int    `json:"number"`
	Title  string `json:"title"`
}

// PRBodyOptions is a text for pull request Number: its description, or a
// comment on it.
type PRBodyOptions struct {
	Path   string `json:"path"`
	Number int    `json:"number"`
	Body   string `json:"body"`
}

// AvatarOptions names the picture at URL, an author's Avatar in a conversation
// read from the checkout at Path.
type AvatarOptions struct {
	Path string `json:"path"`
	URL  string `json:"url"`
}

// PRCommentOptions names comment CommentID of pull request Number.
type PRCommentOptions struct {
	Path      string `json:"path"`
	Number    int    `json:"number"`
	CommentID int64  `json:"commentId"`
}

// PRCommentBodyOptions is a text for comment CommentID of pull request Number:
// its new body, or a reply to it.
type PRCommentBodyOptions struct {
	Path      string `json:"path"`
	Number    int    `json:"number"`
	CommentID int64  `json:"commentId"`
	Body      string `json:"body"`
}

// PRReviewBodyOptions rewrites the body of review ReviewID.
type PRReviewBodyOptions struct {
	Path     string `json:"path"`
	Number   int    `json:"number"`
	ReviewID int64  `json:"reviewId"`
	Body     string `json:"body"`
}

// SubmitReviewOptions submits a review of pull request Number: Event is the
// verdict, Comments the line comments, and Head the commit they were written
// against.
type SubmitReviewOptions struct {
	Path     string          `json:"path"`
	Number   int             `json:"number"`
	Event    string          `json:"event"`
	Body     string          `json:"body"`
	Head     string          `json:"head"`
	Comments []ReviewComment `json:"comments"`
}

// ResolveThreadOptions resolves review thread ThreadID, or opens it again.
type ResolveThreadOptions struct {
	Path     string `json:"path"`
	ThreadID string `json:"threadId"`
	Resolved bool   `json:"resolved"`
}

// BaseStatus is where a checkout stands against the branch it will merge into:
// how far that branch has moved underneath it, and which files a merge would
// collide on right now. It is what turns "the merge button refused" into
// something a parallel worktree knows before anyone clicks it.
type BaseStatus struct {
	Base      string   `json:"base"`   // the branch merged into, "origin/main" form
	Behind    int      `json:"behind"` // commits the base has that this checkout does not
	Conflicts []string `json:"conflicts"`
}

// Issue is the little of a GitHub issue the New worktree dialog needs: what to
// name the branch after, and what to tell the session it opens.
type Issue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	URL    string `json:"url"`
}

// BranchRules is what a repository's rulesets say about merging into one branch. GitHub's
// pull request fields do not carry it: `mergeStateStatus` answers whether this
// merge can happen, never which methods the branch would accept, so a Merge
// menu built without this offers buttons the ruleset refuses — and gh's
// refusal arrives after the click, as one flat sentence (gherror.go).
//
// The endpoint answers for a branch, not a pull request, which is why this is
// its own read: every pull request onto the same base shares one answer.
type BranchRules struct {
	// AllowedMergeMethods is squash|merge|rebase, in GitHub's own spelling.
	// Empty means the branch has no rule about it and all three stand — which
	// is also what a repository with no rulesets at all reports.
	AllowedMergeMethods []string `json:"allowedMergeMethods"`
	// CommitPatterns are the rules that read a commit's own text. Empty is the
	// common case and means only "no rule of this kind", never "nothing blocks".
	CommitPatterns []CommitPattern `json:"commitPatterns"`
	// ViewerCanBypass is whether this account administers the repository, and so
	// may merge past the rules above. It is the one answer here that must fail
	// *closed*: an unreadable one hides an action rather than offering one that
	// cannot work, because a bypass offered to an account that has none reads as
	// a broken button, not as a permission it never had.
	ViewerCanBypass bool `json:"viewerCanBypass"`
}

// CommitPattern is one ruleset rule that tests the text of a commit rather than
// the state of a pull request. It earns its own field because it is the one
// class of rule nothing on a pull request hints at: the checks pass, the review
// lands, no conflict exists, and GitHub still answers BLOCKED — because the
// message the merge *would* write breaks a pattern nobody can see from here.
type CommitPattern struct {
	// Target is the part the rule reads, already in the words the screen uses:
	// "message", "author email", "committer email".
	Target string `json:"target"`
	// Operator is GitHub's own: regex | starts_with | ends_with | contains.
	Operator string `json:"operator"`
	Pattern  string `json:"pattern"`
	// Negate flips the test — the pattern is then what the text must *not* be.
	Negate bool `json:"negate"`
}

// ThreadTally is how many of a pull request's review threads are still open.
type ThreadTally struct {
	Open  int `json:"open"`
	Total int `json:"total"`
}

// BaseFreshness is where a local branch stands against its upstream, read
// right after fetching it: what the new-worktree dialog needs to start a
// worktree from the newer of the two. Upstream is "" for a branch that tracks
// nothing on a remote, and every other field is then zero.
type BaseFreshness struct {
	Upstream string `json:"upstream"` // "origin/main" form
	Ahead    int    `json:"ahead"`
	Behind   int    `json:"behind"`
	// FetchError is why the fetch failed, "" when it did not. The counts are
	// then against whatever an earlier fetch left.
	FetchError string `json:"fetchError,omitempty"`
}

// FileListing is one listing of a directory's files: the paths, plus the two
// things the Files panel has to say out loud about how they were gathered. A
// tree that silently stops short, or silently leaves a directory out, reads as
// a folder with fewer files in it than it has.
type FileListing struct {
	Files []string `json:"files"`
	// Cut reports a listing that stopped at walkLimit. Only a plain-folder walk
	// can set it; git's own listing is never capped.
	Cut bool `json:"cut"`
	// Hidden names the walkIgnore directories this folder actually had, sorted
	// and deduped: the panel names them, so somebody keeping source under a
	// build/ or vendor/ reads why it is missing instead of guessing.
	Hidden []string `json:"hidden"`
}

// OpenedProject identifies an opened project directory.
type OpenedProject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// PullRequest is the GitHub pull request of a work tree's current branch, as
// reported by the gh CLI, whatever its state: one of gh's OPEN, CLOSED, MERGED.
type PullRequest struct {
	Number      int    `json:"number"`
	URL         string `json:"url"`
	Title       string `json:"title"`
	State       string `json:"state"`
	HeadRefName string `json:"headRefName"`
	BaseRefName string `json:"baseRefName"`
}

// BranchPullRequest is one read of the pull request of the branch checked out
// at Path; PR is nil when the branch has none.
type BranchPullRequest struct {
	Path string       `json:"path"`
	PR   *PullRequest `json:"pr"`
}

// DiffStats is what one status read says about a work tree: its uncommitted
// changes, and the two things they sit on. Head is the commit — the window
// watches it to notice a commit the way it watches Files/Added/Deleted to
// notice an edit — and Branch is the branch, carried here because git answers
// both in the same call the counts come out of.
type DiffStats struct {
	Files   int    `json:"files"`
	Added   int    `json:"added"`
	Deleted int    `json:"deleted"`
	Head    string `json:"head"`
	Branch  string `json:"branch"`
}

// PRDetail is the full view of a branch's open pull request — richer than the
// footer badge's PullRequest — driving the dock's Pulls panel: the title, body,
// CI rollup and mergeability gate the merge affordance. gh's state does not
// travel with it: a detail exists only when the PR is open (parsePRDetail).
type PRDetail struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	// Author is the login of whoever opened it, or their display name when gh
	// reports no login. GitHub refuses a review request addressed to them, which
	// is what keeps them out of the picker.
	Author string `json:"author"`
	// AuthorLogin is the same account as Author, minus the fallback to a display
	// name — the value a commit's login is compared against. Author cannot serve:
	// a name matches no login, so a PR whose author gh reports without one would
	// read as every commit landing under the wrong account.
	AuthorLogin string `json:"authorLogin"`
	// State is gh's OPEN | CLOSED | MERGED. Only a number-addressed lookup can
	// return a non-OPEN one; the branch lookup still gates them out.
	State     string `json:"state"`
	IsDraft   bool   `json:"isDraft"`
	Mergeable string `json:"mergeable"` // gh: MERGEABLE | CONFLICTING | UNKNOWN
	// MergeStateStatus is GitHub's own answer to "can this merge right now" —
	// CLEAN | UNSTABLE | BLOCKED | BEHIND | DIRTY | DRAFT | HAS_HOOKS | UNKNOWN.
	// Mergeable alone cannot gate the button: it says nothing about a required
	// review or a required check, and it reads UNKNOWN while GitHub recomputes
	// after a push.
	MergeStateStatus string `json:"mergeStateStatus"`
	// ReviewDecision is gh's aggregate verdict — APPROVED | CHANGES_REQUESTED |
	// REVIEW_REQUIRED — and "" where the repository requires no review. Who
	// reviewed is a different field (latestReviews) this does not carry.
	ReviewDecision string `json:"reviewDecision"`
	// Reviewers is who was asked and who answered, in one list (prreviewers.go).
	Reviewers    []PRReviewer `json:"reviewers"`
	BaseRefName  string       `json:"baseRefName"`
	HeadRefName  string       `json:"headRefName"`
	ChangedFiles int          `json:"changedFiles"`
	Additions    int          `json:"additions"`
	Deletions    int          `json:"deletions"`
	Checks       ChecksRollup `json:"checks"`
	CheckRuns    []CheckItem  `json:"checkRuns"`
	Commits      []PRCommit   `json:"commits"`
	// IsCrossRepository marks a head branch that lives on a fork.
	IsCrossRepository bool `json:"isCrossRepository"`
	// MaintainerCanModify is GitHub's "allow edits by maintainers" on a fork PR,
	// and the whole of what decides whether a session can be opened on one: with
	// it a push back to the fork's branch is authorised, without it there is
	// nowhere for an agent's commits to go (CreateWorktreeFromPR). It is false on
	// a same-repository PR, where nothing needs the permission.
	MaintainerCanModify bool `json:"maintainerCanModify"`
}

// PRCommit is one commit the pull request would land, split the way git itself
// splits a message: the subject line, then the body (empty for a one-liner).
//
// Login and Name are kept apart because only one of them is evidence: Name is
// free text, Login is the account GitHub resolved the author's email to against
// every account's verified addresses — an answer lich cannot compute, and one
// whose "" means the email belongs to nobody rather than "unknown".
type PRCommit struct {
	OID      string `json:"oid"`
	Headline string `json:"headline"`
	Body     string `json:"body"`
	Login    string `json:"login"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Date     string `json:"date"` // gh's ISO committedDate
}

// PRSummary is one row of the repository's open pull requests — what the list
// column needs to render and rank a PR before anything is selected. The full
// view behind a row is PRDetail, fetched per PR.
type PRSummary struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	Author  string `json:"author"` // login, or the display name when gh reports no login
	State   string `json:"state"`  // gh: OPEN | CLOSED | MERGED
	IsDraft bool   `json:"isDraft"`
	// ReviewDecision is gh's APPROVED | CHANGES_REQUESTED | REVIEW_REQUIRED,
	// and "" for a repository that requires no review.
	ReviewDecision string `json:"reviewDecision"`
	HeadRefName    string `json:"headRefName"`
	// IsCrossRepository marks a PR whose head branch lives on a fork. lich can
	// check such a branch out, but the commits would have nowhere to push, so
	// the session flow refuses it (CreateWorktreeFromPR) and the button reads
	// as blocked rather than failing on click.
	IsCrossRepository bool         `json:"isCrossRepository"`
	UpdatedAt         string       `json:"updatedAt"` // gh's ISO timestamp
	Checks            ChecksRollup `json:"checks"`
	// Threads is counted only on a CHANGES_REQUESTED row (prlistthreads.go);
	// every other row carries a zero tally.
	Threads ThreadTally `json:"threads"`
}

// CommitIdentity is the name and email the next commit in a checkout would be
// authored with — the fact the project's gh account (ghaccount.go) does not
// carry, since that one only governs what lich asks gh. Local says the value
// comes from the checkout's own config rather than the global one.
//
// The zero value is a legitimate answer, not a failure: a checkout with no
// identity configured is one of the states worth showing.
type CommitIdentity struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Local bool   `json:"local"`
}

// RevertLine names one changed line of the diff the panel drew. Side "old" is a
// deletion, numbered in HEAD; "new" is an addition, numbered in the working
// tree. Text is the line without its +/- prefix, and is what proves the number
// still points at the same line.
type RevertLine struct {
	Side string `json:"side"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

// PRReviewer is one entry of a pull request's review roster: somebody who was
// asked and has not answered yet, or somebody who has.
type PRReviewer struct {
	// Login is the account's login, or a team's slug — which is not what gh takes
	// to add or remove one (that is org/slug), so a team is shown and not edited.
	Login  string `json:"login"`
	IsTeam bool   `json:"isTeam"`
	// State is "" while the review is still owed, else gh's APPROVED |
	// CHANGES_REQUESTED | COMMENTED | DISMISSED.
	State string `json:"state"`
}

// PRConversation is everything said about a pull request: the reviews and their
// verdicts, the comments left on the pull request itself, and the threads
// anchored to a file and line. The Pulls screen renders the anchored ones inside
// the diff and the rest in its Conversation tab.
type PRConversation struct {
	// HeadRefOID is the commit a new review comment is filed against. GitHub
	// defaults to the latest commit, but naming it means a review submitted
	// seconds after a push cannot silently land on lines nobody read.
	HeadRefOID string     `json:"headRefOid"`
	Reviews    []PRReview `json:"reviews"`
	// Comments are the pull request's own comments — the ones with no file and
	// no line, which GitHub files as issue comments.
	Comments []PRComment      `json:"comments"`
	Threads  []PRReviewThread `json:"threads"`
}

// PRReview is one submitted review: its verdict, and whatever was written in the
// summary box above the line comments.
type PRReview struct {
	// ID is the GraphQL node id, which is what dismissing the review takes.
	ID string `json:"id"`
	// DatabaseID is the REST id, the address its summary is rewritten at.
	DatabaseID int64  `json:"databaseId"`
	Author     string `json:"author"`
	// Avatar is the address of the author's picture, read with
	// ProjectPullRequestAvatar; "" for an account deleted since.
	Avatar string `json:"avatar,omitempty"`
	// State is gh's APPROVED | CHANGES_REQUESTED | COMMENTED | DISMISSED.
	State   string `json:"state"`
	Body    string `json:"body"`
	Date    string `json:"date"`
	CanEdit bool   `json:"canEdit"`
}

// PRComment is one comment on the pull request itself.
type PRComment struct {
	// ID is the REST database id, the address an edit or a delete takes.
	ID        int64  `json:"id"`
	Author    string `json:"author"`
	Avatar    string `json:"avatar,omitempty"`
	Body      string `json:"body"`
	Date      string `json:"date"`
	CanEdit   bool   `json:"canEdit"`
	CanDelete bool   `json:"canDelete"`
}

// PRReviewThread is one conversation anchored to a line of the diff.
type PRReviewThread struct {
	// ID is the GraphQL node id, which is what resolving a thread takes. It is
	// not the number a reply is addressed to — that is a comment's own ID.
	ID   string `json:"id"`
	Path string `json:"path"`
	// Line is the anchor in the file the thread's side names, and StartLine the
	// first line of a multi-line thread (0 when it covers one line). A thread
	// GitHub could not re-anchor keeps its original line and is marked outdated,
	// which is what sends it to the Conversation tab instead of the diff.
	Line      int `json:"line"`
	StartLine int `json:"startLine"`
	// Side is RIGHT for a thread on the new file, LEFT for one on a deleted
	// line — the two sides the diff renders in one document.
	Side       string            `json:"side"`
	IsResolved bool              `json:"isResolved"`
	IsOutdated bool              `json:"isOutdated"`
	Comments   []PRThreadComment `json:"comments"`
}

// PRThreadComment is one message inside a thread.
type PRThreadComment struct {
	// ID is the REST database id — the address a reply is posted to. GraphQL
	// node ids do not work there, which is why both identifiers travel.
	ID     int64  `json:"id"`
	Author string `json:"author"`
	Avatar string `json:"avatar,omitempty"`
	Body   string `json:"body"`
	Date   string `json:"date"`
	// DiffHunk is GitHub's own snippet of the lines the thread is about. The
	// diff already has them where the thread is anchored; this is what an
	// outdated thread has left.
	DiffHunk string `json:"diffHunk"`
	// CanEdit is GitHub's own answer to whether the account reading may
	// rewrite the comment, which is what the Edit button stands on.
	CanEdit   bool `json:"canEdit"`
	CanDelete bool `json:"canDelete"`
}

// PRChanged names the checkout the write ran in and the pull request it
// addressed; Number is 0 where the write addresses a thread or a review rather
// than a number (the checkout's repository is then all that is known).
type PRChanged struct {
	Path   string `json:"path"`
	Number int    `json:"number"`
}

// SearchMatch is one line of one file that contains the query.
type SearchMatch struct {
	Path string `json:"path"`
	// Line is 1-based, the same numbering the preview's gutter shows.
	Line int `json:"line"`
	// Text is the line with its indentation dropped, cut to an excerpt around
	// the first occurrence when the line is long.
	Text string `json:"text"`
}

// SearchResult is one search over a checkout's files.
type SearchResult struct {
	Matches []SearchMatch `json:"matches"`
	// Cut reports a search that stopped at maxSearchMatches.
	Cut bool `json:"cut"`
	// TooLarge counts the listed files skipped for being above maxReadFileSize.
	// The tree shows them, so an answer that left them out says so; a binary
	// is skipped without a count, since nobody searches one for text.
	TooLarge int `json:"tooLarge"`
}

// ChecksRollup collapses gh's statusCheckRollup array into the counts the panel
// shows. Total is every check, so "all green" is Passed == Total && Total > 0.
type ChecksRollup struct {
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Pending int `json:"pending"`
	Total   int `json:"total"`
}

// CheckItem is one check, flattened from gh's two rollup shapes into what the
// Checks tab lists. State is this package's own passed|failed|pending, the same
// verdict ChecksRollup counts. The timestamps are gh's ISO strings, left for the
// window to turn into a duration.
type CheckItem struct {
	Name        string `json:"name"`
	State       string `json:"state"`
	Description string `json:"description"` // workflow name, or the status context's own line
	URL         string `json:"url"`         // where to read the run; "" when gh reports none
	StartedAt   string `json:"startedAt"`
	CompletedAt string `json:"completedAt"`
	// Required is GitHub's own answer to "does this check gate the merge", the
	// same verdict its checks list labels Required.
	Required bool `json:"required"`
}

// ReviewComment is one line comment of a review being submitted. Line is the
// last line it covers and StartLine the first (0 for a single-line comment) —
// GitHub's own convention, kept here so a selection crosses unchanged.
type ReviewComment struct {
	Path      string `json:"path"`
	Line      int    `json:"line"`
	StartLine int    `json:"startLine"`
	// Side is RIGHT (the new file) or LEFT (a deleted line). Empty means RIGHT,
	// which is where all but a deletion comment goes.
	Side string `json:"side"`
	Body string `json:"body"`
}

// Worktree is a git worktree checkout: the branch it holds and its path.
type Worktree struct {
	Name string `json:"name"`
	Path string `json:"path"`
	// Whether CreateWorktree checked out a branch that already existed instead
	// of starting one at the base it was given — the one case where the base
	// the caller picked decided nothing. Always false on a listed worktree.
	Reused bool `json:"reused,omitempty"`
}

// Branches groups everything the base-branch picker offers: local and remote
// branch names plus already-existing worktrees, so a closed-but-kept worktree
// can be resumed instead of recreated.
type Branches struct {
	Local     []string   `json:"local"`
	Remote    []string   `json:"remote"` // "origin/main" form
	Worktrees []Worktree `json:"worktrees"`
}

// RevertResult is what undoing a revert needs: the patch reversed onto the
// working tree, and the one reversed onto the index ("" when the index did not
// hold the change and was left alone).
type RevertResult struct {
	Patch      string
	IndexPatch string
}

// revertWire is RevertResult as it crosses the RPC: the patches are git's raw
// bytes, which need not be UTF-8 (a Latin-1 file), so they travel as base64
// rather than as JSON text that would turn each such byte into U+FFFD.
type revertWire struct {
	Patch      []byte `json:"patch"`
	IndexPatch []byte `json:"indexPatch"`
}

func (r RevertResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(revertWire{Patch: []byte(r.Patch), IndexPatch: []byte(r.IndexPatch)})
}

func (r *RevertResult) UnmarshalJSON(data []byte) error {
	var wire revertWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	r.Patch, r.IndexPatch = string(wire.Patch), string(wire.IndexPatch)
	return nil
}
