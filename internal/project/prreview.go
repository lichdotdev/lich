package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/lichdotdev/lich/api"
)

// The three writes a review takes, and why each one is the API it is:
//
//   - a reply belongs to a thread, and REST addresses one by the database id of
//     the comment that opened it;
//   - resolving a thread has no REST endpoint at all, only the GraphQL mutation;
//   - a review lands as one call carrying every line comment at once, which is
//     what makes it a review rather than a pile of separate remarks;
//   - dismissing takes the review's node id, which only the conversation read
//     carries (prthreads.go); `gh pr view` never reports one.
const (
	resolveThreadMutation = `mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}`

	unresolveThreadMutation = `mutation($id:ID!){unresolveReviewThread(input:{threadId:$id}){thread{isResolved}}}`
)

// reviewEvents allow-lists what a submitted review can say, and doubles as the
// map onto GitHub's own wording: an unknown verdict is rejected before any
// shell-out.
var reviewEvents = map[string]string{
	"approve":         "APPROVE",
	"comment":         "COMMENT",
	"request_changes": "REQUEST_CHANGES",
}

// reviewSides allow-lists which side of the diff a comment may hang off, so a
// value sent by the window can never reach GitHub as something else.
var reviewSides = map[string]bool{"RIGHT": true, "LEFT": true}

type ReviewComment = api.ReviewComment

// ghReviewPayload is GitHub's REST body for a submitted review. Its field names
// are snake_case and its comment shape is not lich's, which is the whole reason
// this type exists next to ReviewComment.
type ghReviewPayload struct {
	CommitID string            `json:"commit_id,omitempty"`
	Body     string            `json:"body,omitempty"`
	Event    string            `json:"event"`
	Comments []ghReviewComment `json:"comments,omitempty"`
}

type ghReviewComment struct {
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Side      string `json:"side,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	StartSide string `json:"start_side,omitempty"`
	Body      string `json:"body"`
}

// SubmitReview files one review on GitHub: the verdict, the summary body, and
// every line comment the window has been holding, in a single call. opts.Head is the
// commit the comments were written against — empty lets GitHub assume its own
// latest, which is wrong the moment a push landed while the review was open.
//
// This is also the plain Approve: no comments, no body, opts.Event "approve". GitHub
// refuses an approval of one's own pull request, which the message table turns
// into a sentence (gherror.go).
func (s *Service) SubmitReview(_ context.Context, opts api.SubmitReviewOptions) (api.None, error) {
	payload, err := reviewPayload(opts.Event, opts.Body, opts.Head, opts.Comments)
	if err != nil {
		return api.None{}, err
	}
	file, err := writeTempJSON(payload)
	if err != nil {
		return api.None{}, err
	}
	defer os.Remove(file)
	return api.None{}, s.writePR(prReviewTimeout, opts.Path, opts.Number, "api", "--method", "POST",
		"repos/{owner}/{repo}/pulls/"+strconv.Itoa(opts.Number)+"/reviews", "--input", file)
}

// reviewPayload validates the review and renders GitHub's body for it. The
// checks are here rather than at the call site because every one of them is a
// round-trip GitHub would refuse anyway: an unknown verdict, a side that is
// neither of the two, or a review that says nothing at all — which GitHub
// rejects for anything but an approval.
func reviewPayload(event, body, head string, comments []ReviewComment) (*ghReviewPayload, error) {
	verdict, ok := reviewEvents[event]
	if !ok {
		return nil, fmt.Errorf("unknown review verdict %q", event)
	}
	if verdict != "APPROVE" && body == "" && len(comments) == 0 {
		return nil, fmt.Errorf("a review with no verdict of its own needs a comment or a body")
	}
	out := &ghReviewPayload{CommitID: head, Body: body, Event: verdict}
	for _, c := range comments {
		comment, err := toGHReviewComment(c)
		if err != nil {
			return nil, err
		}
		out.Comments = append(out.Comments, comment)
	}
	return out, nil
}

// toGHReviewComment renders one line comment. A start line equal to the line, or
// past it, is not a range — GitHub rejects such a pair, and a one-line selection
// arrives here as exactly that.
func toGHReviewComment(c ReviewComment) (ghReviewComment, error) {
	side := c.Side
	if side == "" {
		side = "RIGHT"
	}
	if !reviewSides[side] {
		return ghReviewComment{}, fmt.Errorf("unknown diff side %q", c.Side)
	}
	if c.Path == "" || c.Line <= 0 {
		return ghReviewComment{}, fmt.Errorf("a review comment needs a file and a line")
	}
	if c.Body == "" {
		return ghReviewComment{}, fmt.Errorf("a review comment needs something written in it")
	}
	out := ghReviewComment{Path: c.Path, Line: c.Line, Side: side, Body: c.Body}
	if c.StartLine > 0 && c.StartLine < c.Line {
		out.StartLine, out.StartSide = c.StartLine, side
	}
	return out, nil
}

// writeTempJSON hands gh a body through a file because the runner (pr.go) has no
// stdin: gh's own --field flags cannot express an array of objects, and widening
// every gh call site with a stdin argument to serve this one call is a worse
// trade than a file that is removed as soon as the call returns.
func writeTempJSON(payload any) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("could not build the review: %w", err)
	}
	file, err := os.CreateTemp("", "lich-review-*.json")
	if err != nil {
		return "", fmt.Errorf("could not stage the review: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(body); err != nil {
		os.Remove(file.Name())
		return "", fmt.Errorf("could not stage the review: %w", err)
	}
	return file.Name(), nil
}

// ReplyToReviewThread answers an existing thread. opts.CommentID is the database id
// of any comment in it — GitHub files the reply against the thread that comment
// belongs to. A reply posts immediately and is never part of the pending review:
// it is an answer to something already said, which is the same split GitHub
// draws between "add single comment" and "start a review".
func (s *Service) ReplyToReviewThread(_ context.Context, opts api.PRCommentBodyOptions) (api.None, error) {
	endpoint := fmt.Sprintf("repos/{owner}/{repo}/pulls/%d/comments/%d/replies", opts.Number, opts.CommentID)
	return api.None{}, s.writeBody(opts.Path, opts.Number, "POST", endpoint, opts.Body, "a reply needs something written in it")
}

// writeBody sends body to a REST endpoint that takes a text and nothing else,
// refusing an empty one with emptyErr before it reaches gh: GitHub would
// refuse it too, a round-trip later.
func (s *Service) writeBody(path string, number int, method, endpoint, body, emptyErr string) error {
	if body == "" {
		return errors.New(emptyErr)
	}
	return s.writePR(prReviewTimeout, path, number, "api", "--method", method, endpoint, "-f", "body="+body)
}

// EditReviewComment rewrites one comment of a review thread. opts.CommentID is its
// database id, the same address a reply takes; GitHub refuses the edit unless
// the account gh runs as may update that comment.
func (s *Service) EditReviewComment(_ context.Context, opts api.PRCommentBodyOptions) (api.None, error) {
	endpoint := fmt.Sprintf("repos/{owner}/{repo}/pulls/comments/%d", opts.CommentID)
	return api.None{}, s.writeBody(opts.Path, opts.Number, "PATCH", endpoint, opts.Body, "a review comment needs something written in it")
}

// DeleteReviewComment removes one comment of a review thread; the last one
// gone takes the thread with it. There is no undo on GitHub's side.
func (s *Service) DeleteReviewComment(_ context.Context, opts api.PRCommentOptions) (api.None, error) {
	endpoint := fmt.Sprintf("repos/{owner}/{repo}/pulls/comments/%d", opts.CommentID)
	return api.None{}, s.writePR(prReviewTimeout, opts.Path, opts.Number, "api", "--method", "DELETE", endpoint)
}

// EditReviewBody rewrites the summary of a submitted review. opts.ReviewID is its
// database id; the verdict stays what it was, GitHub has no way to change it.
func (s *Service) EditReviewBody(_ context.Context, opts api.PRReviewBodyOptions) (api.None, error) {
	endpoint := fmt.Sprintf("repos/{owner}/{repo}/pulls/%d/reviews/%d", opts.Number, opts.ReviewID)
	return api.None{}, s.writeBody(opts.Path, opts.Number, "PUT", endpoint, opts.Body, "a review summary needs something written in it")
}

// ResolveReviewThread marks a thread resolved, or reopens it. opts.ThreadID is the
// GraphQL node id the conversation read carries — the database id a reply uses
// does not address a thread here.
func (s *Service) ResolveReviewThread(_ context.Context, opts api.ResolveThreadOptions) (api.None, error) {
	if opts.ThreadID == "" {
		return api.None{}, fmt.Errorf("a thread is required")
	}
	mutation := unresolveThreadMutation
	if opts.Resolved {
		mutation = resolveThreadMutation
	}
	return api.None{}, s.writePR(prReviewTimeout, opts.Path, 0, "api", "graphql", "-f", "query="+mutation, "-f", "id="+opts.ThreadID)
}

// CommentOnPullRequest leaves a comment on the pull request itself — the
// conversation with no file and no line behind it.
func (s *Service) CommentOnPullRequest(_ context.Context, opts api.PRBodyOptions) (api.None, error) {
	if opts.Body == "" {
		return api.None{}, fmt.Errorf("a comment needs something written in it")
	}
	return api.None{}, s.writePR(prReviewTimeout, opts.Path, opts.Number, prArgs("comment", opts.Number, "--body", opts.Body)...)
}

// EditPullRequestComment rewrites a comment on the pull request itself.
// GitHub files those as issue comments, which is the endpoint they live at.
func (s *Service) EditPullRequestComment(_ context.Context, opts api.PRCommentBodyOptions) (api.None, error) {
	endpoint := fmt.Sprintf("repos/{owner}/{repo}/issues/comments/%d", opts.CommentID)
	return api.None{}, s.writeBody(opts.Path, opts.Number, "PATCH", endpoint, opts.Body, "a comment needs something written in it")
}

// DeletePullRequestComment removes a comment on the pull request itself, for
// good.
func (s *Service) DeletePullRequestComment(_ context.Context, opts api.PRCommentOptions) (api.None, error) {
	endpoint := fmt.Sprintf("repos/{owner}/{repo}/issues/comments/%d", opts.CommentID)
	return api.None{}, s.writePR(prReviewTimeout, opts.Path, opts.Number, "api", "--method", "DELETE", endpoint)
}

// EditPullRequestTitle renames the pull request on GitHub. GitHub refuses a pull
// request without a title, so an empty one never reaches gh.
func (s *Service) EditPullRequestTitle(_ context.Context, opts api.PRTitleOptions) (api.None, error) {
	if strings.TrimSpace(opts.Title) == "" {
		return api.None{}, fmt.Errorf("a pull request needs a title")
	}
	return api.None{}, s.writePR(prReviewTimeout, opts.Path, opts.Number, prArgs("edit", opts.Number, "--title", opts.Title)...)
}

// EditPullRequestBody rewrites the pull request's description on GitHub, and an
// empty body clears it. The title is left alone on purpose: sending the one the
// screen read would undo a rename made on GitHub since.
func (s *Service) EditPullRequestBody(_ context.Context, opts api.PRBodyOptions) (api.None, error) {
	return api.None{}, s.writePR(prReviewTimeout, opts.Path, opts.Number, prArgs("edit", opts.Number, "--body", opts.Body)...)
}
