package store

import (
	"fmt"

	"github.com/lichdotdev/lich/api"
)

// Services returns every project service, each project's in the order they
// were added.
func (s *Service) Services() ([]api.ServiceConfig, error) {
	rows, err := s.db.Query(
		`SELECT id, project_id, name, command, start_with_lich
		   FROM project_services ORDER BY project_id, position, rowid`,
	)
	if err != nil {
		return nil, fmt.Errorf("query services: %w", err)
	}
	defer rows.Close()
	var out []api.ServiceConfig
	for rows.Next() {
		var c api.ServiceConfig
		if err := rows.Scan(&c.ID, &c.ProjectID, &c.Name, &c.Command, &c.StartWithLich); err != nil {
			return nil, fmt.Errorf("scan service: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SaveService writes c, inserting it after its project's last service when its
// id is new. The project it names must exist, and its name must be free in
// that project: both are the schema's constraints, and a write that breaks
// either fails.
func (s *Service) SaveService(c api.ServiceConfig) error {
	_, err := s.db.Exec(
		`INSERT INTO project_services (id, project_id, name, command, start_with_lich, position)
		 VALUES (?, ?, ?, ?, ?,
		         (SELECT COALESCE(MAX(position), 0) + 1 FROM project_services WHERE project_id = ?))
		 ON CONFLICT (id) DO UPDATE SET
		     name = excluded.name, command = excluded.command, start_with_lich = excluded.start_with_lich`,
		c.ID, c.ProjectID, c.Name, c.Command, c.StartWithLich, c.ProjectID,
	)
	if err != nil {
		return fmt.Errorf("save service %q: %w", c.Name, err)
	}
	return nil
}

// DeleteService removes service id. One that is already gone is not an error.
func (s *Service) DeleteService(id string) error {
	if _, err := s.db.Exec(`DELETE FROM project_services WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete service: %w", err)
	}
	return nil
}

// ProjectName is the name of project id, "" when there is no such project.
func (s *Service) ProjectName(projectID string) string {
	var name string
	if err := s.db.QueryRow(`SELECT name FROM projects WHERE id = ?`, projectID).Scan(&name); err != nil {
		return ""
	}
	return name
}
