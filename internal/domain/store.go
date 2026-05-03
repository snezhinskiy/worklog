package domain

// Store is the persistence contract every UI layer (TUI, CLI, MCP) talks to.
// The concrete implementation lives in internal/store; tests can substitute
// any in-memory or fake type that satisfies these methods.
//
// Method semantics:
//   - List*(includeHidden bool): when false, archived rows are filtered out;
//     when true, the full set is returned (used by unhide pickers).
//   - Set*Archived: soft-hide (a flag), reversible. The cascade variants
//     propagate the flag to children inside a transaction.
//   - Delete*: hard removal, irreversible. Tasks cascade-delete their logs
//     and activities; projects refuse to delete while tasks reference them.
type Store interface {
	// Lifecycle / utility
	IsEmpty() (bool, error)

	// Projects
	ListProjects(includeHidden bool) ([]Project, error)
	CreateProject(p Project) error
	UpdateProject(oldSlug string, p Project) error
	SetProjectArchived(slug string, archived, cascade bool) error
	DeleteProject(slug string) error

	// Tasks
	ListTasks(includeHidden bool) ([]Task, error)
	CreateTask(t Task) (Task, error)
	UpdateTask(oldExtID string, t Task) error
	SetTaskStatus(extID, newStatus string) error
	SetTaskArchived(extID string, archived, cascade bool) error
	DeleteTask(extID string) error

	// Logs
	ListLogs(includeHidden bool) ([]LogEntry, error)
	CreateLog(e LogEntry) (LogEntry, error)
	UpdateLog(e LogEntry) error
	SetLogArchived(id int64, archived bool) error
	DeleteLog(id int64) error

	// Activities
	ListActivities(taskExtID string, includeHidden bool) ([]Activity, error)
	CreateActivity(a Activity) (Activity, error)
	UpdateActivity(a Activity) error
	SetActivityArchived(id int64, archived bool) error
	DeleteActivity(id int64) error
}
