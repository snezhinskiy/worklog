package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/snezhinskiy/worklog/internal/domain"
)

// newEditLogForm builds the edit-log form (/log edit). Allows moving a log to
// a different task (the task picker shows all tasks across projects).
func newEditLogForm(m *Model, logIdx int) *cmdForm {
	if logIdx < 0 || logIdx >= len(m.logs) {
		return nil
	}
	l := m.logs[logIdx]

	tasks := append([]Task(nil), m.tasks...)
	taskOpts := make([]string, len(tasks))
	initialTask := 0
	for i, t := range tasks {
		taskOpts[i] = t.ExternalID + " — " + t.Short
		if t.ExternalID == l.TaskID {
			initialTask = i
		}
	}

	taskField := newChoiceField("Task", taskOpts, initialTask, func(idx int) string {
		t := tasks[idx]
		return statusStyle[t.Status].Render(statusIcon[t.Status]) + "  " +
			lipgloss.NewStyle().Foreground(colBright).Render(t.ExternalID) + "  " +
			text.Render(t.Short)
	})

	form := &cmdForm{
		title: fmt.Sprintf("Edit log · %s · %s  ·  /log edit", l.TaskID, l.Date.Format("2006-01-02")),
		fields: []*formField{
			taskField,
			newTextField("Date", l.Date.Format("2006-01-02"), 12),
			newTextField("Time", l.Time, 6),
			newTextField("Hours", strconv.FormatFloat(l.Hours, 'f', -1, 64), 6),
			newTextField("Note", l.Note, 60),
		},
		onSubmit: func(m *Model, f *cmdForm) error {
			ti := f.fields[0].idx
			newTask := tasks[ti].ExternalID
			date, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(f.fields[1].text.Value()), m.today.Location())
			if err != nil {
				return fmt.Errorf("date: %w", err)
			}
			tm := strings.TrimSpace(f.fields[2].text.Value())
			h, err := domain.ParseHours(f.fields[3].text.Value())
			if err != nil {
				return err
			}
			note := strings.TrimSpace(f.fields[4].text.Value())
			updated := LogEntry{
				ID:     m.logs[logIdx].ID,
				TaskID: newTask,
				Date:   date,
				Time:   tm,
				Hours:  h,
				Note:   note,
			}
			if m.db != nil {
				if err := m.db.UpdateLog(updated); err != nil {
					return err
				}
				return m.reload()
			}
			m.logs[logIdx] = updated
			return nil
		},
	}
	form.refocusInputs()
	return form
}

// newLogForm builds the create-log form (/log). The task choice is scoped to
// open tasks of the selected project — flipping projects refreshes the list
// via the onChange hook.
func newLogForm(m *Model) *cmdForm {
	projects := projectSlugs(m)
	initialProj := 0
	if m.projectFilter > 0 {
		initialProj = m.projectFilter - 1
	}

	projField := newChoiceField("Project", projects, initialProj, func(idx int) string {
		return projectColor(projects[idx]).Render(projects[idx])
	})

	tasks := openTasksOfProject(m, projects[initialProj])
	taskField := newChoiceField("Task", taskLabels(tasks), 0, func(idx int) string {
		if idx >= len(tasks) {
			return muted.Render("(no tasks — create via /task)")
		}
		t := tasks[idx]
		return statusStyle[t.Status].Render(statusIcon[t.Status]) + "  " +
			lipgloss.NewStyle().Foreground(colBright).Render(t.ExternalID) + "  " +
			text.Render(t.Short)
	})

	durField := newTextField("Time", "1h", 8)
	dateField := newTextField("Date", m.today.Format("2006-01-02"), 12)
	noteField := newTextField("Note", "", 60)

	// When project changes, refresh task choices.
	projField.onChange = func(m *Model, f *cmdForm) {
		newTasks := openTasksOfProject(m, projects[projField.idx])
		taskField.options = taskLabels(newTasks)
		taskField.idx = 0
		taskField.display = func(idx int) string {
			if idx >= len(newTasks) {
				return muted.Render("(no tasks)")
			}
			t := newTasks[idx]
			return statusStyle[t.Status].Render(statusIcon[t.Status]) + "  " +
				lipgloss.NewStyle().Foreground(colBright).Render(t.ExternalID) + "  " +
				text.Render(t.Short)
		}
	}

	form := &cmdForm{
		title: "Log work  ·  /log",
		fields: []*formField{projField, taskField, durField, dateField, noteField},
		onSubmit: func(m *Model, f *cmdForm) error {
			projIdx := f.fields[0].idx
			currentTasks := openTasksOfProject(m, projects[projIdx])
			taskIdx := f.fields[1].idx
			if taskIdx >= len(currentTasks) {
				return fmt.Errorf("task not selected")
			}
			task := currentTasks[taskIdx]

			hours, err := domain.ParseHours(f.fields[2].text.Value())
			if err != nil {
				return err
			}
			date, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(f.fields[3].text.Value()), m.today.Location())
			if err != nil {
				return fmt.Errorf("date: %w", err)
			}
			note := strings.TrimSpace(f.fields[4].text.Value())

			e := LogEntry{
				TaskID: task.ExternalID,
				Date:   date,
				Time:   time.Now().Format("15:04"),
				Hours:  hours,
				Note:   note,
			}
			if m.db != nil {
				if _, err := m.db.CreateLog(e); err != nil {
					return err
				}
				return m.reload()
			}
			m.logs = append(m.logs, e)
			return nil
		},
	}
	form.refocusInputs()
	return form
}
