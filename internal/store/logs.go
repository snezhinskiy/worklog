package store

import (
	"errors"
	"fmt"
	"time"

	"github.com/snezhinskiy/worklog/internal/domain"
)

const dateLayout = "2006-01-02"

// ListLogs returns log entries ordered by date asc, time asc. When
// includeHidden is false (the default for read-paths), archived entries are
// filtered out so they don't appear in totals, search, or pickers.
func (d *DB) ListLogs(includeHidden bool) ([]LogEntry, error) {
	q := `SELECT id, task_external_id, date, time, hours, note, archived
	      FROM log_entries`
	if !includeHidden {
		q += ` WHERE archived = 0`
	}
	q += ` ORDER BY date, time, id`
	rows, err := d.Query(q)
	if err != nil {
		return nil, fmt.Errorf("list logs: %w", err)
	}
	defer rows.Close()
	var out []LogEntry
	for rows.Next() {
		var e LogEntry
		var dateStr string
		var arch int
		if err := rows.Scan(&e.ID, &e.TaskID, &dateStr, &e.Time, &e.Hours, &e.Note, &arch); err != nil {
			return nil, err
		}
		if t, err := time.ParseInLocation(dateLayout, dateStr, time.Local); err == nil {
			e.Date = t
		}
		e.Archived = arch != 0
		out = append(out, e)
	}
	return out, rows.Err()
}

// CreateLog inserts a log entry. Returns the entry with ID populated.
func (d *DB) CreateLog(e LogEntry) (LogEntry, error) {
	if err := domain.ValidateLog(e); err != nil {
		return e, err
	}
	if e.Date.IsZero() {
		e.Date = time.Now()
	}
	if e.Time == "" {
		e.Time = time.Now().Format("15:04")
	}
	res, err := d.Exec(`
		INSERT INTO log_entries(task_external_id, date, time, hours, note, archived)
		VALUES (?, ?, ?, ?, ?, ?)
	`, e.TaskID, e.Date.Format(dateLayout), e.Time, e.Hours, e.Note, boolInt(e.Archived))
	if err != nil {
		return e, err
	}
	id, _ := res.LastInsertId()
	e.ID = id
	return e, nil
}

// UpdateLog updates all editable fields by id.
func (d *DB) UpdateLog(e LogEntry) error {
	if e.ID == 0 {
		return errors.New("log id is required")
	}
	if err := domain.ValidateLog(e); err != nil {
		return err
	}
	_, err := d.Exec(`
		UPDATE log_entries
		SET task_external_id = ?, date = ?, time = ?, hours = ?, note = ?, archived = ?
		WHERE id = ?
	`, e.TaskID, e.Date.Format(dateLayout), e.Time, e.Hours, e.Note, boolInt(e.Archived), e.ID)
	return err
}

// DeleteLog removes one entry by id.
func (d *DB) DeleteLog(id int64) error {
	_, err := d.Exec(`DELETE FROM log_entries WHERE id = ?`, id)
	return err
}

// SetLogArchived hides or unhides a log entry. Hidden entries stay in the DB
// and remain visible to includeHidden=true callers (e.g. the unhide picker)
// but are excluded from default reads.
func (d *DB) SetLogArchived(id int64, archived bool) error {
	_, err := d.Exec(`UPDATE log_entries SET archived = ? WHERE id = ?`,
		boolInt(archived), id)
	return err
}
