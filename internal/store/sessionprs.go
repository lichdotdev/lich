package store

import (
	"encoding/json"
	"fmt"

	"github.com/lichdotdev/lich/api"
)

// SessionPullRequests is every pull request linked to or unlinked from a
// session by hand, the newest word first.
func (s *Service) SessionPullRequests() ([]api.SessionPullRequest, error) {
	rows, err := s.db.Query(`SELECT session_id, pr_url, linked, at, forge, here, pr, error FROM session_pull_requests ORDER BY at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query session pull requests: %w", err)
	}
	defer rows.Close()
	out := []api.SessionPullRequest{}
	for rows.Next() {
		var l api.SessionPullRequest
		var pr string
		if err := rows.Scan(&l.Session, &l.URL, &l.Linked, &l.At, &l.Forge, &l.Here, &pr, &l.Error); err != nil {
			return nil, fmt.Errorf("scan session pull request: %w", err)
		}
		if pr != "" {
			if err := json.Unmarshal([]byte(pr), &l.PR); err != nil {
				return nil, fmt.Errorf("read the pull request of %s: %w", l.URL, err)
			}
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// SaveSessionPullRequest records l, replacing the earlier word on the same
// session and URL. A nil PR keeps the read the row already holds.
func (s *Service) SaveSessionPullRequest(l api.SessionPullRequest) error {
	pr := ""
	if l.PR != nil {
		b, err := json.Marshal(l.PR)
		if err != nil {
			return fmt.Errorf("encode the pull request of %s: %w", l.URL, err)
		}
		pr = string(b)
	}
	_, err := s.db.Exec(
		`INSERT INTO session_pull_requests (session_id, pr_url, linked, at, forge, here, pr, error) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT (session_id, pr_url) DO UPDATE SET
		     linked = excluded.linked, at = excluded.at, forge = excluded.forge, here = excluded.here,
		     pr = CASE excluded.pr WHEN '' THEN pr ELSE excluded.pr END, error = excluded.error`,
		l.Session, l.URL, l.Linked, l.At, l.Forge, l.Here, pr, l.Error,
	)
	if err != nil {
		return fmt.Errorf("save session pull request: %w", err)
	}
	return nil
}
