package routes

import (
	"context"

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
	rpc.Handle(d, api.ProjectOpen, func(context.Context, api.None) (*api.OpenedProject, error) {
		return s.Open()
	})
	rpc.Handle(d, api.ProjectHome, func(context.Context, api.None) (*api.OpenedProject, error) {
		return s.Home()
	})
	rpc.Handle(d, api.ProjectRelocate, func(_ context.Context, o api.ProjectIDOptions) (*api.OpenedProject, error) {
		return s.Relocate(o.ID)
	})
	rpc.Handle(d, api.ProjectExists, func(_ context.Context, o api.PathOptions) (bool, error) {
		return s.Exists(o.Path), nil
	})
	rpc.Handle(d, api.ProjectMissing, func(_ context.Context, o api.PathsOptions) ([]string, error) {
		return s.Missing(o.Paths), nil
	})
	rpc.Handle(d, api.ProjectPickFile, func(_ context.Context, o api.PickFileOptions) (string, error) {
		return s.PickFile(o.Title)
	})
	rpc.Handle(d, api.ProjectPickSaveFile, func(_ context.Context, o api.PickSaveFileOptions) (string, error) {
		return s.PickSaveFile(o.Title, o.DefaultName)
	})
	rpc.Handle(d, api.ProjectTree, func(_ context.Context, o api.PathOptions) (api.FileListing, error) {
		return s.Tree(o.Path)
	})
	rpc.Handle(d, api.ProjectSearch, func(_ context.Context, o api.SearchOptions) (api.SearchResult, error) {
		return s.Search(o.Path, o.Query)
	})
	rpc.Handle(d, api.ProjectReadFile, func(_ context.Context, o api.CheckoutFileOptions) (string, error) {
		return s.ReadFile(o.Path, o.Rel)
	})
	rpc.Handle(d, api.ProjectReadFileAt, func(_ context.Context, o api.ReadFileAtOptions) (string, error) {
		return s.ReadFileAt(o.Path, o.Rel, o.Ref)
	})
	rpc.Handle(d, api.ProjectFileLines, func(_ context.Context, o api.FileLinesOptions) ([]string, error) {
		return s.FileLines(o.Path, o.Rel, o.Ref, o.From, o.To)
	})
}

func projectChanges(d *rpc.Handler, s *project.Service) {
	rpc.Handle(d, api.ProjectDiff, func(_ context.Context, o api.PathOptions) (api.DiffStats, error) {
		return s.Diff(o.Path), nil
	})
	rpc.Handle(d, api.ProjectDiffText, func(_ context.Context, o api.PathOptions) (string, error) {
		return s.DiffText(o.Path)
	})
	rpc.Handle(d, api.ProjectDiscardFile, func(_ context.Context, o api.CheckoutFileOptions) (api.None, error) {
		return api.None{}, s.DiscardFile(o.Path, o.Rel)
	})
	rpc.Handle(d, api.ProjectRevertLines, func(_ context.Context, o api.RevertLinesOptions) (api.RevertResult, error) {
		return s.RevertLines(o.Path, o.Rel, o.Lines)
	})
	rpc.Handle(d, api.ProjectRestoreLines, func(_ context.Context, o api.RestoreLinesOptions) (api.None, error) {
		return api.None{}, s.RestoreLines(o.Path, o.Rel, o.Undo)
	})
	rpc.Handle(d, api.ProjectCarryUncommitted, func(_ context.Context, o api.CarryUncommittedOptions) (api.None, error) {
		return api.None{}, s.CarryUncommitted(o.Src, o.Dst)
	})
	rpc.Handle(d, api.ProjectCommitIdentity, func(_ context.Context, o api.PathOptions) (api.CommitIdentity, error) {
		return s.CommitIdentity(o.Path), nil
	})
}

func projectBranches(d *rpc.Handler, s *project.Service) {
	rpc.Handle(d, api.ProjectBranchesOf, func(_ context.Context, o api.PathsOptions) (map[string]string, error) {
		return s.BranchesOf(o.Paths), nil
	})
	rpc.Handle(d, api.ProjectBaseStatus, func(_ context.Context, o api.PathOptions) (*api.BaseStatus, error) {
		return s.BaseStatus(o.Path), nil
	})
	rpc.Handle(d, api.ProjectBaseFreshness, func(_ context.Context, o api.PathBranchOptions) (api.BaseFreshness, error) {
		return s.BaseFreshness(o.Path, o.Branch)
	})
	rpc.Handle(d, api.ProjectBranchRules, func(_ context.Context, o api.PathBranchOptions) (*api.BranchRules, error) {
		return s.BranchRules(o.Path, o.Branch)
	})
	rpc.Handle(d, api.ProjectListBranches, func(_ context.Context, o api.PathOptions) (api.Branches, error) {
		return s.ListBranches(o.Path)
	})
	rpc.Handle(d, api.ProjectListCheckouts, func(_ context.Context, o api.PathOptions) ([]api.Worktree, error) {
		return s.ListCheckouts(o.Path)
	})
	rpc.Handle(d, api.ProjectCreateWorktree, func(_ context.Context, o api.CreateWorktreeOptions) (*api.Worktree, error) {
		return s.CreateWorktree(o.ProjectPath, o.ProjectID, o.Name, o.Base, o.BaseIsRemote)
	})
	rpc.Handle(d, api.ProjectCreateWorktreeFromPR, func(_ context.Context, o api.CreateWorktreeFromPROptions) (*api.Worktree, error) {
		return s.CreateWorktreeFromPR(o.ProjectPath, o.ProjectID, o.Number)
	})
	rpc.Handle(d, api.ProjectRemoveWorktree, func(_ context.Context, o api.RemoveWorktreeOptions) (api.None, error) {
		return api.None{}, s.RemoveWorktree(o.ProjectPath, o.WorktreePath, o.Force, o.AdoptedAck)
	})
	rpc.Handle(d, api.ProjectWorktreeDirty, func(_ context.Context, o api.PathOptions) (bool, error) {
		return s.WorktreeDirty(o.Path)
	})
	rpc.Handle(d, api.ProjectWorktreeAdopted, func(_ context.Context, o api.PathOptions) (bool, error) {
		return s.WorktreeAdopted(o.Path), nil
	})
}

func projectPullRequests(d *rpc.Handler, s *project.Service) {
	rpc.Handle(d, api.ProjectGitHubAccounts, func(context.Context, api.None) ([]string, error) {
		return s.GitHubAccounts()
	})
	rpc.Handle(d, api.ProjectIssue, func(_ context.Context, o api.NumberOptions) (api.Issue, error) {
		return s.Issue(o.Path, o.Number)
	})
	rpc.Handle(d, api.ProjectPullRequest, func(_ context.Context, o api.PathOptions) (*api.PullRequest, error) {
		return s.PullRequest(o.Path), nil
	})
	rpc.Handle(d, api.ProjectListPullRequests, func(_ context.Context, o api.ListPullRequestsOptions) ([]api.PRSummary, error) {
		return s.ListPullRequests(o.Path, o.State)
	})
	rpc.Handle(d, api.ProjectPullRequestDetail, func(_ context.Context, o api.NumberOptions) (*api.PRDetail, error) {
		return s.PullRequestDetail(o.Path, o.Number)
	})
	rpc.Handle(d, api.ProjectPullRequestConversation, func(_ context.Context, o api.NumberOptions) (*api.PRConversation, error) {
		return s.PullRequestConversation(o.Path, o.Number)
	})
	rpc.Handle(d, api.ProjectPullRequestDiff, func(_ context.Context, o api.NumberOptions) (string, error) {
		return s.PullRequestDiff(o.Path, o.Number)
	})
	rpc.Handle(d, api.ProjectPullRequestMergeBase, func(_ context.Context, o api.NumberOptions) (string, error) {
		return s.PullRequestMergeBase(o.Path, o.Number)
	})
	rpc.Handle(d, api.ProjectPullRequestConflicts, func(_ context.Context, o api.PRConflictsOptions) ([]string, error) {
		return s.PullRequestConflicts(o.Path, o.Number, o.Base, o.PRURL)
	})
	rpc.Handle(d, api.ProjectCreatePullRequest, func(_ context.Context, o api.PathOptions) (api.None, error) {
		return api.None{}, s.CreatePullRequest(o.Path)
	})
	rpc.Handle(d, api.ProjectMergePullRequest, func(_ context.Context, o api.MergePullRequestOptions) (api.None, error) {
		return api.None{}, s.MergePullRequest(o.Path, o.Number, o.Method, o.Subject, o.Body, o.Admin)
	})
}

func projectPullRequestWrites(d *rpc.Handler, s *project.Service) {
	rpc.Handle(d, api.ProjectEditPullRequestTitle, func(_ context.Context, o api.PRTitleOptions) (api.None, error) {
		return api.None{}, s.EditPullRequestTitle(o.Path, o.Number, o.Title)
	})
	rpc.Handle(d, api.ProjectEditPullRequestBody, func(_ context.Context, o api.PRBodyOptions) (api.None, error) {
		return api.None{}, s.EditPullRequestBody(o.Path, o.Number, o.Body)
	})
	rpc.Handle(d, api.ProjectCommentOnPullRequest, func(_ context.Context, o api.PRBodyOptions) (api.None, error) {
		return api.None{}, s.CommentOnPullRequest(o.Path, o.Number, o.Body)
	})
	rpc.Handle(d, api.ProjectEditPullRequestComment, func(_ context.Context, o api.PRCommentBodyOptions) (api.None, error) {
		return api.None{}, s.EditPullRequestComment(o.Path, o.Number, o.CommentID, o.Body)
	})
	rpc.Handle(d, api.ProjectDeletePullRequestComment, func(_ context.Context, o api.PRCommentOptions) (api.None, error) {
		return api.None{}, s.DeletePullRequestComment(o.Path, o.Number, o.CommentID)
	})
	rpc.Handle(d, api.ProjectSubmitReview, func(_ context.Context, o api.SubmitReviewOptions) (api.None, error) {
		return api.None{}, s.SubmitReview(o.Path, o.Number, o.Event, o.Body, o.Head, o.Comments)
	})
	rpc.Handle(d, api.ProjectEditReviewBody, func(_ context.Context, o api.PRReviewBodyOptions) (api.None, error) {
		return api.None{}, s.EditReviewBody(o.Path, o.Number, o.ReviewID, o.Body)
	})
	rpc.Handle(d, api.ProjectEditReviewComment, func(_ context.Context, o api.PRCommentBodyOptions) (api.None, error) {
		return api.None{}, s.EditReviewComment(o.Path, o.Number, o.CommentID, o.Body)
	})
	rpc.Handle(d, api.ProjectDeleteReviewComment, func(_ context.Context, o api.PRCommentOptions) (api.None, error) {
		return api.None{}, s.DeleteReviewComment(o.Path, o.Number, o.CommentID)
	})
	rpc.Handle(d, api.ProjectReplyToReviewThread, func(_ context.Context, o api.PRCommentBodyOptions) (api.None, error) {
		return api.None{}, s.ReplyToReviewThread(o.Path, o.Number, o.CommentID, o.Body)
	})
	rpc.Handle(d, api.ProjectResolveReviewThread, func(_ context.Context, o api.ResolveThreadOptions) (api.None, error) {
		return api.None{}, s.ResolveReviewThread(o.Path, o.ThreadID, o.Resolved)
	})
}
