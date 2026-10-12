package store

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/omartelo/lich/internal/claudeprofile"
)

// claudeProfileKey holds the Claude Code profile new sessions are born in: a
// slug from internal/claudeprofile. Scoped like the sandbox rung — a project
// value wins over the global one — because which account a repository is
// worked under is a decision about that repository. Globally "" and
// "personal" both mean Personal; in a project "" inherits the global value and
// "personal" is how a project names Personal over a global profile.
const claudeProfileKey = "provider.claude.profile"

// ClaudeProfile is the profile a Claude Code session born in this project now
// runs in, "" for Personal.
func (s *Service) ClaudeProfile(projectID string) string {
	slug := s.projectSetting(claudeProfileKey, projectID)
	if claudeprofile.IsPersonal(slug) {
		return ""
	}
	return slug
}

// ForgetClaudeProfile clears every setting that names a deleted profile, so a
// project set to it falls back to the global value and the global one to
// Personal. The sessions born in it keep their mark: what they ran in is a fact
// about them, and resuming one reports the profile gone (ErrProfileGone).
func (s *Service) ForgetClaudeProfile(slug string) error {
	if _, err := s.db.Exec(
		`DELETE FROM settings WHERE key = ? AND value = ?`, claudeProfileKey, slug,
	); err != nil {
		return fmt.Errorf("forget claude profile %q: %w", slug, err)
	}
	return nil
}

// SetSessionProfile records the Claude Code profile a session runs in, "" for
// Personal. A session is born in its project's profile (addSession); this moves
// it, which is what Continue in does once its conversation has moved, and what a
// resume does when the conversation turns out to live in another profile.
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

// SessionProfile is the Claude Code profile a session runs in, "" for Personal
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
