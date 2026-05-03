package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/snezhinskiy/worklog/internal/domain"
)

// DB is a thin wrapper over *sql.DB carrying the file path for diagnostics.
type DB struct {
	*sql.DB
	Path string
}

// Compile-time check: *DB satisfies the domain.Store contract every UI
// layer depends on. If this stops compiling, the interface and the
// implementation have drifted.
var _ domain.Store = (*DB)(nil)

// Open opens (creating dirs if needed) the SQLite database at path, applies
// the schema, and returns a ready-to-use handle. WAL is enabled so reads
// don't block during writes; FK is on so cascades work.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)&_pragma=busy_timeout(5000)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping %s: %w", path, err)
	}
	d := &DB{DB: sqlDB, Path: path}
	if err := d.migrate(); err != nil {
		return nil, err
	}
	return d, nil
}

// DefaultPath returns ~/.local/share/worklog/worklog.db (XDG-aware).
func DefaultPath() string {
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "worklog", "worklog.db")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "worklog.db"
	}
	return filepath.Join(home, ".local", "share", "worklog", "worklog.db")
}

func (d *DB) migrate() error {
	if _, err := d.Exec(schemaSQL); err != nil {
		return fmt.Errorf("migrate (schema): %w", err)
	}
	for _, stmt := range migrationsSQL {
		if _, err := d.Exec(stmt); err != nil {
			// SQLite ALTER TABLE ADD COLUMN errors with "duplicate column
			// name" if the column already exists; that's the idempotent
			// case for DBs created after the column was baked into schema.
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			return fmt.Errorf("migrate (alter): %w", err)
		}
	}
	return nil
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS projects (
    slug        TEXT PRIMARY KEY,
    name        TEXT NOT NULL DEFAULT '',
    task_prefix TEXT NOT NULL DEFAULT '',
    archived    INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS tasks (
    external_id       TEXT PRIMARY KEY,
    project_slug      TEXT NOT NULL REFERENCES projects(slug) ON UPDATE CASCADE,
    status            TEXT NOT NULL DEFAULT 'todo',
    short             TEXT NOT NULL,
    archived          INTEGER NOT NULL DEFAULT 0,
    status_changed_at TEXT NOT NULL DEFAULT (datetime('now')),
    created_at        TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at        TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project_slug);
CREATE INDEX IF NOT EXISTS idx_tasks_status  ON tasks(status);

CREATE TABLE IF NOT EXISTS log_entries (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    task_external_id TEXT NOT NULL REFERENCES tasks(external_id) ON UPDATE CASCADE ON DELETE CASCADE,
    date             TEXT NOT NULL,
    time             TEXT NOT NULL DEFAULT '',
    hours            REAL NOT NULL,
    note             TEXT NOT NULL DEFAULT '',
    archived         INTEGER NOT NULL DEFAULT 0,
    created_at       TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_logs_task ON log_entries(task_external_id);
CREATE INDEX IF NOT EXISTS idx_logs_date ON log_entries(date);

CREATE TABLE IF NOT EXISTS task_activities (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    task_external_id TEXT NOT NULL REFERENCES tasks(external_id) ON UPDATE CASCADE ON DELETE CASCADE,
    type             TEXT NOT NULL,
    url              TEXT NOT NULL DEFAULT '',
    text             TEXT NOT NULL DEFAULT '',
    archived         INTEGER NOT NULL DEFAULT 0,
    created_at       TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_activities_task ON task_activities(task_external_id);
`

// migrationsSQL is applied after the canonical CREATE TABLEs above. Each entry
// is run unconditionally; "duplicate column name" errors are swallowed so
// re-runs against a fresh DB (where the column is already in CREATE TABLE)
// are no-ops.
var migrationsSQL = []string{
	`ALTER TABLE log_entries ADD COLUMN archived INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE projects ADD COLUMN task_prefix TEXT NOT NULL DEFAULT ''`,
}

// IsEmpty reports whether the DB has no projects yet — used to decide whether
// to seed demo data on first run.
func (d *DB) IsEmpty() (bool, error) {
	var n int
	if err := d.QueryRow(`SELECT count(*) FROM projects`).Scan(&n); err != nil {
		return false, err
	}
	return n == 0, nil
}
