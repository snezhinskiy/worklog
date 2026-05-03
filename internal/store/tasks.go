package store

import (
	"fmt"
	"time"

	"github.com/snezhinskiy/worklog/internal/domain"
)

// ListTasks returns tasks ordered by external_id. When includeHidden is false
// archived rows are filtered out; pass true to include them (e.g. for the
// unhide picker).
func (d *DB) ListTasks(includeHidden bool) ([]Task, error) {
	q := `SELECT project_slug, external_id, status, short, archived, status_changed_at
	      FROM tasks`
	if !includeHidden {
		q += ` WHERE archived = 0`
	}
	q += ` ORDER BY external_id`
	rows, err := d.Query(q)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()
	var out []Task
	for rows.Next() {
		var t Task
		var arch int
		var changedAt string
		if err := rows.Scan(&t.Project, &t.ExternalID, &t.Status, &t.Short, &arch, &changedAt); err != nil {
			return nil, err
		}
		t.Archived = arch != 0
		t.StatusChangedAt = parseTime(changedAt)
		out = append(out, t)
	}
	return out, rows.Err()
}

// CreateTask inserts a task. If ExternalID is empty, an ID is generated as
// "{PROJECT_SLUG}-{n}" where n is one greater than the largest existing
// numeric suffix for that project (or 1 if none).
func (d *DB) CreateTask(t Task) (Task, error) {
	if err := domain.ValidateTask(t); err != nil {
		return t, err
	}
	if t.Status == "" {
		t.Status = "todo"
	}
	if t.ExternalID == "" {
		next, err := d.nextExtID(t.Project)
		if err != nil {
			return t, err
		}
		t.ExternalID = next
	}
	if t.StatusChangedAt.IsZero() {
		t.StatusChangedAt = time.Now()
	}
	_, err := d.Exec(`
		INSERT INTO tasks(external_id, project_slug, status, short, archived, status_changed_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, t.ExternalID, t.Project, t.Status, t.Short, boolInt(t.Archived), formatTime(t.StatusChangedAt))
	return t, err
}

// UpdateTask updates fields by old external_id. If newExtID differs, it's a
// rename — log_entries.task_external_id cascades via ON UPDATE.
func (d *DB) UpdateTask(oldExtID string, t Task) error {
	if t.ExternalID == "" {
		t.ExternalID = oldExtID
	}
	if err := domain.ValidateTask(t); err != nil {
		return err
	}
	_, err := d.Exec(`
		UPDATE tasks
		SET external_id = ?, project_slug = ?, status = ?, short = ?, archived = ?,
		    status_changed_at = ?, updated_at = datetime('now')
		WHERE external_id = ?
	`, t.ExternalID, t.Project, t.Status, t.Short, boolInt(t.Archived),
		formatTime(t.StatusChangedAt), oldExtID)
	return err
}

// SetTaskStatus updates only the status (and bumps StatusChangedAt to now).
// Used by the board's shift+arrow / s shortcut.
func (d *DB) SetTaskStatus(extID, newStatus string) error {
	_, err := d.Exec(`
		UPDATE tasks SET status = ?, status_changed_at = datetime('now'),
		    updated_at = datetime('now')
		WHERE external_id = ?
	`, newStatus, extID)
	return err
}

// DeleteTask removes a task. Cascades log_entries via FK ON DELETE.
func (d *DB) DeleteTask(extID string) error {
	_, err := d.Exec(`DELETE FROM tasks WHERE external_id = ?`, extID)
	return err
}

// SetTaskArchived hides or unhides a task. When cascade is true the same
// archived flag is propagated to the task's logs and activities — the
// natural "make it go away (or come back)" semantics. With cascade=false
// only the task row is touched.
func (d *DB) SetTaskArchived(extID string, archived, cascade bool) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	v := boolInt(archived)
	if _, err := tx.Exec(`UPDATE tasks SET archived = ?, updated_at = datetime('now')
	                      WHERE external_id = ?`, v, extID); err != nil {
		return err
	}
	if cascade {
		if _, err := tx.Exec(`UPDATE log_entries SET archived = ?
		                      WHERE task_external_id = ?`, v, extID); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE task_activities SET archived = ?
		                      WHERE task_external_id = ?`, v, extID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// nextExtID generates the next task id for a project. The prefix is the
// project's task_prefix when set (e.g. "WL"), otherwise the slug itself.
// Picks the max numeric suffix already in use under that prefix and adds one.
func (d *DB) nextExtID(projectSlug string) (string, error) {
	var prefix string
	err := d.QueryRow(`SELECT COALESCE(NULLIF(task_prefix, ''), slug)
	                   FROM projects WHERE slug = ?`, projectSlug).Scan(&prefix)
	if err != nil {
		return "", fmt.Errorf("look up task_prefix for %s: %w", projectSlug, err)
	}
	rows, err := d.Query(`
		SELECT external_id FROM tasks
		WHERE project_slug = ? AND external_id GLOB ? || '-*'
	`, projectSlug, prefix)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	max := 0
	for rows.Next() {
		var ext string
		if err := rows.Scan(&ext); err != nil {
			return "", err
		}
		var n int
		if _, err := fmt.Sscanf(ext, prefix+"-%d", &n); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("%s-%d", prefix, max+1), nil
}

const sqliteTimeLayout = "2006-01-02 15:04:05"

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	// formatTime always writes UTC, so read it back as UTC. Using time.Local
	// would silently misinterpret stored timestamps by the TZ offset.
	if t, err := time.ParseInLocation(sqliteTimeLayout, s, time.UTC); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	return time.Time{}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return time.Now().UTC().Format(sqliteTimeLayout)
	}
	return t.UTC().Format(sqliteTimeLayout)
}
