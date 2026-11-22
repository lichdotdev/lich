package project

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/lichdotdev/lich/api"
)

// gitlabForge is the glab side of forge. A merge request is read in GitLab's
// REST shape (glab -F json and glab api) and handed on in GitHub's vocabulary:
// iid is the number, opened is OPEN, the source branch is the head.
type gitlabForge struct{ run glabRunner }

func (gitlabForge) kind() string { return api.ForgeGitLab }

// GitHub's words for what a GitLab merge request reports.
const (
	prClosed            = "CLOSED"
	prMergeable         = "MERGEABLE"
	prConflicting       = "CONFLICTING"
	prUnknown           = "UNKNOWN"
	prBlocked           = "BLOCKED"
	prReviewRequired    = "REVIEW_REQUIRED"
	prApproved          = "APPROVED"
	glabOpened          = "opened"
	glabMergeableStatus = "mergeable"
)

// glabBranchPage is how many merge requests of one source branch are read to
// find the branch's own: the open one when there is one, else the newest.
const glabBranchPage = 20

// glabMR is the part of GitLab's merge request object lich reads.
type glabMR struct {
	IID                 int           `json:"iid"`
	State               string        `json:"state"`
	Draft               bool          `json:"draft"`
	Title               string        `json:"title"`
	Description         string        `json:"description"`
	Author              glabUser      `json:"author"`
	WebURL              string        `json:"web_url"`
	SourceBranch        string        `json:"source_branch"`
	TargetBranch        string        `json:"target_branch"`
	SHA                 string        `json:"sha"`
	ProjectID           int64         `json:"project_id"`
	SourceProjectID     int64         `json:"source_project_id"`
	TargetProjectID     int64         `json:"target_project_id"`
	AllowCollaboration  bool          `json:"allow_collaboration"`
	ChangesCount        string        `json:"changes_count"`
	DetailedMergeStatus string        `json:"detailed_merge_status"`
	HasConflicts        bool          `json:"has_conflicts"`
	UpdatedAt           string        `json:"updated_at"`
	HeadPipeline        *glabPipeline `json:"head_pipeline"`
}

type glabUser struct {
	Username string `json:"username"`
	Name     string `json:"name"`
}

// glabPipeline names a pipeline by its own project: on a merge request from a
// fork that is the fork, not the project the merge request targets.
type glabPipeline struct {
	ID        int64 `json:"id"`
	ProjectID int64 `json:"project_id"`
}

var glabStates = map[string]string{"opened": prOpen, "merged": prMerged, "closed": prClosed, "locked": prClosed}

// glabRecomputing is the detailed_merge_status GitLab reports while it is still
// working mergeability out, which the window polls through as UNKNOWN.
var glabRecomputing = map[string]bool{"checking": true, "unchecked": true, "preparing": true, "approvals_syncing": true}

// glabMergeStates maps detailed_merge_status onto GitHub's mergeStateStatus.
// Every status left out blocks the merge (not_approved, ci_must_pass,
// ci_still_running, discussions_not_resolved, ...) and reads BLOCKED.
var glabMergeStates = map[string]string{
	glabMergeableStatus: "CLEAN",
	"conflict":          "DIRTY",
	"need_rebase":       "BEHIND",
	"draft_status":      "DRAFT",
	"checking":          prUnknown,
	"unchecked":         prUnknown,
	"preparing":         prUnknown,
	"approvals_syncing": prUnknown,
}

func glabMergeable(mr glabMR) string {
	switch {
	case mr.HasConflicts:
		return prConflicting
	case glabRecomputing[mr.DetailedMergeStatus]:
		return prUnknown
	}
	return prMergeable
}

func glabMergeState(mr glabMR) string {
	if mr.HasConflicts {
		return glabMergeStates["conflict"]
	}
	return cmp.Or(glabMergeStates[mr.DetailedMergeStatus], prBlocked)
}

func (mr glabMR) pullRequest() *PullRequest {
	return &PullRequest{Number: mr.IID, URL: mr.WebURL, Title: mr.Title, State: glabStates[mr.State],
		HeadRefName: mr.SourceBranch, BaseRefName: mr.TargetBranch}
}

func (mr glabMR) crossRepo() bool { return mr.SourceProjectID != mr.TargetProjectID }

// branchMR is the merge request of the branch checked out at path: the open
// one, else with all the newest in any state. errNoPullRequest for a detached
// HEAD or a branch with none. Listed rather than `glab mr view`, which finds
// only an open one and refuses to pick between two.
//
// GitLab filters by branch name alone, so every fork's merge request from a
// branch called main comes back for a checkout of main (measured on
// gitlab-org/cli). A fork's one counts only when this checkout holds its head,
// and never when it merges a branch into its namesake: a fork's main merged
// into main is in main's history, and is still someone else's.
func (g gitlabForge) branchMR(path string, all bool) (glabMR, error) {
	branch, _ := gitQuiet(path, "symbolic-ref", "--short", "HEAD")
	if branch = strings.TrimSpace(branch); branch == "" {
		return glabMR{}, errNoPullRequest
	}
	args := []string{"mr", "list", "--source-branch=" + branch, "--per-page", strconv.Itoa(glabBranchPage), "-F", "json"}
	if all {
		args = append(args, "--all")
	}
	out, err := g.run(prLookupTimeout, path, args...)
	if err != nil {
		return glabMR{}, err
	}
	var mrs []glabMR
	if err := json.Unmarshal(out, &mrs); err != nil {
		return glabMR{}, errUnreadableGitLab(err)
	}
	mrs = slices.DeleteFunc(mrs, func(mr glabMR) bool {
		return mr.crossRepo() && (mr.SourceBranch == mr.TargetBranch || !holdsCommit(path, mr.SHA))
	})
	if len(mrs) == 0 {
		return glabMR{}, errNoPullRequest
	}
	for _, mr := range mrs {
		if mr.State == glabOpened {
			return mr, nil
		}
	}
	return mrs[0], nil
}

func holdsCommit(path, sha string) bool {
	if sha == "" || strings.HasPrefix(sha, "-") {
		return false
	}
	_, ok := gitQuiet(path, "merge-base", "--is-ancestor", sha, "HEAD")
	return ok
}

// view reads merge request number, or the branch's open one for zero.
func (g gitlabForge) view(path string, number int) (glabMR, error) {
	if number <= 0 {
		mr, err := g.branchMR(path, false)
		if err != nil {
			return glabMR{}, err
		}
		number = mr.IID
	}
	out, err := g.run(prReadTimeout, path, "mr", "view", strconv.Itoa(number), "-F", "json")
	if err != nil {
		return glabMR{}, err
	}
	var mr glabMR
	if err := json.Unmarshal(out, &mr); err != nil {
		return glabMR{}, errUnreadableGitLab(err)
	}
	if mr.IID == 0 {
		return glabMR{}, errNoPullRequest
	}
	return mr, nil
}

// iid is number, or the branch's open merge request for zero.
func (g gitlabForge) iid(path string, number int) (int, error) {
	if number > 0 {
		return number, nil
	}
	mr, err := g.branchMR(path, false)
	return mr.IID, err
}

func (g gitlabForge) branchPullRequest(path string) (*PullRequest, error) {
	mr, err := g.branchMR(path, true)
	if err != nil {
		return nil, err
	}
	return mr.pullRequest(), nil
}

// watchRead fills no Review: a GitLab review has no id the watch could tell a
// new one by.
func (g gitlabForge) watchRead(path string, read *api.PRRead) error {
	mr, err := g.view(path, read.Number)
	if err != nil {
		return err
	}
	read.Head = mr.SHA
	read.Checks, _, err = g.checks(path, mr.HeadPipeline)
	return err
}

func (g gitlabForge) pullRequestDetail(path string, number int, openOnly bool) (*PRDetail, error) {
	mr, err := g.view(path, number)
	if err != nil || (openOnly && mr.State != glabOpened) {
		return nil, err
	}
	changed, _ := strconv.Atoi(strings.TrimSuffix(mr.ChangesCount, "+"))
	detail := &PRDetail{
		Number:              mr.IID,
		URL:                 mr.WebURL,
		Title:               mr.Title,
		Body:                mr.Description,
		Author:              firstNonEmpty(mr.Author.Username, mr.Author.Name),
		AuthorLogin:         mr.Author.Username,
		State:               glabStates[mr.State],
		IsDraft:             mr.Draft,
		Mergeable:           glabMergeable(mr),
		MergeStateStatus:    glabMergeState(mr),
		BaseRefName:         mr.TargetBranch,
		HeadRefName:         mr.SourceBranch,
		ChangedFiles:        changed,
		IsCrossRepository:   mr.crossRepo(),
		MaintainerCanModify: mr.AllowCollaboration,
	}
	if err := g.fillDetail(path, mr, detail); err != nil {
		return nil, err
	}
	return detail, nil
}

// fillDetail reads what the merge request object leaves out, one glab call
// each, side by side: glab takes a few hundred milliseconds just to start.
func (g gitlabForge) fillDetail(path string, mr glabMR, d *PRDetail) error {
	var wg sync.WaitGroup
	var commitsErr, checksErr, diffErr error
	wg.Go(func() { d.Commits, commitsErr = g.commits(path, mr) })
	wg.Go(func() { d.Checks, d.CheckRuns, checksErr = g.checks(path, mr.HeadPipeline) })
	wg.Go(func() { d.ReviewDecision = g.reviewDecision(path, mr) })
	wg.Go(func() {
		var diff string
		if diff, diffErr = g.pullRequestDiff(path, mr.IID); diffErr == nil {
			d.Additions, d.Deletions = diffSize(diff)
		}
	})
	wg.Wait()
	return cmp.Or(commitsErr, checksErr, diffErr)
}

// diffSize counts the added and removed lines of a unified diff, inside hunks
// only: outside one, "--- a/x" is a header, inside it a removed "-- x" line.
func diffSize(diff string) (added, deleted int) {
	inHunk := false
	for line := range strings.SplitSeq(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			inHunk = false
		case strings.HasPrefix(line, "@@"):
			inHunk = true
		case inHunk && strings.HasPrefix(line, "+"):
			added++
		case inHunk && strings.HasPrefix(line, "-"):
			deleted++
		}
	}
	return added, deleted
}

type glabCommit struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Message       string `json:"message"`
	AuthorName    string `json:"author_name"`
	AuthorEmail   string `json:"author_email"`
	CommittedDate string `json:"committed_date"`
}

// commits lists the merge request's commits oldest first, as gh does; GitLab
// answers newest first. Login stays "": GitLab does not say which account an
// email belongs to.
// The first 100 only: one page is one call.
func (g gitlabForge) commits(path string, mr glabMR) ([]PRCommit, error) {
	out, err := g.run(prReadTimeout, path, "api",
		fmt.Sprintf("projects/%d/merge_requests/%d/commits?per_page=100", mr.ProjectID, mr.IID))
	if err != nil {
		return nil, err
	}
	var commits []glabCommit
	if err := json.Unmarshal(out, &commits); err != nil {
		return nil, errUnreadableGitLab(err)
	}
	if len(commits) == 0 {
		return nil, nil
	}
	list := make([]PRCommit, len(commits))
	for i, c := range commits {
		_, body, _ := strings.Cut(c.Message, "\n")
		list[len(commits)-1-i] = PRCommit{OID: c.ID, Headline: c.Title, Body: strings.TrimSpace(body),
			Name: c.AuthorName, Email: c.AuthorEmail, Date: c.CommittedDate}
	}
	return list, nil
}

type glabApprovals struct {
	Approved      bool              `json:"approved"`
	ApprovalsLeft int               `json:"approvals_left"`
	ApprovedBy    []json.RawMessage `json:"approved_by"`
}

// reviewDecision reads GitLab's approvals as gh's reviewDecision. A failed
// read answers "", no requirement known, rather than failing the whole detail
// over its least load-bearing field.
func (g gitlabForge) reviewDecision(path string, mr glabMR) string {
	out, err := g.run(prReadTimeout, path, "api",
		fmt.Sprintf("projects/%d/merge_requests/%d/approvals", mr.ProjectID, mr.IID))
	if err != nil {
		return ""
	}
	var a glabApprovals
	if err := json.Unmarshal(out, &a); err != nil {
		slog.Warn("glab approvals could not be decoded", "err", err)
		return ""
	}
	return approvalDecision(a)
}

// approvalDecision wants an approver before APPROVED: approved alone may be
// GitLab's answer for a merge request no rule asks anything of.
func approvalDecision(a glabApprovals) string {
	switch {
	case a.ApprovalsLeft > 0:
		return prReviewRequired
	case a.Approved && len(a.ApprovedBy) > 0:
		return prApproved
	}
	return ""
}

// glabListStates is lich's list state as glab's flag; open is glab's default.
var glabListStates = map[string][]string{"open": nil, "closed": {"--closed"}, "merged": {"--merged"}, "all": {"--all"}}

// listPullRequests leaves Checks zero: a listed merge request carries no
// pipeline (measured, glab 1.131), and a call per row is not a list.
func (g gitlabForge) listPullRequests(path, state string) ([]PRSummary, error) {
	args := append([]string{"mr", "list", "--per-page", strconv.Itoa(prListLimit), "-F", "json"}, glabListStates[state]...)
	out, err := g.run(prReadTimeout, path, args...)
	if err != nil {
		return nil, err
	}
	var mrs []glabMR
	if err := json.Unmarshal(out, &mrs); err != nil {
		return nil, errUnreadableGitLab(err)
	}
	if len(mrs) == 0 {
		return nil, nil
	}
	list := make([]PRSummary, 0, len(mrs))
	for _, mr := range mrs {
		row := PRSummary{Number: mr.IID, Title: mr.Title, Author: firstNonEmpty(mr.Author.Username, mr.Author.Name),
			State: glabStates[mr.State], IsDraft: mr.Draft, HeadRefName: mr.SourceBranch,
			IsCrossRepository: mr.crossRepo(), UpdatedAt: mr.UpdatedAt}
		if mr.DetailedMergeStatus == "not_approved" {
			row.ReviewDecision = prReviewRequired
		}
		list = append(list, row)
	}
	return list, nil
}

// pullRequestDiff reads the raw diff: without --raw glab asks GitLab's
// versions endpoint, which wants a token even on a public project (measured).
func (g gitlabForge) pullRequestDiff(path string, number int) (string, error) {
	iid, err := g.iid(path, number)
	if err != nil {
		return "", err
	}
	out, err := g.run(prReadTimeout, path, "mr", "diff", strconv.Itoa(iid), "--raw")
	return string(out), err
}

// createPullRequest opens GitLab's new merge request page. --fill pushes the
// branch and seeds the page from its commits; --web skips every prompt, and
// glab refuses --yes beside it.
func (g gitlabForge) createPullRequest(path string) error {
	_, err := g.run(prCreateTimeout, path, "mr", "create", "--fill", "--web")
	return err
}

// glabMergeMethods maps each merge method onto glab's flag; a plain merge has
// none.
var glabMergeMethods = map[string][]string{"squash": {"--squash"}, "merge": nil, "rebase": {"--rebase"}}

var errAdminMergeGitLab = errors.New("GitLab has no administrator merge past the rules of a branch. " +
	"Merge it on GitLab, or meet the rules first.")

func (g gitlabForge) mergePullRequest(path string, opts api.MergePullRequestOptions) error {
	flags, ok := glabMergeMethods[opts.Method]
	if !ok {
		return fmt.Errorf("unknown merge method %q", opts.Method)
	}
	if opts.Admin {
		return errAdminMergeGitLab
	}
	iid, err := g.iid(path, opts.Number)
	if err != nil {
		return err
	}
	_, err = g.run(prMergeTimeout, path, glabMergeArgs(iid, flags, opts)...)
	return err
}

// glabMergeArgs builds the glab merge call. --auto-merge defaults to true in
// glab 1.131, which on a merge request with a pipeline sets it to merge once the
// pipeline passes instead of merging now. Never -d: lich's worktree removal
// owns the branch. A rebase replays the branch's commits and takes no message.
func glabMergeArgs(iid int, flags []string, opts api.MergePullRequestOptions) []string {
	args := append([]string{"mr", "merge", strconv.Itoa(iid), "--yes", "--auto-merge=false"}, flags...)
	message := opts.Subject
	if opts.Body != "" {
		message += "\n\n" + opts.Body
	}
	switch {
	case opts.Subject == "":
	case opts.Method == "squash":
		args = append(args, "--squash-message", message)
	case opts.Method == "merge":
		args = append(args, "--message", message)
	}
	return args
}

func (g gitlabForge) pullRequestHead(path string, number int) (prHead, error) {
	mr, err := g.view(path, number)
	if err != nil {
		return prHead{}, err
	}
	if mr.SourceBranch == "" {
		return prHead{}, fmt.Errorf("GitLab reports no source branch for merge request !%d.", number)
	}
	return prHead{RefName: mr.SourceBranch, CrossRepo: mr.crossRepo(), CanModify: mr.AllowCollaboration}, nil
}

func (g gitlabForge) checkoutPullRequest(_, wtPath string, number int) error {
	_, err := g.run(prCheckoutTimeout, wtPath, "mr", "checkout", strconv.Itoa(number))
	return err
}
