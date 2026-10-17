package project

import (
	"errors"
	"testing"
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
		"merge":          func(s *Service) error { return s.MergePullRequest("/repo", 7, "squash", "", "", false) },
		"edit body":      func(s *Service) error { return s.EditPullRequestBody("/repo", 7, "body") },
		"submit review":  func(s *Service) error { return s.SubmitReview("/repo", 7, "approve", "", "", nil) },
		"reply":          func(s *Service) error { return s.ReplyToReviewThread("/repo", 7, 1, "ok") },
		"resolve thread": func(s *Service) error { return s.ResolveReviewThread("/repo", "T_1", true) },
		"dismiss review": func(s *Service) error { return s.DismissReview("/repo", "R_1", "why") },
		"comment":        func(s *Service) error { return s.CommentOnPullRequest("/repo", 7, "hi") },
		"request review": func(s *Service) error { return s.RequestReview("/repo", 7, "octocat", true) },
	}
	for name, write := range writes {
		t.Run(name+" emits once it landed", func(t *testing.T) {
			log := &changeLog{}
			s := withGH(&fakeGH{})
			s.SetEmit(log.emit)
			if err := write(s); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(log.names) != 1 || log.names[0] != PRChangedEvent {
				t.Fatalf("events = %v, want one %q", log.names, PRChangedEvent)
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
