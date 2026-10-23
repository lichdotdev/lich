package project

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lichdotdev/lich/api"
)

// gh network calls are capped so a slow forge or a hung auth prompt never
// wedges the pull request screen. The reads (view, diff) share one budget;
// merge and create get more room because they push and mutate on the remote.
const (
	prReadTimeout   = 8 * time.Second
	prMergeTimeout  = 30 * time.Second
	prCreateTimeout = 20 * time.Second
	prReviewTimeout = 20 * time.Second
	// A checkout fetches the head ref, so it is bounded by the size of the
	// branch rather than by a round-trip.
	prCheckoutTimeout = 90 * time.Second
)

// ghRunner runs one gh subcommand inside dir as the account token (empty: gh's
// own active account) and returns its stdout. It is a field on Service so the
// pull request flows can be exercised without a GitHub remote; production
// wiring is runGH.
type ghRunner func(timeout time.Duration, dir, token string, args ...string) ([]byte, error)

// errNoPullRequest marks gh's "no pull requests found" — the one failure that
// means the branch simply has no PR, which the read flows turn into an empty
// panel instead of an error.
var errNoPullRequest = errors.New("no pull request for this branch")

// runGH shells out to gh with the call capped by timeout, and turns a failure
// into the message the screen shows (gherror.go); gh's own stderr reaches the
// log, never the screen. A non-empty token rides in GH_TOKEN, which is how gh is
// told to answer as one specific account.
func runGH(timeout time.Duration, dir, token string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := commandContext(ctx, "gh", args...)
	cmd.Dir = dir
	if token != "" {
		cmd.Env = append(cmd.Env, "GH_TOKEN="+token)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if isNoPullRequest(stderr.String()) {
			return nil, errNoPullRequest
		}
		return nil, ghFailure(args, stderr.String(), err, ctx.Err())
	}
	return out, nil
}

// prViewFields is the gh `pr view --json` selection backing the Pulls panel.
const prViewFields = "number,url,state,title,body,author,isDraft,mergeable,mergeStateStatus,reviewDecision,reviewRequests,latestReviews,baseRefName,headRefName,isCrossRepository,maintainerCanModify,statusCheckRollup,changedFiles,commits"

// prSelector renders the pull request argument gh's subcommands take ahead of
// their flags. Zero means "the branch checked out at path" — gh's own default,
// and the only mode there was before the repository-wide list. The selector is
// an int rather than a string on purpose: a number can never reach gh's
// selector slot as a "-"-prefixed value that would parse as a flag, nor as a
// branch name the caller never meant to address.
func prSelector(number int) []string {
	if number <= 0 {
		return nil
	}
	return []string{strconv.Itoa(number)}
}

// prArgs builds "pr <verb> [<number>] <rest...>" — the shape every gh pull
// request subcommand takes, with the selector always ahead of the flags.
func prArgs(verb string, number int, rest ...string) []string {
	args := append([]string{"pr", verb}, prSelector(number)...)
	return append(args, rest...)
}

type PRDetail = api.PRDetail

type PRCommit = api.PRCommit

// ghPRView mirrors the requested gh payload; statusCheckRollup is reduced to
// ChecksRollup inside parsePRDetail and never leaves this package raw.
type ghPRView struct {
	Number              int               `json:"number"`
	URL                 string            `json:"url"`
	State               string            `json:"state"`
	Title               string            `json:"title"`
	Body                string            `json:"body"`
	Author              ghPRAuthor        `json:"author"`
	IsDraft             bool              `json:"isDraft"`
	Mergeable           string            `json:"mergeable"`
	MergeStateStatus    string            `json:"mergeStateStatus"`
	ReviewDecision      string            `json:"reviewDecision"`
	ReviewRequests      []ghReviewRequest `json:"reviewRequests"`
	LatestReviews       []ghLatestReview  `json:"latestReviews"`
	BaseRefName         string            `json:"baseRefName"`
	HeadRefName         string            `json:"headRefName"`
	ChangedFiles        int               `json:"changedFiles"`
	IsCrossRepository   bool              `json:"isCrossRepository"`
	MaintainerCanModify bool              `json:"maintainerCanModify"`
	StatusCheckRollup   []checkItem       `json:"statusCheckRollup"`
	Commits             []ghCommit        `json:"commits"`
}

// ghCommit is one entry of gh's commits array. Authors is a list because of
// co-authored commits; the first one is the one the list names.
type ghCommit struct {
	OID             string           `json:"oid"`
	MessageHeadline string           `json:"messageHeadline"`
	MessageBody     string           `json:"messageBody"`
	CommittedDate   string           `json:"committedDate"`
	Authors         []ghCommitAuthor `json:"authors"`
}

type ghCommitAuthor struct {
	Login string `json:"login"`
	Name  string `json:"name"`
	// Email is the address git recorded and GitHub resolved (or failed to). It
	// is carried for the unresolved case alone: "this commit belongs to nobody"
	// is only actionable next to the address that has to be added or corrected.
	Email string `json:"email"`
}

// PullRequestDetail returns one open pull request in full: the given number, or
// the PR of the branch checked out at opts.Path when opts.Number is zero. It yields nil
// when there is no open PR — gh reports none, or the PR is merged/closed, which
// the OPEN-only gate treats the same as the badge does. A real failure (gh
// missing, not a GitHub repo) yields an error so the panel can tell "no PR"
// apart from "could not look up".
func (s *Service) PullRequestDetail(_ context.Context, opts api.NumberOptions) (*PRDetail, error) {
	out, err := s.gh(prReadTimeout, opts.Path, prArgs("view", opts.Number, "--json", prViewFields)...)
	if errors.Is(err, errNoPullRequest) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// The OPEN-only gate belongs to the branch lookup alone. Naming a number is
	// asking for that pull request, whatever became of it — the list can show
	// merged and closed ones, and a row that cannot be opened is not a row.
	detail, err := parsePRDetail(out, opts.Number == 0)
	if err != nil || detail == nil || len(detail.CheckRuns) == 0 {
		return detail, err
	}
	if err := s.markRequiredChecks(opts.Path, detail.Number, detail.CheckRuns); err != nil {
		return nil, err
	}
	return detail, nil
}

// parsePRDetail decodes gh's JSON and reduces the check rollup. With openOnly it
// returns nil for a non-OPEN PR — gh still reports a merged/closed branch PR,
// but the badge wants only an actionable one — matching parsePullRequest's
// contract.
func parsePRDetail(out []byte, openOnly bool) (*PRDetail, error) {
	var v ghPRView
	if err := json.Unmarshal(out, &v); err != nil {
		return nil, errUnreadableAnswer(err)
	}
	if v.Number == 0 || (openOnly && v.State != "OPEN") {
		return nil, nil
	}
	return &PRDetail{
		Number:              v.Number,
		URL:                 v.URL,
		Title:               v.Title,
		Body:                v.Body,
		Author:              firstNonEmpty(v.Author.Login, v.Author.Name),
		AuthorLogin:         v.Author.Login,
		State:               v.State,
		IsDraft:             v.IsDraft,
		Mergeable:           v.Mergeable,
		MergeStateStatus:    v.MergeStateStatus,
		ReviewDecision:      v.ReviewDecision,
		Reviewers:           toReviewers(v.LatestReviews, v.ReviewRequests),
		BaseRefName:         v.BaseRefName,
		HeadRefName:         v.HeadRefName,
		ChangedFiles:        v.ChangedFiles,
		IsCrossRepository:   v.IsCrossRepository,
		MaintainerCanModify: v.MaintainerCanModify,
		Checks:              reduceChecks(v.StatusCheckRollup),
		CheckRuns:           toCheckItems(v.StatusCheckRollup),
		Commits:             toCommits(v.Commits),
	}, nil
}

// toCommits flattens gh's commits into the list the Commits tab renders, in
// gh's own order — oldest first, the order the branch built them. Returns nil
// when gh reports none, so the tab can tell "no commits" from a list.
func toCommits(commits []ghCommit) []PRCommit {
	if len(commits) == 0 {
		return nil
	}
	out := make([]PRCommit, 0, len(commits))
	for _, c := range commits {
		// Never collapsed to one display string: both kinds of commit carry a
		// name, so falling back to it makes an unattributed commit read exactly
		// like an attributed one.
		var author ghCommitAuthor
		if len(c.Authors) > 0 {
			author = c.Authors[0]
		}
		out = append(out, PRCommit{
			OID:      c.OID,
			Headline: c.MessageHeadline,
			Body:     c.MessageBody,
			Login:    author.Login,
			Name:     author.Name,
			Email:    author.Email,
			Date:     c.CommittedDate,
		})
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// mergeMethods maps each accepted method to its gh flag, and doubles as the
// allow-list: an unknown method is rejected before any shell-out.
var mergeMethods = map[string]string{
	"squash": "--squash",
	"merge":  "--merge",
	"rebase": "--rebase",
}

// MergePullRequest merges a pull request on GitHub with the given method
// (squash|merge|rebase) — the given number, or the PR of the branch checked out
// at opts.Path when opts.Number is zero. A non-empty opts.Subject overrides the commit message
// (the dock's "edit message" flow); an empty opts.Subject leaves gh's default, the
// quick merge. opts.Admin merges with administrator privileges, GitHub's own bypass
// of the rules on the base branch. It does not pass --delete-branch: lich's
// worktree removal owns that cleanup, and deleting a checked-out worktree's
// branch is trouble. gh's stderr is surfaced on failure (not mergeable, branch
// protection, gh missing).
func (s *Service) MergePullRequest(_ context.Context, opts api.MergePullRequestOptions) (api.None, error) {
	args, err := mergeArgs(opts.Number, opts.Method, opts.Subject, opts.Body, opts.Admin)
	if err != nil {
		return api.None{}, err
	}
	return api.None{}, s.writePR(prMergeTimeout, opts.Path, opts.Number, args...)
}

// mergeArgs builds the gh argument list for a merge, rejecting an unknown method
// before any shell-out. The subject override applies only to squash and merge
// commits — rebase replays the branch's own commits, so gh rejects a message
// there — and an empty subject means gh's default message. body rides along with
// a subject and may itself be empty.
//
// --admin is what makes gh call GitHub at all on a governed branch: without it
// gh refuses from the client the moment mergeStateStatus is BLOCKED or BEHIND,
// so no rule violation ever reaches the screen. Whether this account may
// actually bypass is GitHub's answer to give, not this one's to guess.
func mergeArgs(number int, method, subject, body string, admin bool) ([]string, error) {
	flag, ok := mergeMethods[method]
	if !ok {
		return nil, fmt.Errorf("unknown merge method %q", method)
	}
	args := prArgs("merge", number, flag)
	if subject != "" && method != "rebase" {
		args = append(args, "--subject", subject, "--body", body)
	}
	if admin {
		args = append(args, "--admin")
	}
	return args, nil
}

// CreatePullRequest opens GitHub's "new pull request" page in the browser for
// opts.Path's branch (gh pushes the branch first when it has no upstream).
// Deliberately the web flow, not an in-app form — GitHub's page already owns the
// title, body, reviewers and template. Returns once the browser is launched.
func (s *Service) CreatePullRequest(_ context.Context, opts api.PathOptions) (api.None, error) {
	if _, err := s.gh(prCreateTimeout, opts.Path, "pr", "create", "--web"); err != nil {
		return api.None{}, err
	}
	return api.None{}, nil
}

// PullRequestDiff returns the unified diff of opts.Path's branch pull request —
// every change the PR would merge into its base, as GitHub computes it — for the
// Pulls screen's "Files changed" tab. --color never keeps the output plain so
// the window's parsePatchFiles reads it. An empty string with no error means the
// branch has no open PR (nothing to show).
//
// Ceiling: the whole diff is buffered here and shipped as one JSON string, so a
// monster PR costs its own size in memory on both sides. Reviewing a diff that
// large in a panel is not the workflow; stream it (or cap and mark it
// truncated) if that ever stops being true.
func (s *Service) PullRequestDiff(_ context.Context, opts api.NumberOptions) (string, error) {
	out, err := s.gh(prReadTimeout, opts.Path, prArgs("diff", opts.Number, "--color", "never")...)
	if errors.Is(err, errNoPullRequest) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return displayDiff(string(out)), nil
}

// prListFields is the gh `pr list --json` selection backing the Pulls list
// column, and prListLimit caps how many it asks for. gh's own default is 30;
// the ceiling is here so the list stays one bounded call. gh answers with the
// most recently updated, and the column marks a full page so the cap is never
// silent — a filter typed over a truncated list would otherwise read as the
// repository's whole answer.
const (
	prListFields = "number,title,author,state,isDraft,reviewDecision,headRefName,isCrossRepository,updatedAt,statusCheckRollup"
	prListLimit  = 50
)

// prListStates allow-lists the values gh's --state takes, so a state sent by
// the window can never widen into an arbitrary flag.
var prListStates = map[string]bool{"open": true, "closed": true, "merged": true, "all": true}

type PRSummary = api.PRSummary

// ghPRListItem mirrors one gh `pr list --json` entry; the rollup is reduced to
// ChecksRollup and the author object flattened before either leaves this package.
type ghPRListItem struct {
	Number            int         `json:"number"`
	Title             string      `json:"title"`
	Author            ghPRAuthor  `json:"author"`
	State             string      `json:"state"`
	IsDraft           bool        `json:"isDraft"`
	ReviewDecision    string      `json:"reviewDecision"`
	HeadRefName       string      `json:"headRefName"`
	IsCrossRepository bool        `json:"isCrossRepository"`
	UpdatedAt         string      `json:"updatedAt"`
	StatusCheckRollup []checkItem `json:"statusCheckRollup"`
}

type ghPRAuthor struct {
	Login string `json:"login"`
	Name  string `json:"name"`
}

// ListPullRequests returns the repository's pull requests in the given state
// (open|closed|merged|all, defaulting to open), newest update first — gh's own
// order, which the window re-sorts. It returns nil for a repository with none;
// a real failure (gh missing, unauthenticated, not a GitHub repo) yields an
// error so the column can say why it is empty.
func (s *Service) ListPullRequests(_ context.Context, opts api.ListPullRequestsOptions) ([]PRSummary, error) {
	if opts.State == "" {
		opts.State = "open"
	}
	if !prListStates[opts.State] {
		return nil, fmt.Errorf("unknown pull request state %q", opts.State)
	}
	// No errNoPullRequest branch here: asked for --json, gh answers a repository
	// with no matching pull request as an empty array and exits 0. Its "no pull
	// requests found" message belongs to view and diff, which address one.
	out, err := s.gh(prReadTimeout, opts.Path,
		"pr", "list", "--state", opts.State, "--limit", strconv.Itoa(prListLimit), "--json", prListFields)
	if err != nil {
		return nil, err
	}
	list, err := parsePRList(out)
	if err != nil {
		return nil, err
	}
	if err := s.tallyRequestedThreads(opts.Path, list); err != nil {
		return nil, err
	}
	return list, nil
}

// parsePRList decodes gh's list payload into the column's rows.
func parsePRList(out []byte) ([]PRSummary, error) {
	var items []ghPRListItem
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, errUnreadableAnswer(err)
	}
	if len(items) == 0 {
		return nil, nil
	}
	list := make([]PRSummary, 0, len(items))
	for _, it := range items {
		list = append(list, PRSummary{
			Number:            it.Number,
			Title:             it.Title,
			Author:            firstNonEmpty(it.Author.Login, it.Author.Name),
			State:             it.State,
			IsDraft:           it.IsDraft,
			ReviewDecision:    it.ReviewDecision,
			HeadRefName:       it.HeadRefName,
			IsCrossRepository: it.IsCrossRepository,
			UpdatedAt:         it.UpdatedAt,
			Checks:            reduceChecks(it.StatusCheckRollup),
		})
	}
	return list, nil
}

// isNoPullRequest recognises gh's "no PR for this branch" message — the one
// failure that means an empty panel rather than a real error.
func isNoPullRequest(stderr string) bool {
	return strings.Contains(strings.ToLower(stderr), "no pull requests found")
}
