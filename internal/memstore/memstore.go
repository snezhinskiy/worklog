// Package memstore is an in-memory implementation of domain.Store. It exists
// so tests (and the snapshot/--demo path, in time) can exercise the upper
// layers without spinning up SQLite. Behaviour mirrors the real store where
// it would be observable from outside (validation, cascade hide, FK refusals
// for project_delete, etc.); shortcuts are taken on things only the SQL
// layer would care about (no migrations, no WAL, no transactions).
package memstore

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/snezhinskiy/worklog/internal/domain"
)

type Store struct {
	projects   []domain.Project
	tasks      []domain.Task
	logs       []domain.LogEntry
	activities []domain.Activity
	nextLog    int64
	nextAct    int64
}

func New() *Store { return &Store{nextLog: 1, nextAct: 1} }

// Compile-time satisfaction check.
var _ domain.Store = (*Store)(nil)

func (s *Store) IsEmpty() (bool, error) { return len(s.projects) == 0, nil }

// ── projects ────────────────────────────────────────────────────────────────

func (s *Store) ListProjects(includeHidden bool) ([]domain.Project, error) {
	out := make([]domain.Project, 0, len(s.projects))
	for _, p := range s.projects {
		if !includeHidden && p.Archived {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

func (s *Store) CreateProject(p domain.Project) error {
	if err := domain.ValidateProject(p); err != nil {
		return err
	}
	for _, ex := range s.projects {
		if ex.Slug == p.Slug {
			return fmt.Errorf("project %s already exists", p.Slug)
		}
	}
	s.projects = append(s.projects, p)
	return nil
}

func (s *Store) UpdateProject(oldSlug string, p domain.Project) error {
	if err := domain.ValidateProject(p); err != nil {
		return err
	}
	for i := range s.projects {
		if s.projects[i].Slug == oldSlug {
			s.projects[i] = p
			if oldSlug != p.Slug {
				for j := range s.tasks {
					if s.tasks[j].Project == oldSlug {
						s.tasks[j].Project = p.Slug
					}
				}
			}
			return nil
		}
	}
	return fmt.Errorf("project %s not found", oldSlug)
}

func (s *Store) SetProjectArchived(slug string, archived, cascade bool) error {
	found := false
	for i := range s.projects {
		if s.projects[i].Slug == slug {
			s.projects[i].Archived = archived
			found = true
		}
	}
	if !found {
		return fmt.Errorf("project %s not found", slug)
	}
	if !cascade {
		return nil
	}
	for i := range s.tasks {
		if s.tasks[i].Project == slug {
			s.tasks[i].Archived = archived
			ext := s.tasks[i].ExternalID
			for j := range s.logs {
				if s.logs[j].TaskID == ext {
					s.logs[j].Archived = archived
				}
			}
			for j := range s.activities {
				if s.activities[j].TaskID == ext {
					s.activities[j].Archived = archived
				}
			}
		}
	}
	return nil
}

func (s *Store) DeleteProject(slug string) error {
	for _, t := range s.tasks {
		if t.Project == slug {
			return fmt.Errorf("project %s still has tasks; delete or reassign first", slug)
		}
	}
	for i := range s.projects {
		if s.projects[i].Slug == slug {
			s.projects = append(s.projects[:i], s.projects[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("project %s not found", slug)
}

// ── tasks ───────────────────────────────────────────────────────────────────

func (s *Store) ListTasks(includeHidden bool) ([]domain.Task, error) {
	out := make([]domain.Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		if !includeHidden && t.Archived {
			continue
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ExternalID < out[j].ExternalID })
	return out, nil
}

func (s *Store) CreateTask(t domain.Task) (domain.Task, error) {
	if err := domain.ValidateTask(t); err != nil {
		return t, err
	}
	if t.Status == "" {
		t.Status = "todo"
	}
	if t.ExternalID == "" {
		var prefix string
		for _, p := range s.projects {
			if p.Slug == t.Project {
				prefix = p.TaskPrefix
				if prefix == "" {
					prefix = p.Slug
				}
				break
			}
		}
		if prefix == "" {
			return t, fmt.Errorf("project %s not found", t.Project)
		}
		max := 0
		for _, ex := range s.tasks {
			var n int
			if _, err := fmt.Sscanf(ex.ExternalID, prefix+"-%d", &n); err == nil && n > max {
				max = n
			}
		}
		t.ExternalID = fmt.Sprintf("%s-%d", prefix, max+1)
	}
	if t.StatusChangedAt.IsZero() {
		t.StatusChangedAt = time.Now()
	}
	s.tasks = append(s.tasks, t)
	return t, nil
}

func (s *Store) UpdateTask(oldExtID string, t domain.Task) error {
	if t.ExternalID == "" {
		t.ExternalID = oldExtID
	}
	if err := domain.ValidateTask(t); err != nil {
		return err
	}
	for i := range s.tasks {
		if s.tasks[i].ExternalID == oldExtID {
			s.tasks[i] = t
			if oldExtID != t.ExternalID {
				for j := range s.logs {
					if s.logs[j].TaskID == oldExtID {
						s.logs[j].TaskID = t.ExternalID
					}
				}
				for j := range s.activities {
					if s.activities[j].TaskID == oldExtID {
						s.activities[j].TaskID = t.ExternalID
					}
				}
			}
			return nil
		}
	}
	return fmt.Errorf("task %s not found", oldExtID)
}

func (s *Store) SetTaskStatus(extID, newStatus string) error {
	for i := range s.tasks {
		if s.tasks[i].ExternalID == extID {
			s.tasks[i].Status = newStatus
			s.tasks[i].StatusChangedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("task %s not found", extID)
}

func (s *Store) SetTaskArchived(extID string, archived, cascade bool) error {
	found := false
	for i := range s.tasks {
		if s.tasks[i].ExternalID == extID {
			s.tasks[i].Archived = archived
			found = true
		}
	}
	if !found {
		return fmt.Errorf("task %s not found", extID)
	}
	if !cascade {
		return nil
	}
	for i := range s.logs {
		if s.logs[i].TaskID == extID {
			s.logs[i].Archived = archived
		}
	}
	for i := range s.activities {
		if s.activities[i].TaskID == extID {
			s.activities[i].Archived = archived
		}
	}
	return nil
}

func (s *Store) DeleteTask(extID string) error {
	for i := range s.tasks {
		if s.tasks[i].ExternalID == extID {
			s.tasks = append(s.tasks[:i], s.tasks[i+1:]...)
			// Cascade: drop logs and activities (mirrors FK ON DELETE).
			s.logs = filterLogs(s.logs, func(l domain.LogEntry) bool { return l.TaskID != extID })
			s.activities = filterActs(s.activities, func(a domain.Activity) bool { return a.TaskID != extID })
			return nil
		}
	}
	return fmt.Errorf("task %s not found", extID)
}

// ── logs ────────────────────────────────────────────────────────────────────

func (s *Store) ListLogs(includeHidden bool) ([]domain.LogEntry, error) {
	out := make([]domain.LogEntry, 0, len(s.logs))
	for _, l := range s.logs {
		if !includeHidden && l.Archived {
			continue
		}
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Date.Equal(out[j].Date) {
			return out[i].Date.Before(out[j].Date)
		}
		if out[i].Time != out[j].Time {
			return out[i].Time < out[j].Time
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (s *Store) CreateLog(e domain.LogEntry) (domain.LogEntry, error) {
	if err := domain.ValidateLog(e); err != nil {
		return e, err
	}
	if e.Date.IsZero() {
		e.Date = time.Now()
	}
	if e.Time == "" {
		e.Time = time.Now().Format("15:04")
	}
	e.ID = s.nextLog
	s.nextLog++
	s.logs = append(s.logs, e)
	return e, nil
}

func (s *Store) UpdateLog(e domain.LogEntry) error {
	if e.ID == 0 {
		return errors.New("log id is required")
	}
	if err := domain.ValidateLog(e); err != nil {
		return err
	}
	for i := range s.logs {
		if s.logs[i].ID == e.ID {
			s.logs[i] = e
			return nil
		}
	}
	return fmt.Errorf("log %d not found", e.ID)
}

func (s *Store) SetLogArchived(id int64, archived bool) error {
	for i := range s.logs {
		if s.logs[i].ID == id {
			s.logs[i].Archived = archived
			return nil
		}
	}
	return fmt.Errorf("log %d not found", id)
}

func (s *Store) DeleteLog(id int64) error {
	for i := range s.logs {
		if s.logs[i].ID == id {
			s.logs = append(s.logs[:i], s.logs[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("log %d not found", id)
}

// ── activities ──────────────────────────────────────────────────────────────

func (s *Store) ListActivities(taskExtID string, includeHidden bool) ([]domain.Activity, error) {
	out := make([]domain.Activity, 0, len(s.activities))
	for _, a := range s.activities {
		if taskExtID != "" && a.TaskID != taskExtID {
			continue
		}
		if !includeHidden && a.Archived {
			continue
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (s *Store) CreateActivity(a domain.Activity) (domain.Activity, error) {
	if strings.TrimSpace(a.TaskID) == "" {
		return a, errors.New("activity task_id is required")
	}
	if err := domain.ValidateActivity(a); err != nil {
		return a, err
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now()
	}
	a.ID = s.nextAct
	s.nextAct++
	s.activities = append(s.activities, a)
	return a, nil
}

func (s *Store) UpdateActivity(a domain.Activity) error {
	if a.ID == 0 {
		return errors.New("activity id is required")
	}
	if err := domain.ValidateActivity(a); err != nil {
		return err
	}
	for i := range s.activities {
		if s.activities[i].ID == a.ID {
			// Don't move between tasks; mirrors store contract.
			a.TaskID = s.activities[i].TaskID
			a.CreatedAt = s.activities[i].CreatedAt
			s.activities[i] = a
			return nil
		}
	}
	return fmt.Errorf("activity %d not found", a.ID)
}

func (s *Store) SetActivityArchived(id int64, archived bool) error {
	for i := range s.activities {
		if s.activities[i].ID == id {
			s.activities[i].Archived = archived
			return nil
		}
	}
	return fmt.Errorf("activity %d not found", id)
}

func (s *Store) DeleteActivity(id int64) error {
	for i := range s.activities {
		if s.activities[i].ID == id {
			s.activities = append(s.activities[:i], s.activities[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("activity %d not found", id)
}

func filterLogs(in []domain.LogEntry, keep func(domain.LogEntry) bool) []domain.LogEntry {
	out := in[:0]
	for _, x := range in {
		if keep(x) {
			out = append(out, x)
		}
	}
	return out
}

func filterActs(in []domain.Activity, keep func(domain.Activity) bool) []domain.Activity {
	out := in[:0]
	for _, x := range in {
		if keep(x) {
			out = append(out, x)
		}
	}
	return out
}
