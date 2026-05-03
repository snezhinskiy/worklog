// Package mcpsrv exposes the worklog store over the Model Context Protocol so
// an LLM (typically Claude) can read and write entries by name. The DTOs here
// live in their own package so the wire shape is decoupled from the storage
// layer's Go-typed time.Time fields.
package mcpsrv

import (
	"time"

	"github.com/snezhinskiy/worklog/internal/store"
)

const dateLayout = "2006-01-02"

// ProjectDTO is the wire shape for projects exposed via MCP.
type ProjectDTO struct {
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	TaskPrefix string `json:"task_prefix,omitempty"`
	Archived   bool   `json:"archived"`
}

// TaskDTO is the wire shape for tasks exposed via MCP. StatusChangedAt is
// emitted as an RFC3339 timestamp so the LLM can reason about staleness.
type TaskDTO struct {
	Project         string `json:"project"`
	ExternalID      string `json:"external_id"`
	Status          string `json:"status"`
	Short           string `json:"short"`
	Archived        bool   `json:"archived"`
	StatusChangedAt string `json:"status_changed_at,omitempty"`
}

// LogDTO is the wire shape for log entries. Date is YYYY-MM-DD, Time is HH:MM.
type LogDTO struct {
	ID     int64   `json:"id"`
	TaskID string  `json:"task_id"`
	Date   string  `json:"date"`
	Time   string  `json:"time"`
	Hours  float64 `json:"hours"`
	Note   string  `json:"note"`
}

func projectToDTO(p store.Project) ProjectDTO {
	return ProjectDTO{Slug: p.Slug, Name: p.Name, TaskPrefix: p.TaskPrefix, Archived: p.Archived}
}

func taskToDTO(t store.Task) TaskDTO {
	d := TaskDTO{
		Project:    t.Project,
		ExternalID: t.ExternalID,
		Status:     t.Status,
		Short:      t.Short,
		Archived:   t.Archived,
	}
	if !t.StatusChangedAt.IsZero() {
		d.StatusChangedAt = t.StatusChangedAt.UTC().Format(time.RFC3339)
	}
	return d
}

func logToDTO(l store.LogEntry) LogDTO {
	return LogDTO{
		ID:     l.ID,
		TaskID: l.TaskID,
		Date:   l.Date.Format(dateLayout),
		Time:   l.Time,
		Hours:  l.Hours,
		Note:   l.Note,
	}
}
