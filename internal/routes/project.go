package routes

import (
	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/project"
	"github.com/lichdotdev/lich/internal/rpc"
)

// Project exposes the project service as api declares it.
func Project(d *rpc.Handler, s *project.Service) {
	projectFiles(d, s)
	projectChanges(d, s)
	projectBranches(d, s)
	projectPullRequests(d, s)
	projectPullRequestWrites(d, s)
}

func projectFiles(d *rpc.Handler, s *project.Service) {
	rpc.Handle(d, api.ProjectOpen, s.Open)
	rpc.Handle(d, api.ProjectHome, s.Home)
	rpc.Handle(d, api.ProjectRelocate, s.Relocate)
	rpc.Handle(d, api.ProjectExists, s.Exists)
	rpc.Handle(d, api.ProjectMissing, s.Missing)
	rpc.Handle(d, api.ProjectPickFile, s.PickFile)
	rpc.Handle(d, api.ProjectPickSaveFile, s.PickSaveFile)
	rpc.Handle(d, api.ProjectTree, s.Tree)
	rpc.Handle(d, api.ProjectSearch, s.Search)
	rpc.Handle(d, api.ProjectReadFile, s.ReadFile)
	rpc.Handle(d, api.ProjectReadFileAt, s.ReadFileAt)
	rpc.Handle(d, api.ProjectFileLines, s.FileLines)
}

func projectChanges(d *rpc.Handler, s *project.Service) {
	rpc.Handle(d, api.ProjectDiff, s.Diff)
	rpc.Handle(d, api.ProjectDiffText, s.DiffText)
	rpc.Handle(d, api.ProjectDiscardFile, s.DiscardFile)
	rpc.Handle(d, api.ProjectRevertLines, s.RevertLines)
	rpc.Handle(d, api.ProjectRestoreLines, s.RestoreLines)
	rpc.Handle(d, api.ProjectCarryUncommitted, s.CarryUncommitted)
	rpc.Handle(d, api.ProjectCommitIdentity, s.CommitIdentity)
}

func projectBranches(d *rpc.Handler, s *project.Service) {
	rpc.Handle(d, api.ProjectBranchesOf, s.BranchesOf)
	rpc.Handle(d, api.ProjectBaseStatus, s.BaseStatus)
	rpc.Handle(d, api.ProjectBaseFreshness, s.BaseFreshness)
	rpc.Handle(d, api.ProjectBranchRules, s.BranchRules)
	rpc.Handle(d, api.ProjectListBranches, s.ListBranches)
	rpc.Handle(d, api.ProjectListCheckouts, s.ListCheckouts)
	rpc.Handle(d, api.ProjectCreateWorktree, s.CreateWorktree)
	rpc.Handle(d, api.ProjectCreateWorktreeFromPR, s.CreateWorktreeFromPR)
	rpc.Handle(d, api.ProjectRemoveWorktree, s.RemoveWorktree)
	rpc.Handle(d, api.ProjectWorktreeDirty, s.WorktreeDirty)
	rpc.Handle(d, api.ProjectWorktreeAdopted, s.WorktreeAdopted)
}

func projectPullRequests(d *rpc.Handler, s *project.Service) {
	rpc.Handle(d, api.ProjectGitHubAccounts, s.GitHubAccounts)
	rpc.Handle(d, api.ProjectIssue, s.Issue)
	rpc.Handle(d, api.ProjectRecheckPullRequest, s.RecheckPullRequest)
	rpc.Handle(d, api.ProjectListPullRequests, s.ListPullRequests)
	rpc.Handle(d, api.ProjectPullRequestDetail, s.PullRequestDetail)
	rpc.Handle(d, api.ProjectPullRequestConversation, s.PullRequestConversation)
	rpc.Handle(d, api.ProjectPullRequestDiff, s.PullRequestDiff)
	rpc.Handle(d, api.ProjectPullRequestMergeBase, s.PullRequestMergeBase)
	rpc.Handle(d, api.ProjectPullRequestConflicts, s.PullRequestConflicts)
	rpc.Handle(d, api.ProjectCreatePullRequest, s.CreatePullRequest)
	rpc.Handle(d, api.ProjectMergePullRequest, s.MergePullRequest)
}

func projectPullRequestWrites(d *rpc.Handler, s *project.Service) {
	rpc.Handle(d, api.ProjectEditPullRequestTitle, s.EditPullRequestTitle)
	rpc.Handle(d, api.ProjectEditPullRequestBody, s.EditPullRequestBody)
	rpc.Handle(d, api.ProjectCommentOnPullRequest, s.CommentOnPullRequest)
	rpc.Handle(d, api.ProjectEditPullRequestComment, s.EditPullRequestComment)
	rpc.Handle(d, api.ProjectDeletePullRequestComment, s.DeletePullRequestComment)
	rpc.Handle(d, api.ProjectSubmitReview, s.SubmitReview)
	rpc.Handle(d, api.ProjectEditReviewBody, s.EditReviewBody)
	rpc.Handle(d, api.ProjectEditReviewComment, s.EditReviewComment)
	rpc.Handle(d, api.ProjectDeleteReviewComment, s.DeleteReviewComment)
	rpc.Handle(d, api.ProjectReplyToReviewThread, s.ReplyToReviewThread)
	rpc.Handle(d, api.ProjectResolveReviewThread, s.ResolveReviewThread)
	rpc.Handle(d, api.ProjectAwaitThreadFix, s.AwaitThreadFix)
	rpc.Handle(d, api.ProjectThreadFixes, s.ThreadFixes)
}
