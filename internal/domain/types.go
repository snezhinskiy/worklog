// Package domain holds the pure-Go primitives every other layer (store,
// tui, mcpsrv, cli) reads and writes. No persistence concerns, no I/O —
// just the data shape and the invariants that hold across all three UIs.
package domain

import "time"

// Project is the top-level grouping. Slug is the user-facing primary key;
// TaskPrefix overrides the auto-generated task id prefix (so a project with
// Slug="WORKLOG" and TaskPrefix="WL" yields task ids WL-1, WL-2, …). When
// empty, the slug is used as the prefix.
type Project struct {
	Slug       string
	Name       string
	TaskPrefix string
	Archived   bool
}

// Task is a unit of work. ExternalID is the user-facing primary key (think
// Jira ticket); auto-generated as "{prefix}-{n}" when the user leaves it
// blank, where prefix is the project's TaskPrefix or Slug.
type Task struct {
	Project         string // project slug
	ExternalID      string
	Status          string
	Short           string
	Archived        bool
	StatusChangedAt time.Time
}

// LogEntry is one stamp of work on a task. ID is the storage row id — the
// stable handle the TUI uses to address a single entry for edit/hide.
type LogEntry struct {
	ID       int64
	TaskID   string // task external_id (matches Task.ExternalID)
	Date     time.Time
	Time     string // HH:MM
	Hours    float64
	Note     string
	Archived bool
}

// Activity is a typed event recorded against a task — a merge request link,
// a commit reference, a deploy, etc. Unlike LogEntry, activities don't
// carry hours and aren't summed into reports; they're a parallel timeline
// for "what happened" alongside "how long it took".
type Activity struct {
	ID        int64
	TaskID    string // task external_id
	Type      string // one of ActivityTypes
	URL       string
	Text      string
	Archived  bool
	CreatedAt time.Time
}

// ActivityTypes is the canonical set the UI/CLI/MCP advertise. Validation
// against this list lives in IsActivityType / ValidateActivity.
var ActivityTypes = []string{"mr", "commit", "deploy", "link", "note"}
