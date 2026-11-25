package store

import (
	"context"
	"fmt"
	"time"

	"github.com/lichdotdev/lich/api"
)

// viewedFilesTTL is how long a pull request's Viewed ticks outlive its last
// tick: a closed or merged one is never opened again, and nothing else
// collects its rows.
const viewedFilesTTL = 90 * 24 * time.Hour

// ViewedFiles is every file of pull request opts.PR ticked as viewed.
func (s *Service) ViewedFiles(_ context.Context, opts api.ViewedFilesOptions) ([]api.ViewedFile, error) {
	rows, err := s.db.Query(`SELECT path, hash, viewed_at FROM viewed_files WHERE pr_url = ? ORDER BY path`, opts.PR)
	if err != nil {
		return nil, fmt.Errorf("query viewed files: %w", err)
	}
	defer rows.Close()
	out := []api.ViewedFile{}
	for rows.Next() {
		var f api.ViewedFile
		if err := rows.Scan(&f.Path, &f.Hash, &f.ViewedAt); err != nil {
			return nil, fmt.Errorf("scan viewed file: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// MarkViewed ticks a file, replacing an earlier tick of it.
func (s *Service) MarkViewed(_ context.Context, opts api.MarkViewedOptions) (api.None, error) {
	return api.None{}, s.markViewed(opts, time.Now())
}

func (s *Service) markViewed(opts api.MarkViewedOptions, at time.Time) error {
	_, err := s.db.Exec(
		`INSERT INTO viewed_files (pr_url, path, hash, viewed_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT (pr_url, path) DO UPDATE SET hash = excluded.hash, viewed_at = excluded.viewed_at`,
		opts.PR, opts.Path, opts.Hash, at.Unix(),
	)
	if err != nil {
		return fmt.Errorf("mark viewed: %w", err)
	}
	return nil
}

// UnmarkViewed unticks a file. One that is not ticked is not an error.
func (s *Service) UnmarkViewed(_ context.Context, opts api.UnmarkViewedOptions) (api.None, error) {
	if _, err := s.db.Exec(`DELETE FROM viewed_files WHERE pr_url = ? AND path = ?`, opts.PR, opts.Path); err != nil {
		return api.None{}, fmt.Errorf("unmark viewed: %w", err)
	}
	return api.None{}, nil
}

// pruneViewedFiles drops every tick of a pull request whose latest tick is
// older than viewedFilesTTL, so one still under review keeps all of its.
func (s *Service) pruneViewedFiles(now time.Time) error {
	_, err := s.db.Exec(
		`DELETE FROM viewed_files WHERE pr_url IN
		     (SELECT pr_url FROM viewed_files GROUP BY pr_url HAVING MAX(viewed_at) < ?)`,
		now.Add(-viewedFilesTTL).Unix(),
	)
	if err != nil {
		return fmt.Errorf("prune viewed files: %w", err)
	}
	return nil
}
