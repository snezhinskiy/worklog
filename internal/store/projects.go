package store

import (
	"fmt"

	"github.com/snezhinskiy/worklog/internal/domain"
)

// ListProjects returns projects ordered by slug. When includeHidden is false
// archived rows are filtered out.
func (d *DB) ListProjects(includeHidden bool) ([]Project, error) {
	q := `SELECT slug, name, task_prefix, archived FROM projects`
	if !includeHidden {
		q += ` WHERE archived = 0`
	}
	q += ` ORDER BY slug`
	rows, err := d.Query(q)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		var p Project
		var arch int
		if err := rows.Scan(&p.Slug, &p.Name, &p.TaskPrefix, &arch); err != nil {
			return nil, err
		}
		p.Archived = arch != 0
		out = append(out, p)
	}
	return out, rows.Err()
}

// CreateProject inserts a new project. Slug must be unique.
func (d *DB) CreateProject(p Project) error {
	if err := domain.ValidateProject(p); err != nil {
		return err
	}
	_, err := d.Exec(
		`INSERT INTO projects(slug, name, task_prefix, archived) VALUES(?, ?, ?, ?)`,
		p.Slug, p.Name, p.TaskPrefix, boolInt(p.Archived),
	)
	return err
}

// UpdateProject updates a project by old slug. Renaming the slug cascades
// to tasks.project_slug via FK ON UPDATE. Pass the full new state in p.
func (d *DB) UpdateProject(oldSlug string, p Project) error {
	if err := domain.ValidateProject(p); err != nil {
		return err
	}
	_, err := d.Exec(
		`UPDATE projects SET slug = ?, name = ?, task_prefix = ?, archived = ?
		 WHERE slug = ?`,
		p.Slug, p.Name, p.TaskPrefix, boolInt(p.Archived), oldSlug,
	)
	return err
}

// DeleteProject removes a project. Fails if any task still references it.
func (d *DB) DeleteProject(slug string) error {
	_, err := d.Exec(`DELETE FROM projects WHERE slug = ?`, slug)
	return err
}

// SetProjectArchived hides or unhides a project. When cascade is true the
// same archived flag is propagated to all tasks under the project and to
// their logs and activities. With cascade=false only the project row is
// touched (tasks keep their own archived flag).
func (d *DB) SetProjectArchived(slug string, archived, cascade bool) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	v := boolInt(archived)
	if _, err := tx.Exec(`UPDATE projects SET archived = ? WHERE slug = ?`, v, slug); err != nil {
		return err
	}
	if cascade {
		if _, err := tx.Exec(`UPDATE tasks SET archived = ? WHERE project_slug = ?`, v, slug); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE log_entries SET archived = ?
		                      WHERE task_external_id IN
		                        (SELECT external_id FROM tasks WHERE project_slug = ?)`,
			v, slug); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE task_activities SET archived = ?
		                      WHERE task_external_id IN
		                        (SELECT external_id FROM tasks WHERE project_slug = ?)`,
			v, slug); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
