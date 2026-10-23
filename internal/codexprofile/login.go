package codexprofile

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"

	"github.com/lichdotdev/lich/api"
	"github.com/lichdotdev/lich/internal/events"
)

// loginTimeout bounds a login lich started: a browser tab the user walked away
// from must not leave a CLI waiting for good.
const loginTimeout = 10 * time.Minute

// loginURL finds the sign-in address in what the CLI prints ("If your browser
// did not open, navigate to this URL to authenticate:", measured on 0.162.1).
// The local callback server's own address is http, which this does not match.
var loginURL = regexp.MustCompile(`https://\S+`)

type LoginEvent = api.ProfileLogin

type login struct {
	cancel context.CancelFunc
}

// Login runs `codex login` as a profile. The CLI opens the browser itself and
// waits for the sign-in page's callback on localhost:1455; what it prints, the
// fallback address included, arrives as LoginEvent output. When the CLI exits,
// `codex login status` decides whether the login took. It returns once the CLI
// is started.
//
// The callback port is fixed, so two logins cannot run at once, in lich or
// outside it: the second CLI says the port is taken, in its own words.
func (s *Service) Login(_ context.Context, opts api.ProfileSlugOptions) (api.None, error) {
	dir, err := ConfigDir(opts.Slug)
	if err != nil {
		return api.None{}, err
	}
	p := Profile{Slug: opts.Slug, Dir: dir, Personal: IsPersonal(opts.Slug)}
	ctx, cancel := context.WithTimeout(context.Background(), loginTimeout)
	cmd := s.command(ctx, p, "login")
	out, w := io.Pipe()
	cmd.Stdout, cmd.Stderr = w, w
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, running := s.logins[opts.Slug]; running {
		cancel()
		return api.None{}, fmt.Errorf("profile %q is already logging in", opts.Slug)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return api.None{}, fmt.Errorf("start %s login: %w", s.bin(), err)
	}
	run := &login{cancel: cancel}
	s.logins[opts.Slug] = run
	relayed := make(chan struct{})
	go func() {
		s.relayOutput(opts.Slug, out)
		close(relayed)
	}()
	go func() {
		err := cmd.Wait()
		_ = w.Close()
		<-relayed
		s.finishLogin(ctx, p, run, err)
	}()
	return api.None{}, nil
}

func (s *Service) relayOutput(slug string, out io.Reader) {
	buf := make([]byte, 4096)
	for {
		n, err := out.Read(buf)
		if n > 0 {
			text := string(buf[:n])
			events.Send(s.emit, api.CodexProfileLoginEvent, LoginEvent{Profile: slug, Output: text, URL: loginURL.FindString(text)})
		}
		if err != nil {
			return
		}
	}
}

func (s *Service) finishLogin(ctx context.Context, p Profile, run *login, waitErr error) {
	s.mu.Lock()
	if s.logins[p.Slug] == run {
		delete(s.logins, p.Slug)
	}
	s.mu.Unlock()
	ended := ctx.Err()
	run.cancel()
	done := LoginEvent{Profile: p.Slug, Done: true}
	switch {
	case errors.Is(ended, context.Canceled):
		done.Cancelled = true
		done.Error = "login cancelled"
		events.Send(s.emit, api.CodexProfileLoginEvent, done)
		return
	case errors.Is(ended, context.DeadlineExceeded):
		done.Error = fmt.Sprintf("login timed out after %s", loginTimeout)
		events.Send(s.emit, api.CodexProfileLoginEvent, done)
		return
	}
	events.Send(s.emit, api.CodexProfileLoginEvent, LoginEvent{Profile: p.Slug, Authenticating: true})
	loggedIn, err := s.loggedIn(p)
	switch {
	case err != nil:
		done.Error = err.Error()
	case !loggedIn && waitErr != nil:
		done.Error = fmt.Sprintf("%s login: %v", s.bin(), waitErr)
	}
	done.LoggedIn = err == nil && loggedIn
	if done.LoggedIn && s.installLich != nil {
		if err := s.installLich(p.Slug); err != nil {
			done.PluginError = err.Error()
		}
	}
	events.Send(s.emit, api.CodexProfileLoginEvent, done)
}

// CancelLogin answers cancelLogin over RPC.
func (s *Service) CancelLogin(_ context.Context, opts api.ProfileSlugOptions) (api.None, error) {
	s.cancelLogin(opts.Slug)
	return api.None{}, nil
}

// cancelLogin ends a login lich started; its last event says it was cancelled.
// Nothing running for that profile is nothing to do.
func (s *Service) cancelLogin(slug string) {
	s.mu.Lock()
	run, ok := s.logins[slug]
	s.mu.Unlock()
	if ok {
		run.cancel()
	}
}

// LoginCommand is the command line that logs a profile in from a terminal, for
// a user who would rather run it in a shell card than through Login.
func (s *Service) LoginCommand(_ context.Context, opts api.ProfileSlugOptions) (string, error) {
	dir, err := ConfigDir(opts.Slug)
	if err != nil {
		return "", err
	}
	if IsPersonal(opts.Slug) {
		dir = ""
	}
	return store.LoginLine(s.bin(), "login", dir), nil
}
