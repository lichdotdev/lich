package project

import (
	"errors"
	"testing"

	"github.com/lichdotdev/lich/api"
)

type changeLog struct {
	names []string
	data  []PRChanged
}

func (l *changeLog) emit(name string, data any) {
	l.names = append(l.names, name)
	l.data = append(l.data, data.(PRChanged))
}

func TestPullRequestWritesAnnounceTheChange(t *testing.T) {
	t.Parallel()
	writes := map[string]func(s *Service) error{
		"merge": func(s *Service) error {
			if _, err := s.MergePullRequest(t.Context(), api.MergePullRequestOptions{Path: "/repo", Number: 7, Method: "squash"}); err != nil {
				return err
			}
			return nil
		},
		"submit review": func(s *Service) error {
			if _, err := s.SubmitReview(t.Context(), api.SubmitReviewOptions{Path: "/repo", Number: 7, Event: "approve"}); err != nil {
				return err
			}
			return nil
		},
		"reply": func(s *Service) error {
			if _, err := s.ReplyToReviewThread(t.Context(), api.PRCommentBodyOptions{Path: "/repo", Number: 7, CommentID: 1, Body: "ok"}); err != nil {
				return err
			}
			return nil
		},
		"edit comment": func(s *Service) error {
			if _, err := s.EditReviewComment(t.Context(), api.PRCommentBodyOptions{Path: "/repo", Number: 7, CommentID: 1, Body: "ok"}); err != nil {
				return err
			}
			return nil
		},
		"delete comment": func(s *Service) error {
			if _, err := s.DeleteReviewComment(t.Context(), api.PRCommentOptions{Path: "/repo", Number: 7, CommentID: 1}); err != nil {
				return err
			}
			return nil
		},
		"edit pr comment": func(s *Service) error {
			if _, err := s.EditPullRequestComment(t.Context(), api.PRCommentBodyOptions{Path: "/repo", Number: 7, CommentID: 1, Body: "ok"}); err != nil {
				return err
			}
			return nil
		},
		"delete pr comment": func(s *Service) error {
			if _, err := s.DeletePullRequestComment(t.Context(), api.PRCommentOptions{Path: "/repo", Number: 7, CommentID: 1}); err != nil {
				return err
			}
			return nil
		},
		"edit review body": func(s *Service) error {
			if _, err := s.EditReviewBody(t.Context(), api.PRReviewBodyOptions{Path: "/repo", Number: 7, ReviewID: 1, Body: "ok"}); err != nil {
				return err
			}
			return nil
		},
		"resolve thread": func(s *Service) error {
			if _, err := s.ResolveReviewThread(t.Context(), api.ResolveThreadOptions{Path: "/repo", ThreadID: "T_1", Resolved: true}); err != nil {
				return err
			}
			return nil
		},
		"comment": func(s *Service) error {
			if _, err := s.CommentOnPullRequest(t.Context(), api.PRBodyOptions{Path: "/repo", Number: 7, Body: "hi"}); err != nil {
				return err
			}
			return nil
		},
		"edit title": func(s *Service) error {
			if _, err := s.EditPullRequestTitle(t.Context(), api.PRTitleOptions{Path: "/repo", Number: 7, Title: "title"}); err != nil {
				return err
			}
			return nil
		},
		"edit body": func(s *Service) error {
			if _, err := s.EditPullRequestBody(t.Context(), api.PRBodyOptions{Path: "/repo", Number: 7}); err != nil {
				return err
			}
			return nil
		},
	}
	for name, write := range writes {
		t.Run(name+" emits once it landed", func(t *testing.T) {
			log := &changeLog{}
			s := withGH(&fakeGH{})
			s.SetEmit(log.emit)
			if err := write(s); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(log.names) != 1 || log.names[0] != api.PullRequestChangedEvent.Name {
				t.Fatalf("events = %v, want one %q", log.names, api.PullRequestChangedEvent.Name)
			}
			if log.data[0].Path != "/repo" {
				t.Errorf("path = %q, want /repo", log.data[0].Path)
			}
		})
		t.Run(name+" stays quiet when gh refuses", func(t *testing.T) {
			log := &changeLog{}
			s := withGH(&fakeGH{err: errors.New("refused")})
			s.SetEmit(log.emit)
			if err := write(s); err == nil {
				t.Fatal("expected gh's error")
			}
			if len(log.names) != 0 {
				t.Errorf("events = %v, want none", log.names)
			}
		})
	}
}

func TestPullRequestChangeNamesTheNumber(t *testing.T) {
	t.Parallel()
	log := &changeLog{}
	s := withGH(&fakeGH{})
	s.SetEmit(log.emit)
	if _, err := s.MergePullRequest(t.Context(), api.MergePullRequestOptions{Path: "/repo", Number: 42, Method: "merge"}); err != nil {
		t.Fatal(err)
	}
	if got := log.data[0].Number; got != 42 {
		t.Errorf("number = %d, want 42", got)
	}
}

func TestPullRequestWriteWithoutEmitter(t *testing.T) {
	t.Parallel()
	if _, err := withGH(&fakeGH{}).CommentOnPullRequest(t.Context(), api.PRBodyOptions{Path: "/repo", Number: 7, Body: "hi"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
