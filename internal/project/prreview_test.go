package project

import (
	"encoding/json"
	"github.com/lichdotdev/lich/api"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestReviewPayload(t *testing.T) {
	t.Parallel()
	t.Run("a plain approval carries no body and no comments", func(t *testing.T) {
		payload, err := reviewPayload("approve", "", "abc123", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if payload.Event != "APPROVE" || payload.CommitID != "abc123" {
			t.Errorf("payload = %+v", payload)
		}
		if payload.Comments != nil || payload.Body != "" {
			t.Errorf("expected an empty approval, got %+v", payload)
		}
	})

	t.Run("a single-line comment is a line, never a range", func(t *testing.T) {
		payload, err := reviewPayload("comment", "", "", []ReviewComment{
			{Path: "internal/project/pr.go", Line: 61, StartLine: 61, Body: "this buffers"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := payload.Comments[0]
		if got.Line != 61 || got.StartLine != 0 || got.StartSide != "" {
			t.Errorf("comment = %+v, want a bare line", got)
		}
		// An unset side is the new file: where all but a deletion comment goes.
		if got.Side != "RIGHT" {
			t.Errorf("side = %q, want RIGHT", got.Side)
		}
	})

	t.Run("a multi-line comment keeps both ends on the same side", func(t *testing.T) {
		payload, err := reviewPayload("request_changes", "two things", "", []ReviewComment{
			{Path: "a.go", Line: 61, StartLine: 58, Side: "LEFT", Body: "x"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := payload.Comments[0]
		if got.StartLine != 58 || got.Side != "LEFT" || got.StartSide != "LEFT" {
			t.Errorf("comment = %+v", got)
		}
	})

	t.Run("what GitHub would refuse never becomes a call", func(t *testing.T) {
		cases := []struct {
			name     string
			event    string
			body     string
			comments []ReviewComment
		}{
			{"unknown verdict", "merge-it", "x", nil},
			{"a review that says nothing", "comment", "", nil},
			{"changes requested with nothing to change", "request_changes", "", nil},
			{"a comment with no body", "comment", "", []ReviewComment{{Path: "a.go", Line: 3}}},
			{"a comment with no file", "comment", "", []ReviewComment{{Line: 3, Body: "x"}}},
			{"a comment with no line", "comment", "", []ReviewComment{{Path: "a.go", Body: "x"}}},
			{"a side that is neither", "comment", "", []ReviewComment{{Path: "a.go", Line: 3, Side: "MIDDLE", Body: "x"}}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if _, err := reviewPayload(tc.event, tc.body, "", tc.comments); err == nil {
					t.Error("expected a refusal before the shell-out")
				}
			})
		}
	})
}

func TestSubmitReviewFlow(t *testing.T) {
	t.Parallel()
	t.Run("the review reaches gh as a JSON body, and the file does not outlive it", func(t *testing.T) {
		gh := &fakeGH{}
		comments := []ReviewComment{{Path: "a.go", Line: 4, Body: "why"}}
		if _, err := withGH(gh).SubmitReview(t.Context(), api.SubmitReviewOptions{Path: "/repo", Number: 42, Event: "request_changes", Body: "two things", Head: "abc", Comments: comments}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(gh.args) != 6 {
			t.Fatalf("args = %v", gh.args)
		}
		head := []string{"api", "--method", "POST", "repos/{owner}/{repo}/pulls/42/reviews", "--input"}
		if !slices.Equal(gh.args[:5], head) {
			t.Errorf("args = %v, want %v followed by a file", gh.args, head)
		}
		if _, err := os.Stat(gh.args[5]); !os.IsNotExist(err) {
			t.Errorf("staged file %q outlived the call", gh.args[5])
		}
		if gh.dir != "/repo" || gh.timeout != prReviewTimeout {
			t.Errorf("wrong scope: dir %q timeout %v", gh.dir, gh.timeout)
		}
	})

	t.Run("the staged body is what GitHub expects", func(t *testing.T) {
		// The file is gone by the time the call returns, so the body is checked
		// where gh would read it: inside the runner.
		var staged []byte
		svc := &Service{runner: func(_ time.Duration, _, _ string, args ...string) ([]byte, error) {
			staged, _ = os.ReadFile(args[len(args)-1])
			return nil, nil
		}}
		_, err := svc.SubmitReview(t.Context(), api.SubmitReviewOptions{Path: "/repo", Number: 7, Event: "approve", Head: "deadbeef", Comments: []ReviewComment{
			{Path: "a.go", Line: 9, StartLine: 7, Body: "here"},
		}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var body map[string]any
		if err := json.Unmarshal(staged, &body); err != nil {
			t.Fatalf("staged body is not JSON: %v (%s)", err, staged)
		}
		if body["event"] != "APPROVE" || body["commit_id"] != "deadbeef" {
			t.Errorf("body = %v", body)
		}
		comments, _ := body["comments"].([]any)
		if len(comments) != 1 {
			t.Fatalf("comments = %v", body["comments"])
		}
		comment, _ := comments[0].(map[string]any)
		if comment["start_line"] != float64(7) || comment["start_side"] != "RIGHT" {
			t.Errorf("comment = %v, want GitHub's snake_case range", comment)
		}
	})

	t.Run("an invalid review never reaches gh", func(t *testing.T) {
		gh := &fakeGH{}
		if _, err := withGH(gh).SubmitReview(t.Context(), api.SubmitReviewOptions{Path: "/repo", Number: 1, Event: "nope"}); err == nil {
			t.Error("expected an error for an unknown verdict")
		}
		if gh.calls != 0 {
			t.Errorf("gh was called %d times, want 0", gh.calls)
		}
	})

	t.Run("GitHub's refusal reaches the caller as written", func(t *testing.T) {
		gh := &fakeGH{err: testError("GitHub does not let an account approve its own pull request.")}
		_, err := withGH(gh).SubmitReview(t.Context(), api.SubmitReviewOptions{Path: "/repo", Number: 7, Event: "approve"})
		if err == nil || !strings.Contains(err.Error(), "approve its own") {
			t.Errorf("error = %v, want the runner's message unchanged", err)
		}
	})
}

func TestReplyToReviewThreadFlow(t *testing.T) {
	t.Parallel()
	t.Run("a reply is addressed to the comment's database id", func(t *testing.T) {
		gh := &fakeGH{}
		if _, err := withGH(gh).ReplyToReviewThread(t.Context(), api.PRCommentBodyOptions{Path: "/repo", Number: 42, CommentID: 900001, Body: "dropped it"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{
			"api", "--method", "POST",
			"repos/{owner}/{repo}/pulls/42/comments/900001/replies",
			"-f", "body=dropped it",
		}
		if !slices.Equal(gh.args, want) {
			t.Errorf("args = %v, want %v", gh.args, want)
		}
	})

	t.Run("an empty reply never reaches gh", func(t *testing.T) {
		gh := &fakeGH{}
		if _, err := withGH(gh).ReplyToReviewThread(t.Context(), api.PRCommentBodyOptions{Path: "/repo", Number: 42, CommentID: 1}); err == nil {
			t.Error("expected an error for an empty reply")
		}
		if gh.calls != 0 {
			t.Errorf("gh was called %d times, want 0", gh.calls)
		}
	})
}

func TestEditReviewCommentFlow(t *testing.T) {
	t.Parallel()
	t.Run("an edit patches the comment by its database id", func(t *testing.T) {
		gh := &fakeGH{}
		if _, err := withGH(gh).EditReviewComment(t.Context(), api.PRCommentBodyOptions{Path: "/repo", Number: 42, CommentID: 900001, Body: "reworded"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{
			"api", "--method", "PATCH",
			"repos/{owner}/{repo}/pulls/comments/900001",
			"-f", "body=reworded",
		}
		if !slices.Equal(gh.args, want) {
			t.Errorf("args = %v, want %v", gh.args, want)
		}
	})

	t.Run("an empty edit never reaches gh", func(t *testing.T) {
		gh := &fakeGH{}
		if _, err := withGH(gh).EditReviewComment(t.Context(), api.PRCommentBodyOptions{Path: "/repo", Number: 42, CommentID: 1}); err == nil {
			t.Error("expected an error for an empty comment")
		}
		if gh.calls != 0 {
			t.Errorf("gh was called %d times, want 0", gh.calls)
		}
	})
}

func TestConversationWritesAddressGitHub(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		write func(s *Service) error
		want  []string
	}{
		"deleting a thread comment": {
			func(s *Service) error {
				if _, err := s.DeleteReviewComment(t.Context(), api.PRCommentOptions{Path: "/repo", Number: 42, CommentID: 900001}); err != nil {
					return err
				}
				return nil
			},
			[]string{"api", "--method", "DELETE", "repos/{owner}/{repo}/pulls/comments/900001"},
		},
		"editing a pull request comment": {
			func(s *Service) error {
				if _, err := s.EditPullRequestComment(t.Context(), api.PRCommentBodyOptions{Path: "/repo", Number: 42, CommentID: 800001, Body: "reworded"}); err != nil {
					return err
				}
				return nil
			},
			[]string{"api", "--method", "PATCH", "repos/{owner}/{repo}/issues/comments/800001", "-f", "body=reworded"},
		},
		"deleting a pull request comment": {
			func(s *Service) error {
				if _, err := s.DeletePullRequestComment(t.Context(), api.PRCommentOptions{Path: "/repo", Number: 42, CommentID: 800001}); err != nil {
					return err
				}
				return nil
			},
			[]string{"api", "--method", "DELETE", "repos/{owner}/{repo}/issues/comments/800001"},
		},
		"editing a review's summary": {
			func(s *Service) error {
				if _, err := s.EditReviewBody(t.Context(), api.PRReviewBodyOptions{Path: "/repo", Number: 42, ReviewID: 700001, Body: "reworded"}); err != nil {
					return err
				}
				return nil
			},
			[]string{"api", "--method", "PUT", "repos/{owner}/{repo}/pulls/42/reviews/700001", "-f", "body=reworded"},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			gh := &fakeGH{}
			if err := c.write(withGH(gh)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(gh.args, c.want) {
				t.Errorf("args = %v, want %v", gh.args, c.want)
			}
		})
	}
}

func TestConversationEditsRefuseAnEmptyBody(t *testing.T) {
	t.Parallel()
	edits := map[string]func(s *Service) error{
		"pull request comment": func(s *Service) error {
			if _, err := s.EditPullRequestComment(t.Context(), api.PRCommentBodyOptions{Path: "/repo", Number: 42, CommentID: 1}); err != nil {
				return err
			}
			return nil
		},
		"review summary": func(s *Service) error {
			if _, err := s.EditReviewBody(t.Context(), api.PRReviewBodyOptions{Path: "/repo", Number: 42, ReviewID: 1}); err != nil {
				return err
			}
			return nil
		},
	}
	for name, edit := range edits {
		t.Run(name, func(t *testing.T) {
			gh := &fakeGH{}
			if err := edit(withGH(gh)); err == nil {
				t.Error("expected an error for an empty body")
			}
			if gh.calls != 0 {
				t.Errorf("gh was called %d times, want 0", gh.calls)
			}
		})
	}
}

func TestResolveReviewThreadFlow(t *testing.T) {
	t.Parallel()
	t.Run("resolving and reopening are two mutations, one node id", func(t *testing.T) {
		gh := &fakeGH{}
		if _, err := withGH(gh).ResolveReviewThread(t.Context(), api.ResolveThreadOptions{Path: "/repo", ThreadID: "PRRT_kw1", Resolved: true}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"api", "graphql", "-f", "query=" + resolveThreadMutation, "-f", "id=PRRT_kw1"}
		if !slices.Equal(gh.args, want) {
			t.Errorf("args = %v, want %v", gh.args, want)
		}

		if _, err := withGH(gh).ResolveReviewThread(t.Context(), api.ResolveThreadOptions{Path: "/repo", ThreadID: "PRRT_kw1"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(gh.args[3], "unresolveReviewThread") {
			t.Errorf("args = %v, want the unresolve mutation", gh.args)
		}
	})

	t.Run("no thread never reaches gh", func(t *testing.T) {
		gh := &fakeGH{}
		if _, err := withGH(gh).ResolveReviewThread(t.Context(), api.ResolveThreadOptions{Path: "/repo", Resolved: true}); err == nil {
			t.Error("expected an error without a thread")
		}
		if gh.calls != 0 {
			t.Errorf("gh was called %d times, want 0", gh.calls)
		}
	})
}

func TestCommentOnPullRequestFlow(t *testing.T) {
	t.Parallel()
	t.Run("a numbered pull request is addressed by number", func(t *testing.T) {
		gh := &fakeGH{}
		if _, err := withGH(gh).CommentOnPullRequest(t.Context(), api.PRBodyOptions{Path: "/repo", Number: 42, Body: "rebased"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := []string{"pr", "comment", "42", "--body", "rebased"}; !slices.Equal(gh.args, want) {
			t.Errorf("args = %v, want %v", gh.args, want)
		}
	})

	t.Run("an empty comment never reaches gh", func(t *testing.T) {
		gh := &fakeGH{}
		if _, err := withGH(gh).CommentOnPullRequest(t.Context(), api.PRBodyOptions{Path: "/repo", Number: 42}); err == nil {
			t.Error("expected an error for an empty comment")
		}
		if gh.calls != 0 {
			t.Errorf("gh was called %d times, want 0", gh.calls)
		}
	})
}

func TestEditPullRequestFlow(t *testing.T) {
	t.Parallel()
	t.Run("a rename sends the title alone", func(t *testing.T) {
		gh := &fakeGH{}
		if _, err := withGH(gh).EditPullRequestTitle(t.Context(), api.PRTitleOptions{Path: "/repo", Number: 42, Title: "New title"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := []string{"pr", "edit", "42", "--title", "New title"}; !slices.Equal(gh.args, want) {
			t.Errorf("args = %v, want %v", gh.args, want)
		}
	})

	t.Run("a blank title never reaches gh", func(t *testing.T) {
		gh := &fakeGH{}
		if _, err := withGH(gh).EditPullRequestTitle(t.Context(), api.PRTitleOptions{Path: "/repo", Number: 42, Title: "  "}); err == nil {
			t.Error("expected an error for a blank title")
		}
		if gh.calls != 0 {
			t.Errorf("gh was called %d times, want 0", gh.calls)
		}
	})

	t.Run("a description edit sends the body alone, empty clearing it", func(t *testing.T) {
		gh := &fakeGH{}
		if _, err := withGH(gh).EditPullRequestBody(t.Context(), api.PRBodyOptions{Path: "/repo", Number: 42}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := []string{"pr", "edit", "42", "--body", ""}; !slices.Equal(gh.args, want) {
			t.Errorf("args = %v, want %v", gh.args, want)
		}
	})
}
