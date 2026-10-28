package spawn

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/lichdotdev/lich/api"
)

type ClosedProject = api.ClosedProject

type CloseProjectOptions = api.CloseProjectOptions

// CloseProject takes a project off the window's strip the way its tab's close
// does: the row is marked closed with its sessions kept, so a reopen (an open
// on its path, or the window's history) brings them back, and their terminals
// are stopped. Nothing about a checkout is decided, because nothing is removed.
//
// The window stops the terminals itself after its own close
// (native/cmd/lich-native/topbar_apply.go, closeProject) and only drops its
// views on project-closed, so a close from here stops them in the backend, in
// the window's order: the row first, so a refused close leaves every session
// running.
//
// Two projects are refused. The caller's own, because closing it takes down the
// agent asking, halfway through the call. And Home, which the window keeps open
// for good (errHomeIsPermanent there).
func (s *Service) CloseProject(ctx context.Context, opts CloseProjectOptions) (ClosedProject, error) {
	if strings.TrimSpace(opts.Project) == "" {
		return ClosedProject{}, fmt.Errorf("no project given to close")
	}
	projects, err := s.sessions.LoadState(ctx, api.None{})
	if err != nil {
		return ClosedProject{}, fmt.Errorf("read the workspace: %w", err)
	}
	target, err := resolveProject(projects, opts.From, opts.Project)
	if err != nil {
		return ClosedProject{}, err
	}
	for _, sess := range target.Sessions {
		if sess.ID == opts.From {
			return ClosedProject{}, fmt.Errorf(
				"%q is the project this session runs in — closing it would take down the agent asking for it",
				target.Name,
			)
		}
	}
	if home, err := os.UserHomeDir(); err == nil && target.Path == home {
		return ClosedProject{}, fmt.Errorf("%q is the home project, which the window always keeps open", target.Name)
	}

	if _, err := s.sessions.CloseProject(ctx, api.ProjectIDOptions{ID: target.ID}); err != nil {
		return ClosedProject{}, err
	}
	for _, sess := range target.Sessions {
		// The row is already closed, so there is nothing to undo; the window's
		// own close logs the same failure and carries on.
		if err := s.term.CloseBy(sess.ID, opts.From); err != nil {
			slog.Warn("spawn: close the session's terminal", "session", sess.ID, "err", err)
		}
	}
	return ClosedProject{ID: target.ID, Name: target.Name, Path: target.Path, Sessions: len(target.Sessions)}, nil
}
