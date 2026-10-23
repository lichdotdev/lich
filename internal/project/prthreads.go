package project

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lichdotdev/lich/api"
)

// A pull request's conversation is the one part of it gh's own `pr view` cannot
// answer: --json reviews carries a review's body but never the threads hanging
// off its lines, and no REST call reports whether a thread was resolved. The
// GraphQL API has all three in one round-trip, so that is what this asks for.
//
// The account resolution is the same as every other gh call (ghaccount.go): the
// query runs inside the checkout, and gh substitutes {owner}/{repo} from its
// remote.
//
// Ceiling: every collection is capped and unpaged — the last 50 reviews, the
// last 50 pull request comments, the last 100 threads and the last 50 comments
// inside each thread. `last` throughout, because a conversation is read at its
// end: what a screen has to show is the newest reply, and the tail past the cap
// is on github.com until somebody hits it.
const prConversationQuery = `query($owner:String!,$repo:String!,$number:Int!){
  repository(owner:$owner,name:$repo){
    pullRequest(number:$number){
      headRefOid
      reviews(last:50){nodes{id databaseId author{login} state body submittedAt viewerCanUpdate}}
      comments(last:50){nodes{databaseId author{login} body createdAt viewerCanUpdate viewerCanDelete}}
      reviewThreads(last:100){nodes{
        id isResolved isOutdated path line originalLine startLine originalStartLine diffSide
        comments(last:50){nodes{databaseId author{login} body createdAt diffHunk viewerCanUpdate viewerCanDelete}}
      }}
    }
  }
}`

type PRConversation = api.PRConversation

type PRReview = api.PRReview

type PRComment = api.PRComment

type PRReviewThread = api.PRReviewThread

type PRThreadComment = api.PRThreadComment

// ghConversation mirrors the GraphQL payload; nothing below leaves this package
// in this shape.
type ghConversation struct {
	Data struct {
		Repository struct {
			PullRequest struct {
				HeadRefOID string `json:"headRefOid"`
				Reviews    struct {
					Nodes []ghReview `json:"nodes"`
				} `json:"reviews"`
				Comments struct {
					Nodes []ghIssueComment `json:"nodes"`
				} `json:"comments"`
				ReviewThreads struct {
					Nodes []ghReviewThread `json:"nodes"`
				} `json:"reviewThreads"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
}

// ghActor is GitHub's author object, which is null for an account that has been
// deleted since — the zero value then reads as an empty login.
type ghActor struct {
	Login string `json:"login"`
}

type ghReview struct {
	ID              string  `json:"id"`
	DatabaseID      int64   `json:"databaseId"`
	Author          ghActor `json:"author"`
	State           string  `json:"state"`
	Body            string  `json:"body"`
	SubmittedAt     string  `json:"submittedAt"`
	ViewerCanUpdate bool    `json:"viewerCanUpdate"`
}

type ghIssueComment struct {
	DatabaseID      int64   `json:"databaseId"`
	Author          ghActor `json:"author"`
	Body            string  `json:"body"`
	CreatedAt       string  `json:"createdAt"`
	ViewerCanUpdate bool    `json:"viewerCanUpdate"`
	ViewerCanDelete bool    `json:"viewerCanDelete"`
}

type ghReviewThread struct {
	ID                string `json:"id"`
	IsResolved        bool   `json:"isResolved"`
	IsOutdated        bool   `json:"isOutdated"`
	Path              string `json:"path"`
	Line              *int   `json:"line"`
	OriginalLine      *int   `json:"originalLine"`
	StartLine         *int   `json:"startLine"`
	OriginalStartLine *int   `json:"originalStartLine"`
	DiffSide          string `json:"diffSide"`
	Comments          struct {
		Nodes []ghThreadComment `json:"nodes"`
	} `json:"comments"`
}

type ghThreadComment struct {
	DatabaseID int64   `json:"databaseId"`
	Author     ghActor `json:"author"`
	Body       string  `json:"body"`
	CreatedAt  string  `json:"createdAt"`
	DiffHunk   string  `json:"diffHunk"`
	// ViewerCanUpdate answers for the account gh ran as.
	ViewerCanUpdate bool `json:"viewerCanUpdate"`
	ViewerCanDelete bool `json:"viewerCanDelete"`
}

// PullRequestConversation reads one pull request's reviews, comments and review
// threads. opts.Number is required: the query addresses a pull request by number,
// and the screen always knows it by the time there is a conversation to show —
// resolving "the branch's PR" here would cost a second gh round-trip on every
// read.
func (s *Service) PullRequestConversation(_ context.Context, opts api.NumberOptions) (*PRConversation, error) {
	if opts.Number <= 0 {
		return nil, fmt.Errorf("a pull request number is required")
	}
	out, err := s.gh(prReadTimeout, opts.Path, "api", "graphql",
		"-F", "owner={owner}", "-F", "repo={repo}",
		"-F", fmt.Sprintf("number=%d", opts.Number),
		"-f", "query="+prConversationQuery)
	if err != nil {
		return nil, err
	}
	return parseConversation(out)
}

// parseConversation reduces the GraphQL payload to what the screen renders.
func parseConversation(out []byte) (*PRConversation, error) {
	var payload ghConversation
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, errUnreadableAnswer(err)
	}
	pr := payload.Data.Repository.PullRequest
	return &PRConversation{
		HeadRefOID: pr.HeadRefOID,
		Reviews:    toReviews(pr.Reviews.Nodes),
		Comments:   toComments(pr.Comments.Nodes),
		Threads:    toThreads(pr.ReviewThreads.Nodes),
	}, nil
}

// toReviews keeps the reviews that say something. A review with no body and no
// verdict is the envelope GitHub wraps line comments in — its threads are
// rendered on their own lines, and the empty shell would read as a person who
// reviewed and wrote nothing. A pending review is one nobody has submitted yet.
func toReviews(nodes []ghReview) []PRReview {
	out := make([]PRReview, 0, len(nodes))
	for _, r := range nodes {
		if r.State == "PENDING" || (r.Body == "" && r.State == "COMMENTED") {
			continue
		}
		out = append(out, PRReview{
			ID:         r.ID,
			DatabaseID: r.DatabaseID,
			Author:     r.Author.Login,
			State:      r.State,
			Body:       r.Body,
			Date:       r.SubmittedAt,
			CanEdit:    r.ViewerCanUpdate,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func toComments(nodes []ghIssueComment) []PRComment {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]PRComment, 0, len(nodes))
	for _, c := range nodes {
		out = append(out, PRComment{
			ID:        c.DatabaseID,
			Author:    c.Author.Login,
			Body:      c.Body,
			Date:      c.CreatedAt,
			CanEdit:   c.ViewerCanUpdate,
			CanDelete: c.ViewerCanDelete,
		})
	}
	return out
}

func toThreads(nodes []ghReviewThread) []PRReviewThread {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]PRReviewThread, 0, len(nodes))
	for _, t := range nodes {
		out = append(out, PRReviewThread{
			ID:         t.ID,
			Path:       t.Path,
			Line:       threadLine(t),
			StartLine:  threadStartLine(t),
			Side:       t.DiffSide,
			IsResolved: t.IsResolved,
			IsOutdated: t.IsOutdated,
			Comments:   toThreadComments(t.Comments.Nodes),
		})
	}
	return out
}

// threadLine is where the thread hangs. GitHub nulls `line` for a thread whose
// lines the branch has since rewritten and keeps the original — a number that no
// longer addresses anything in the current diff, which is exactly why such a
// thread is marked outdated and rendered away from the file.
func threadLine(t ghReviewThread) int {
	if t.Line != nil {
		return *t.Line
	}
	return deref(t.OriginalLine)
}

// threadStartLine is the same fallback for the range's first line: an outdated
// thread keeps the original, which is the number its diff hunk is written in.
func threadStartLine(t ghReviewThread) int {
	if t.StartLine != nil {
		return *t.StartLine
	}
	return deref(t.OriginalStartLine)
}

func deref(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func toThreadComments(nodes []ghThreadComment) []PRThreadComment {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]PRThreadComment, 0, len(nodes))
	for _, c := range nodes {
		out = append(out, PRThreadComment{
			ID:        c.DatabaseID,
			Author:    c.Author.Login,
			Body:      c.Body,
			Date:      c.CreatedAt,
			DiffHunk:  c.DiffHunk,
			CanEdit:   c.ViewerCanUpdate,
			CanDelete: c.ViewerCanDelete,
		})
	}
	return out
}
