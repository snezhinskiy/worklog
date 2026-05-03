// Package store is the persistence layer for worklog. The data shapes
// (Project, Task, LogEntry, Activity) live in internal/domain — store
// re-exports them here so existing call-sites (store.Task, etc.) keep
// compiling. SQLite-backed, no CGO.
package store

import "github.com/snezhinskiy/worklog/internal/domain"

type (
	Project  = domain.Project
	Task     = domain.Task
	LogEntry = domain.LogEntry
	Activity = domain.Activity
)

// ActivityTypes is re-exported for compatibility; new code should reach
// for domain.ActivityTypes directly.
var ActivityTypes = domain.ActivityTypes
