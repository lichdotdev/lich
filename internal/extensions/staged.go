package extensions

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lichdotdev/lich/api"
)

const (
	// stagedTTL is how long a prepared install waits for the person's answer
	// before its copy is deleted: long enough to read the confirmation, short
	// enough that an abandoned download does not sit on disk.
	stagedTTL = 15 * time.Minute
	// tokenBytes is the randomness of a staged install's token.
	tokenBytes = 16
)

var errNoStaged = errors.New("this install is no longer waiting: it was installed, cancelled or expired; start it again")

// stagedInstall is a prepared install: work holds id's checked copy, and
// replaces is the installed extension an update takes the place of.
type stagedInstall struct {
	id       string
	work     string
	replaces *installed
	timer    *time.Timer
}

// hold records work as staged under a new token, deleted after stagedTTL
// unless CommitInstall or DiscardInstall takes it first.
func (s *Service) hold(id, work string, replaces *installed) (string, error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("name the staged install: %w", err)
	}
	token := hex.EncodeToString(raw)
	s.stagedMu.Lock()
	defer s.stagedMu.Unlock()
	if s.staged == nil {
		s.staged = map[string]stagedInstall{}
	}
	timer := time.AfterFunc(s.timing.stagedTTL, func() { s.expire(token) })
	s.staged[token] = stagedInstall{id: id, work: work, replaces: replaces, timer: timer}
	return token, nil
}

// take removes token's staged install from the registry, which is what makes
// a token good for one commit or discard.
func (s *Service) take(token string) (stagedInstall, bool) {
	s.stagedMu.Lock()
	defer s.stagedMu.Unlock()
	st, ok := s.staged[token]
	if ok {
		delete(s.staged, token)
		st.timer.Stop()
	}
	return st, ok
}

// drop deletes token's staged copy, if it is still staged.
func (s *Service) drop(token string) error {
	st, ok := s.take(token)
	if !ok {
		return errNoStaged
	}
	if err := os.RemoveAll(st.work); err != nil {
		return fmt.Errorf("delete %s: %w; delete it by hand", st.work, err)
	}
	return nil
}

func (s *Service) expire(token string) {
	if err := s.drop(token); err != nil && !errors.Is(err, errNoStaged) {
		slog.Warn("extensions: delete an expired staged install", "err", err)
	}
}

// CommitInstall moves the staged copy into the extensions directory and
// starts it, in place of the installed version when it is an update.
func (s *Service) CommitInstall(ctx context.Context, opts api.ExtensionInstallTokenOptions) (api.Extension, error) {
	st, ok := s.take(opts.Token)
	if !ok {
		return api.Extension{}, errNoStaged
	}
	s.installMu.Lock()
	defer s.installMu.Unlock()
	if st.replaces != nil {
		return s.replace(ctx, st)
	}
	defer func() { _ = os.RemoveAll(st.work) }()
	// Another install of the id may have committed since this one was staged.
	if err := s.checkFree(st.id); err != nil {
		return api.Extension{}, err
	}
	return s.place(st.id, filepath.Join(st.work, st.id))
}

// DiscardInstall deletes the staged copy.
func (s *Service) DiscardInstall(_ context.Context, opts api.ExtensionInstallTokenOptions) (api.None, error) {
	return api.None{}, s.drop(opts.Token)
}

// sweepStaged deletes the staged installs under dir, which only an earlier
// run of lich can have left: no token names them any more.
func sweepStaged(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), stagingPrefix) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			slog.Warn("extensions: delete a staged install", "dir", entry.Name(), "err", err)
		}
	}
}
