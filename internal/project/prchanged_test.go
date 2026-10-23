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
		"merge":             func(s *Service) error { return s.MergePullRequest("/repo", 7, "squash", "", "", false) },
		"submit review":     func(s *Service) error { return s.SubmitReview("/repo", 7, "approve", "", "", nil) },
		"reply":             func(s *Service) error { return s.ReplyToReviewThread("/repo", 7, 1, "ok") },
		"edit comment":      func(s *Service) error { return s.EditReviewComment("/repo", 7, 1, "ok") },
		"delete comment":    func(s *Service) error { return s.DeleteReviewComment("/repo", 7, 1) },
		"edit pr comment":   func(s *Service) error { return s.EditPullRequestComment("/repo", 7, 1, "ok") },
		"delete pr comment": func(s *Service) error { return s.DeletePullRequestComment("/repo", 7, 1) },
		"edit review body":  func(s *Service) error { return s.EditReviewBody("/repo", 7, 1, "ok") },
		"resolve thread":    func(s *Service) error { return s.ResolveReviewThread("/repo", "T_1", true) },
		"comment":           func(s *Service) error { return s.CommentOnPullRequest("/repo", 7, "hi") },
		"edit title":        func(s *Service) error { return s.EditPullRequestTitle("/repo", 7, "title") },
		"edit body":         func(s *Service) error { return s.EditPullRequestBody("/repo", 7, "") },
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
	if err := s.MergePullRequest("/repo", 42, "merge", "", "", false); err != nil {
		t.Fatal(err)
	}
	if got := log.data[0].Number; got != 42 {
		t.Errorf("number = %d, want 42", got)
	}
}

func TestPullRequestWriteWithoutEmitter(t *testing.T) {
	t.Parallel()
	if err := withGH(&fakeGH{}).CommentOnPullRequest("/repo", 7, "hi"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
