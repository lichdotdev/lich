package backend

import (
	"context"

	"github.com/lichdotdev/lich/api"
)

type PRChanged = api.PRChanged

type ChecksRollup = api.ChecksRollup

type CheckItem = api.CheckItem

type ThreadTally = api.ThreadTally

type PRSummary = api.PRSummary

type PRReviewer = api.PRReviewer

type PRCommit = api.PRCommit

type PRDetail = api.PRDetail

type CommitPattern = api.CommitPattern

type BranchRules = api.BranchRules

// ListPullRequests reads the repository's pull requests in state
// (open|closed|merged|all) through gh, at most the backend's page of them.
func (c *Client) ListPullRequests(ctx context.Context, path, state string) ([]PRSummary, error) {
	return callMethod(ctx, c, api.ProjectListPullRequests, api.ListPullRequestsOptions{Path: path, State: state})
}

// PullRequestDetail reads pull request number; 0 is the PR of path's branch,
// nil when it has none.
func (c *Client) PullRequestDetail(ctx context.Context, path string, number int) (*PRDetail, error) {
	return callMethod(ctx, c, api.ProjectPullRequestDetail, api.NumberOptions{Path: path, Number: number})
}

// PullRequestConflicts works out which files conflict with base; it fetches
// both refs first.
func (c *Client) PullRequestConflicts(ctx context.Context, path string, number int, base, url string) ([]string, error) {
	return callMethod(ctx, c, api.ProjectPullRequestConflicts, api.PRConflictsOptions{Path: path, Number: number, Base: base, PRURL: url})
}

func (c *Client) BranchRules(ctx context.Context, path, branch string) (*BranchRules, error) {
	return callMethod(ctx, c, api.ProjectBranchRules, api.PathBranchOptions{Path: path, Branch: branch})
}

// MergePullRequest merges with method (squash|merge|rebase); an empty subject
// keeps gh's message, admin bypasses the base branch's rules.
func (c *Client) MergePullRequest(ctx context.Context, path string, number int, method, subject, body string, admin bool) error {
	_, err := callMethod(ctx, c, api.ProjectMergePullRequest, api.MergePullRequestOptions{Path: path, Number: number, Method: method, Subject: subject, Body: body, Admin: admin})
	return err
}

// CreatePullRequest opens GitHub's new pull request page for path's branch.
func (c *Client) CreatePullRequest(ctx context.Context, path string) error {
	_, err := callMethod(ctx, c, api.ProjectCreatePullRequest, api.PathOptions{Path: path})
	return err
}

// CreateWorktreeFromPR checks pull request number out into a new worktree.
func (c *Client) CreateWorktreeFromPR(ctx context.Context, projectPath, projectID string, number int) (*Worktree, error) {
	return callMethod(ctx, c, api.ProjectCreateWorktreeFromPR, api.CreateWorktreeFromPROptions{ProjectPath: projectPath, ProjectID: projectID, Number: number})
}

// ListCheckouts is every checkout of path's repository holding a branch, the
// project's own directory included; Name is the branch.
func (c *Client) ListCheckouts(ctx context.Context, path string) ([]Worktree, error) {
	return callMethod(ctx, c, api.ProjectListCheckouts, api.PathOptions{Path: path})
}

func (c *Client) WorktreeAdopted(ctx context.Context, path string) (bool, error) {
	return callMethod(ctx, c, api.ProjectWorktreeAdopted, api.PathOptions{Path: path})
}

func (c *Client) WorktreeDirty(ctx context.Context, path string) (bool, error) {
	return callMethod(ctx, c, api.ProjectWorktreeDirty, api.PathOptions{Path: path})
}

// RemoveWorktree removes the checkout at path; git refuses a dirty one
// without force, and an adopted one is refused without adoptedAck.
func (c *Client) RemoveWorktree(ctx context.Context, projectPath, path string, force, adoptedAck bool) error {
	_, err := callMethod(ctx, c, api.ProjectRemoveWorktree, api.RemoveWorktreeOptions{ProjectPath: projectPath, WorktreePath: path, Force: force, AdoptedAck: adoptedAck})
	return err
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

type ReviewComment = api.ReviewComment

// SubmitReview sends a review: event is approve|comment|request_changes,
// head the commit the comments were written against ("" lets GitHub assume
// its latest).
func (c *Client) SubmitReview(ctx context.Context, path string, number int, event, body, head string, comments []ReviewComment) error {
	_, err := callMethod(ctx, c, api.ProjectSubmitReview, api.SubmitReviewOptions{Path: path, Number: number, Event: event, Body: body, Head: head, Comments: comments})
	return err
}

// PullRequestDiff is the unified diff pull request number would merge, as
// GitHub computes it.
func (c *Client) PullRequestDiff(ctx context.Context, path string, number int) (string, error) {
	return callMethod(ctx, c, api.ProjectPullRequestDiff, api.NumberOptions{Path: path, Number: number})
}

// PullRequestMergeBase is the commit pull request number's diff is taken
// from: the old side of every file in it.
func (c *Client) PullRequestMergeBase(ctx context.Context, path string, number int) (string, error) {
	return callMethod(ctx, c, api.ProjectPullRequestMergeBase, api.NumberOptions{Path: path, Number: number})
}

type PRConversation = api.PRConversation

type PRReview = api.PRReview

type PRComment = api.PRComment

type PRReviewThread = api.PRReviewThread

type PRThreadComment = api.PRThreadComment

func (c *Client) PullRequestConversation(ctx context.Context, path string, number int) (*PRConversation, error) {
	return callMethod(ctx, c, api.ProjectPullRequestConversation, api.NumberOptions{Path: path, Number: number})
}

// PullRequestAvatar reads the picture at url, an author's Avatar in a
// conversation read from the checkout at path.
func (c *Client) PullRequestAvatar(ctx context.Context, path, url string) ([]byte, error) {
	return callMethod(ctx, c, api.ProjectPullRequestAvatar, api.AvatarOptions{Path: path, URL: url})
}

// ReplyToReviewThread answers the thread comment commentID belongs to.
func (c *Client) ReplyToReviewThread(ctx context.Context, path string, number int, commentID int64, body string) error {
	_, err := callMethod(ctx, c, api.ProjectReplyToReviewThread, api.PRCommentBodyOptions{Path: path, Number: number, CommentID: commentID, Body: body})
	return err
}

// EditReviewComment rewrites thread comment commentID.
func (c *Client) EditReviewComment(ctx context.Context, path string, number int, commentID int64, body string) error {
	_, err := callMethod(ctx, c, api.ProjectEditReviewComment, api.PRCommentBodyOptions{Path: path, Number: number, CommentID: commentID, Body: body})
	return err
}

// DeleteReviewComment removes thread comment commentID.
func (c *Client) DeleteReviewComment(ctx context.Context, path string, number int, commentID int64) error {
	_, err := callMethod(ctx, c, api.ProjectDeleteReviewComment, api.PRCommentOptions{Path: path, Number: number, CommentID: commentID})
	return err
}

// EditReviewBody rewrites the summary of review reviewID (its database id).
func (c *Client) EditReviewBody(ctx context.Context, path string, number int, reviewID int64, body string) error {
	_, err := callMethod(ctx, c, api.ProjectEditReviewBody, api.PRReviewBodyOptions{Path: path, Number: number, ReviewID: reviewID, Body: body})
	return err
}

// ResolveReviewThread resolves thread threadID, or reopens it.
func (c *Client) ResolveReviewThread(ctx context.Context, path, threadID string, resolved bool) error {
	_, err := callMethod(ctx, c, api.ProjectResolveReviewThread, api.ResolveThreadOptions{Path: path, ThreadID: threadID, Resolved: resolved})
	return err
}

// CommentOnPullRequest comments on the pull request itself.
func (c *Client) CommentOnPullRequest(ctx context.Context, path string, number int, body string) error {
	_, err := callMethod(ctx, c, api.ProjectCommentOnPullRequest, api.PRBodyOptions{Path: path, Number: number, Body: body})
	return err
}

// EditPullRequestComment rewrites pull request comment commentID.
func (c *Client) EditPullRequestComment(ctx context.Context, path string, number int, commentID int64, body string) error {
	_, err := callMethod(ctx, c, api.ProjectEditPullRequestComment, api.PRCommentBodyOptions{Path: path, Number: number, CommentID: commentID, Body: body})
	return err
}

// DeletePullRequestComment removes pull request comment commentID.
func (c *Client) DeletePullRequestComment(ctx context.Context, path string, number int, commentID int64) error {
	_, err := callMethod(ctx, c, api.ProjectDeletePullRequestComment, api.PRCommentOptions{Path: path, Number: number, CommentID: commentID})
	return err
}

// EditPullRequestTitle renames the pull request.
func (c *Client) EditPullRequestTitle(ctx context.Context, path string, number int, title string) error {
	_, err := callMethod(ctx, c, api.ProjectEditPullRequestTitle, api.PRTitleOptions{Path: path, Number: number, Title: title})
	return err
}

// EditPullRequestBody rewrites the pull request's description; empty clears it.
func (c *Client) EditPullRequestBody(ctx context.Context, path string, number int, body string) error {
	_, err := callMethod(ctx, c, api.ProjectEditPullRequestBody, api.PRBodyOptions{Path: path, Number: number, Body: body})
	return err
}
