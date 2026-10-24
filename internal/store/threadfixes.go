package store

import (
	"fmt"

	"github.com/lichdotdev/lich/api"
)

// ThreadFixes returns every review thread waiting for its fix, in the order
// they were sent.
func (s *Service) ThreadFixes() ([]api.ThreadFix, error) {
	rows, err := s.db.Query(`SELECT thread_id, path, number, comment_id, head FROM thread_fixes ORDER BY rowid`)
	if err != nil {
		return nil, fmt.Errorf("query thread fixes: %w", err)
	}
	defer rows.Close()
	var out []api.ThreadFix
	for rows.Next() {
		var f api.ThreadFix
		if err := rows.Scan(&f.ThreadID, &f.Path, &f.Number, &f.CommentID, &f.Head); err != nil {
			return nil, fmt.Errorf("scan thread fix: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// SaveThreadFix records f, replacing what an earlier send of the same thread
// recorded: the thread waits on the head it was last sent at.
func (s *Service) SaveThreadFix(f api.ThreadFix) error {
	_, err := s.db.Exec(
		`INSERT INTO thread_fixes (thread_id, path, number, comment_id, head) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (thread_id) DO UPDATE SET
		     path = excluded.path, number = excluded.number, comment_id = excluded.comment_id, head = excluded.head`,
		f.ThreadID, f.Path, f.Number, f.CommentID, f.Head,
	)
	if err != nil {
		return fmt.Errorf("save thread fix: %w", err)
	}
	return nil
}

// DeleteThreadFix stops thread id waiting. One that is not waiting is not an
// error.
func (s *Service) DeleteThreadFix(threadID string) error {
	if _, err := s.db.Exec(`DELETE FROM thread_fixes WHERE thread_id = ?`, threadID); err != nil {
		return fmt.Errorf("delete thread fix: %w", err)
	}
	return nil
}
