package store

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/omartelo/lich/internal/claudeprofile"
	"github.com/omartelo/lich/internal/providers"
)

// claudeProfileKey holds the Claude Code profile new sessions are born in: a
// slug from internal/claudeprofile. Scoped like the sandbox rung (a project
// value wins over the global one) because which account a repository is
// worked under is a decision about that repository. Globally "" and
// "personal" both mean Personal; in a project "" inherits the global value and
// "personal" is how a project names Personal over a global profile.
const claudeProfileKey = "provider.claude.profile"

// codexProfileKey is claudeProfileKey for Codex, whose profiles are their own
// set (internal/codexprofile) with the same reserved "personal" slug.
const codexProfileKey = "provider.codex.profile"

// ompProfileKey is claudeProfileKey for oh-my-pi (internal/ompprofile).
const ompProfileKey = "provider.omp.profile"

// xdgProfileKeys are claudeProfileKey for the providers whose profiles are
// link homes (internal/xdgprofile), each its own set.
var xdgProfileKeys = map[string]string{
	providers.OpenCode: "provider.opencode.profile",
	providers.Cursor:   "provider.cursor.profile",
	providers.Kiro:     "provider.kiro.profile",
}

// ClaudeProfile is the profile a Claude Code session born in this project now
// runs in, "" for Personal.
func (s *Service) ClaudeProfile(projectID string) string {
	return s.profileSetting(claudeProfileKey, projectID)
}

// CodexProfile is the profile a Codex session born in this project now runs in,
// "" for Personal.
func (s *Service) CodexProfile(projectID string) string {
	return s.profileSetting(codexProfileKey, projectID)
}

// OMPProfile is the profile an oh-my-pi session born in this project now runs
// in, "" for Personal.
func (s *Service) OMPProfile(projectID string) string {
	return s.profileSetting(ompProfileKey, projectID)
}

func (s *Service) profileSetting(key, projectID string) string {
	slug := s.projectSetting(key, projectID)
	if claudeprofile.IsPersonal(slug) {
		return ""
	}
	return slug
}

// bornProfile is the profile a session of this provider is born in: the
// project's setting for a provider that has profiles, "" for the rest.
func (s *Service) bornProfile(kind, projectID string) string {
	switch kind {
	case providers.Claude:
		return s.ClaudeProfile(projectID)
	case providers.Codex:
		return s.CodexProfile(projectID)
	case providers.OMP:
		return s.OMPProfile(projectID)
	}
	if key, ok := xdgProfileKeys[kind]; ok {
		return s.profileSetting(key, projectID)
	}
	return ""
}

// ForgetClaudeProfile clears every setting that names a deleted profile, so a
// project set to it falls back to the global value and the global one to
// Personal. The sessions born in it keep their mark: what they ran in is a fact
// about them, and resuming one reports the profile gone (ErrProfileGone).
func (s *Service) ForgetClaudeProfile(slug string) error {
	return s.forgetProfile(claudeProfileKey, slug)
}

// ForgetCodexProfile is ForgetClaudeProfile for a Codex profile.
func (s *Service) ForgetCodexProfile(slug string) error {
	return s.forgetProfile(codexProfileKey, slug)
}

// ForgetOMPProfile is ForgetClaudeProfile for an oh-my-pi profile.
func (s *Service) ForgetOMPProfile(slug string) error {
	return s.forgetProfile(ompProfileKey, slug)
}

// ForgetXDGProfile is ForgetClaudeProfile for an opencode, Cursor CLI or Kiro
// CLI profile.
func (s *Service) ForgetXDGProfile(kind, slug string) error {
	key, ok := xdgProfileKeys[kind]
	if !ok {
		return fmt.Errorf("%s has no profiles", kind)
	}
	return s.forgetProfile(key, slug)
}

func (s *Service) forgetProfile(key, slug string) error {
	if _, err := s.db.Exec(
		`DELETE FROM settings WHERE key = ? AND value = ?`, key, slug,
	); err != nil {
		return fmt.Errorf("forget profile %q (%s): %w", slug, key, err)
	}
	return nil
}

// SessionKind is the provider a session runs, "" for a session with no row or
// when the read fails: a caller that needs the answer to be right asks the
// session's spawn first.
func (s *Service) SessionKind(sessionID string) string {
	var kind string
	if err := s.db.QueryRow(`SELECT kind FROM sessions WHERE id = ?`, sessionID).Scan(&kind); err != nil {
		return ""
	}
	return kind
}

// SetSessionProfile records the profile a session runs in, "" for Personal. A
// session is born in its project's profile (addSession); this moves it, which
// is what Continue in does once its conversation has moved, and what a resume
// does when the conversation turns out to live in another profile.
func (s *Service) SetSessionProfile(sessionID, profile string) error {
	if claudeprofile.IsPersonal(profile) {
		profile = ""
	}
	if _, err := s.db.Exec(
		`UPDATE sessions SET profile = ? WHERE id = ?`, profile, sessionID,
	); err != nil {
		return fmt.Errorf("set profile on %q: %w", sessionID, err)
	}
	return nil
}

// SessionProfile is the profile a session runs in, "" for Personal
// and for a session with no row. A failed read is an error rather than
// Personal: spawning into the wrong account is not a default to fall back to.
func (s *Service) SessionProfile(sessionID string) (string, error) {
	var profile string
	err := s.db.QueryRow(`SELECT profile FROM sessions WHERE id = ?`, sessionID).Scan(&profile)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read profile on %q: %w", sessionID, err)
	}
	return profile, nil
}
