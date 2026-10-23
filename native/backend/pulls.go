package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

// PRChangedEvent is project.PRChangedEvent: a pull request write through lich
// landed on GitHub.
const PRChangedEvent = "pull-request-changed"

// PRChanged mirrors project.PRChanged.
type PRChanged struct {
	Path   string `json:"path"`
	Number int    `json:"number"`
}

// ChecksRollup mirrors project.ChecksRollup.
type ChecksRollup struct {
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Pending int `json:"pending"`
	Total   int `json:"total"`
}

// CheckItem mirrors project.CheckItem; State is passed|failed|pending.
type CheckItem struct {
	Name        string `json:"name"`
	State       string `json:"state"`
	Description string `json:"description"`
	URL         string `json:"url"`
	StartedAt   string `json:"startedAt"`
	CompletedAt string `json:"completedAt"`
	Required    bool   `json:"required"`
}

// ThreadTally mirrors project.ThreadTally.
type ThreadTally struct {
	Open  int `json:"open"`
	Total int `json:"total"`
}

// PRSummary mirrors project.PRSummary: one row of the list.
type PRSummary struct {
	Number            int          `json:"number"`
	Title             string       `json:"title"`
	Author            string       `json:"author"`
	State             string       `json:"state"`
	IsDraft           bool         `json:"isDraft"`
	ReviewDecision    string       `json:"reviewDecision"`
	HeadRefName       string       `json:"headRefName"`
	IsCrossRepository bool         `json:"isCrossRepository"`
	UpdatedAt         string       `json:"updatedAt"`
	Checks            ChecksRollup `json:"checks"`
	Threads           ThreadTally  `json:"threads"`
}

// PRReviewer mirrors project.PRReviewer.
type PRReviewer struct {
	Login  string `json:"login"`
	IsTeam bool   `json:"isTeam"`
	State  string `json:"state"`
}

// PRCommit mirrors project.PRCommit.
type PRCommit struct {
	OID      string `json:"oid"`
	Headline string `json:"headline"`
	Body     string `json:"body"`
	Login    string `json:"login"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Date     string `json:"date"`
}

// PRDetail mirrors project.PRDetail.
type PRDetail struct {
	Number              int          `json:"number"`
	URL                 string       `json:"url"`
	Title               string       `json:"title"`
	Body                string       `json:"body"`
	Author              string       `json:"author"`
	AuthorLogin         string       `json:"authorLogin"`
	State               string       `json:"state"`
	IsDraft             bool         `json:"isDraft"`
	Mergeable           string       `json:"mergeable"`
	MergeStateStatus    string       `json:"mergeStateStatus"`
	ReviewDecision      string       `json:"reviewDecision"`
	Reviewers           []PRReviewer `json:"reviewers"`
	BaseRefName         string       `json:"baseRefName"`
	HeadRefName         string       `json:"headRefName"`
	ChangedFiles        int          `json:"changedFiles"`
	Checks              ChecksRollup `json:"checks"`
	CheckRuns           []CheckItem  `json:"checkRuns"`
	Commits             []PRCommit   `json:"commits"`
	IsCrossRepository   bool         `json:"isCrossRepository"`
	MaintainerCanModify bool         `json:"maintainerCanModify"`
}

// CommitPattern mirrors project.CommitPattern.
type CommitPattern struct {
	Target   string `json:"target"`
	Operator string `json:"operator"`
	Pattern  string `json:"pattern"`
	Negate   bool   `json:"negate"`
}

// BranchRules mirrors project.BranchRules.
type BranchRules struct {
	AllowedMergeMethods []string        `json:"allowedMergeMethods"`
	CommitPatterns      []CommitPattern `json:"commitPatterns"`
	ViewerCanBypass     bool            `json:"viewerCanBypass"`
}

// ListPullRequests reads the repository's pull requests in state
// (open|closed|merged|all) through gh, at most the backend's page of them.
func (c *Client) ListPullRequests(ctx context.Context, path, state string) ([]PRSummary, error) {
	var list []PRSummary
	return list, c.Call(ctx, "project.ListPullRequests", &list, path, state)
}

// PullRequestDetail reads pull request number; 0 is the PR of path's branch,
// nil when it has none.
func (c *Client) PullRequestDetail(ctx context.Context, path string, number int) (*PRDetail, error) {
	var d *PRDetail
	return d, c.Call(ctx, "project.PullRequestDetail", &d, path, number)
}

// PullRequestConflicts works out which files conflict with base; it fetches
// both refs first.
func (c *Client) PullRequestConflicts(ctx context.Context, path string, number int, base, url string) ([]string, error) {
	var files []string
	return files, c.Call(ctx, "project.PullRequestConflicts", &files, path, number, base, url)
}

func (c *Client) BranchRules(ctx context.Context, path, branch string) (*BranchRules, error) {
	var r *BranchRules
	return r, c.Call(ctx, "project.BranchRules", &r, path, branch)
}

// MergePullRequest merges with method (squash|merge|rebase); an empty subject
// keeps gh's message, admin bypasses the base branch's rules.
func (c *Client) MergePullRequest(ctx context.Context, path string, number int, method, subject, body string, admin bool) error {
	return c.Call(ctx, "project.MergePullRequest", nil, path, number, method, subject, body, admin)
}

// CreatePullRequest opens GitHub's new pull request page for path's branch.
func (c *Client) CreatePullRequest(ctx context.Context, path string) error {
	return c.Call(ctx, "project.CreatePullRequest", nil, path)
}

// CreateWorktreeFromPR checks pull request number out into a new worktree.
func (c *Client) CreateWorktreeFromPR(ctx context.Context, projectPath, projectID string, number int) (*Worktree, error) {
	var wt *Worktree
	return wt, c.Call(ctx, "project.CreateWorktreeFromPR", &wt, projectPath, projectID, number)
}

// ListCheckouts is every checkout of path's repository holding a branch, the
// project's own directory included; Name is the branch.
func (c *Client) ListCheckouts(ctx context.Context, path string) ([]Worktree, error) {
	var list []Worktree
	return list, c.Call(ctx, "project.ListCheckouts", &list, path)
}

func (c *Client) WorktreeAdopted(ctx context.Context, path string) (bool, error) {
	var ok bool
	return ok, c.Call(ctx, "project.WorktreeAdopted", &ok, path)
}

func (c *Client) WorktreeDirty(ctx context.Context, path string) (bool, error) {
	var dirty bool
	return dirty, c.Call(ctx, "project.WorktreeDirty", &dirty, path)
}

// RemoveWorktree removes the checkout at path; git refuses a dirty one
// without force, and an adopted one is refused without adoptedAck.
func (c *Client) RemoveWorktree(ctx context.Context, projectPath, path string, force, adoptedAck bool) error {
	return c.Call(ctx, "project.RemoveWorktree", nil, projectPath, path, force, adoptedAck)
}

// PurgeWorktreeSessions drops every stored session rooted in path.
func (c *Client) PurgeWorktreeSessions(ctx context.Context, projectID, path string) error {
	_, err := callMethod(ctx, c, api.StorePurgeWorktreeSessions, api.WorktreeSessionsOptions{ProjectID: projectID, Path: path})
	return err
}

// AnnounceMerge tells the agents in checkout that its pull request merged.
func (c *Client) AnnounceMerge(ctx context.Context, checkout string, number int, title, branch, base string) error {
	_, err := callMethod(ctx, c, api.RelayAnnounceMerge, api.AnnounceMergeOptions{
		Checkout: checkout, Number: number, Title: title, Branch: branch, Base: base,
	})
	return err
}

// Submit types message at session id's prompt and sends it; the session has to
// be a running agent.
func (c *Client) Submit(ctx context.Context, id, message string) error {
	_, err := callMethod(ctx, c, api.RelaySubmit, api.SubmitOptions{SessionID: id, Message: message})
	return err
}

// ReviewComment mirrors project.ReviewComment: one line comment of a review.
type ReviewComment struct {
	Path      string `json:"path"`
	Line      int    `json:"line"`
	StartLine int    `json:"startLine"`
	Side      string `json:"side"`
	Body      string `json:"body"`
}

// SubmitReview sends a review: event is approve|comment|request_changes,
// head the commit the comments were written against ("" lets GitHub assume
// its latest).
func (c *Client) SubmitReview(ctx context.Context, path string, number int, event, body, head string, comments []ReviewComment) error {
	return c.Call(ctx, "project.SubmitReview", nil, path, number, event, body, head, comments)
}

// PullRequestDiff is the unified diff pull request number would merge, as
// GitHub computes it.
func (c *Client) PullRequestDiff(ctx context.Context, path string, number int) (string, error) {
	var diff string
	return diff, c.Call(ctx, "project.PullRequestDiff", &diff, path, number)
}

// PullRequestMergeBase is the commit pull request number's diff is taken
// from: the old side of every file in it.
func (c *Client) PullRequestMergeBase(ctx context.Context, path string, number int) (string, error) {
	var oid string
	return oid, c.Call(ctx, "project.PullRequestMergeBase", &oid, path, number)
}

// PRConversation mirrors project.PRConversation.
type PRConversation struct {
	HeadRefOID string           `json:"headRefOid"`
	Reviews    []PRReview       `json:"reviews"`
	Comments   []PRComment      `json:"comments"`
	Threads    []PRReviewThread `json:"threads"`
}

// PRReview mirrors project.PRReview.
type PRReview struct {
	ID         string `json:"id"`
	DatabaseID int64  `json:"databaseId"`
	Author     string `json:"author"`
	State      string `json:"state"`
	Body       string `json:"body"`
	Date       string `json:"date"`
	CanEdit    bool   `json:"canEdit"`
}

// PRComment mirrors project.PRComment.
type PRComment struct {
	ID        int64  `json:"id"`
	Author    string `json:"author"`
	Body      string `json:"body"`
	Date      string `json:"date"`
	CanEdit   bool   `json:"canEdit"`
	CanDelete bool   `json:"canDelete"`
}

// PRReviewThread mirrors project.PRReviewThread.
type PRReviewThread struct {
	ID         string            `json:"id"`
	Path       string            `json:"path"`
	Line       int               `json:"line"`
	StartLine  int               `json:"startLine"`
	Side       string            `json:"side"`
	IsResolved bool              `json:"isResolved"`
	IsOutdated bool              `json:"isOutdated"`
	Comments   []PRThreadComment `json:"comments"`
}

// PRThreadComment mirrors project.PRThreadComment.
type PRThreadComment struct {
	ID        int64  `json:"id"`
	Author    string `json:"author"`
	Body      string `json:"body"`
	Date      string `json:"date"`
	DiffHunk  string `json:"diffHunk"`
	CanEdit   bool   `json:"canEdit"`
	CanDelete bool   `json:"canDelete"`
}

func (c *Client) PullRequestConversation(ctx context.Context, path string, number int) (*PRConversation, error) {
	var conv *PRConversation
	return conv, c.Call(ctx, "project.PullRequestConversation", &conv, path, number)
}

// ReplyToReviewThread answers the thread comment commentID belongs to.
func (c *Client) ReplyToReviewThread(ctx context.Context, path string, number int, commentID int64, body string) error {
	return c.Call(ctx, "project.ReplyToReviewThread", nil, path, number, commentID, body)
}

// EditReviewComment rewrites thread comment commentID.
func (c *Client) EditReviewComment(ctx context.Context, path string, number int, commentID int64, body string) error {
	return c.Call(ctx, "project.EditReviewComment", nil, path, number, commentID, body)
}

// DeleteReviewComment removes thread comment commentID.
func (c *Client) DeleteReviewComment(ctx context.Context, path string, number int, commentID int64) error {
	return c.Call(ctx, "project.DeleteReviewComment", nil, path, number, commentID)
}

// EditReviewBody rewrites the summary of review reviewID (its database id).
func (c *Client) EditReviewBody(ctx context.Context, path string, number int, reviewID int64, body string) error {
	return c.Call(ctx, "project.EditReviewBody", nil, path, number, reviewID, body)
}

// ResolveReviewThread resolves thread threadID, or reopens it.
func (c *Client) ResolveReviewThread(ctx context.Context, path, threadID string, resolved bool) error {
	return c.Call(ctx, "project.ResolveReviewThread", nil, path, threadID, resolved)
}

// CommentOnPullRequest comments on the pull request itself.
func (c *Client) CommentOnPullRequest(ctx context.Context, path string, number int, body string) error {
	return c.Call(ctx, "project.CommentOnPullRequest", nil, path, number, body)
}

// EditPullRequestComment rewrites pull request comment commentID.
func (c *Client) EditPullRequestComment(ctx context.Context, path string, number int, commentID int64, body string) error {
	return c.Call(ctx, "project.EditPullRequestComment", nil, path, number, commentID, body)
}

// DeletePullRequestComment removes pull request comment commentID.
func (c *Client) DeletePullRequestComment(ctx context.Context, path string, number int, commentID int64) error {
	return c.Call(ctx, "project.DeletePullRequestComment", nil, path, number, commentID)
}

// EditPullRequestTitle renames the pull request.
func (c *Client) EditPullRequestTitle(ctx context.Context, path string, number int, title string) error {
	return c.Call(ctx, "project.EditPullRequestTitle", nil, path, number, title)
}

// EditPullRequestBody rewrites the pull request's description; empty clears it.
func (c *Client) EditPullRequestBody(ctx context.Context, path string, number int, body string) error {
	return c.Call(ctx, "project.EditPullRequestBody", nil, path, number, body)
}
