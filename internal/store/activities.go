package store

import (
	"errors"
	"fmt"

	"github.com/snezhinskiy/worklog/internal/domain"
)

// ListActivities returns activities ordered by created_at asc. taskExtID = ""
// returns activities for all tasks. When includeHidden is false, archived
// rows are filtered out (matches projects/tasks/logs convention).
func (d *DB) ListActivities(taskExtID string, includeHidden bool) ([]Activity, error) {
	q := `SELECT id, task_external_id, type, url, text, archived, created_at
	      FROM task_activities`
	var args []any
	conds := []string{}
	if taskExtID != "" {
		conds = append(conds, "task_external_id = ?")
		args = append(args, taskExtID)
	}
	if !includeHidden {
		conds = append(conds, "archived = 0")
	}
	for i, c := range conds {
		if i == 0 {
			q += " WHERE " + c
		} else {
			q += " AND " + c
		}
	}
	q += " ORDER BY created_at, id"

	rows, err := d.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	defer rows.Close()
	var out []Activity
	for rows.Next() {
		var a Activity
		var arch int
		var createdAt string
		if err := rows.Scan(&a.ID, &a.TaskID, &a.Type, &a.URL, &a.Text, &arch, &createdAt); err != nil {
			return nil, err
		}
		a.Archived = arch != 0
		a.CreatedAt = parseTime(createdAt)
		out = append(out, a)
	}
	return out, rows.Err()
}

// CreateActivity inserts an activity. Returns the activity with ID populated.
// Validates type and url-or-text via domain.ValidateActivity; rejects empty
// task_id (the FK would catch it, but the message is friendlier here).
func (d *DB) CreateActivity(a Activity) (Activity, error) {
	if a.TaskID == "" {
		return a, errors.New("activity task_id is required")
	}
	if err := domain.ValidateActivity(a); err != nil {
		return a, err
	}
	res, err := d.Exec(`
		INSERT INTO task_activities(task_external_id, type, url, text, archived)
		VALUES (?, ?, ?, ?, ?)
	`, a.TaskID, a.Type, a.URL, a.Text, boolInt(a.Archived))
	if err != nil {
		return a, err
	}
	id, _ := res.LastInsertId()
	a.ID = id
	return a, nil
}

// UpdateActivity updates type/url/text on an existing activity. Doesn't
// touch archived (use SetActivityArchived) or task_external_id (activities
// don't move between tasks; if needed, hide and recreate).
func (d *DB) UpdateActivity(a Activity) error {
	if a.ID == 0 {
		return errors.New("activity id is required")
	}
	if err := domain.ValidateActivity(a); err != nil {
		return err
	}
	_, err := d.Exec(`
		UPDATE task_activities
		SET type = ?, url = ?, text = ?
		WHERE id = ?
	`, a.Type, a.URL, a.Text, a.ID)
	return err
}

// SetActivityArchived hides or unhides an activity.
func (d *DB) SetActivityArchived(id int64, archived bool) error {
	_, err := d.Exec(`UPDATE task_activities SET archived = ? WHERE id = ?`,
		boolInt(archived), id)
	return err
}

// DeleteActivity removes an activity outright. Most callers should use
// SetActivityArchived for the soft-hide behaviour; this is here for tests
// and one-shot cleanup.
func (d *DB) DeleteActivity(id int64) error {
	_, err := d.Exec(`DELETE FROM task_activities WHERE id = ?`, id)
	return err
}
