package claudeprofile

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

// loginURL finds the sign-in address in what the CLI prints ("If the browser
// didn't open, visit: https://…", measured on 2.1.296).
var loginURL = regexp.MustCompile(`https://\S+`)

type LoginEvent = api.ProfileLogin

type login struct {
	cancel context.CancelFunc
	stdin  io.WriteCloser
}

// Login runs `claude auth login --claudeai` as a profile. The CLI opens the
// browser itself; what it prints, the fallback address included, arrives as
// LoginEvent output, and a code the page shows can be handed back through
// LoginCode. When the CLI exits, `claude auth status` decides whether the login
// took, and a logged-in profile gets the lich plugin. It returns once the CLI
// is started.
func (s *Service) Login(_ context.Context, opts api.ProfileSlugOptions) (api.None, error) {
	dir, err := ConfigDir(opts.Slug)
	if err != nil {
		return api.None{}, err
	}
	p := Profile{Slug: opts.Slug, Dir: dir, Personal: IsPersonal(opts.Slug)}
	ctx, cancel := context.WithTimeout(context.Background(), loginTimeout)
	cmd := s.command(ctx, p, "auth", "login", "--claudeai")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return api.None{}, err
	}
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
		return api.None{}, fmt.Errorf("start %s auth login: %w", s.bin(), err)
	}
	run := &login{cancel: cancel, stdin: stdin}
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
			events.Send(s.emit, api.ClaudeProfileLoginEvent, LoginEvent{Profile: slug, Output: text, URL: loginURL.FindString(text)})
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
		events.Send(s.emit, api.ClaudeProfileLoginEvent, done)
		return
	case errors.Is(ended, context.DeadlineExceeded):
		done.Error = fmt.Sprintf("login timed out after %s", loginTimeout)
		events.Send(s.emit, api.ClaudeProfileLoginEvent, done)
		return
	}
	events.Send(s.emit, api.ClaudeProfileLoginEvent, LoginEvent{Profile: p.Slug, Authenticating: true})
	st, err := s.authStatus(p)
	switch {
	case err != nil:
		done.Error = err.Error()
	case !st.LoggedIn && waitErr != nil:
		done.Error = fmt.Sprintf("%s auth login: %v", s.bin(), waitErr)
	}
	done.LoggedIn = err == nil && st.LoggedIn
	if done.LoggedIn && s.installLich != nil {
		if err := s.installLich(p.Slug); err != nil {
			done.PluginError = err.Error()
		}
	}
	events.Send(s.emit, api.ClaudeProfileLoginEvent, done)
}

// LoginCode hands a running login the code the sign-in page showed, for a
// browser that could not reach the CLI on its own.
func (s *Service) LoginCode(_ context.Context, opts api.LoginCodeOptions) (api.None, error) {
	s.mu.Lock()
	run, ok := s.logins[opts.Slug]
	s.mu.Unlock()
	if !ok {
		return api.None{}, fmt.Errorf("profile %q is not logging in", opts.Slug)
	}
	if _, err := io.WriteString(run.stdin, opts.Code+"\n"); err != nil {
		return api.None{}, fmt.Errorf("hand the code to the login: %w", err)
	}
	return api.None{}, nil
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
	return store.LoginLine(s.bin(), "auth login --claudeai", dir), nil
}
