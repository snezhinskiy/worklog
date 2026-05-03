package tui

import (
	"fmt"
	"strings"
)

// newTaskForm builds the create-task form (/task create). Pre-selects the
// active project filter (if any) so logging into the current view's project
// is a one-step flow.
func newTaskForm(m *Model) *cmdForm {
	projects := projectSlugs(m)
	initialProj := 0
	if m.projectFilter > 0 {
		initialProj = m.projectFilter - 1
	}

	projField := newChoiceField("Project", projects, initialProj, func(idx int) string {
		return projectColor(projects[idx]).Render(projects[idx])
	})
	statusField := newChoiceField("Status", statusOrder, 0, func(idx int) string {
		return statusStyle[statusOrder[idx]].Render(statusIcon[statusOrder[idx]] + " " + statusOrder[idx])
	})

	form := &cmdForm{
		title: "New task  ·  /task create",
		fields: []*formField{
			projField,
			newTextField("ID", "", 12),
			newTextField("Title", "", 60),
			statusField,
		},
		onSubmit: func(m *Model, f *cmdForm) error {
			projIdx := f.fields[0].idx
			extID := strings.TrimSpace(f.fields[1].text.Value())
			short := strings.TrimSpace(f.fields[2].text.Value())
			statusIdx := f.fields[3].idx
			if short == "" {
				return fmt.Errorf("title is required")
			}
			t := Task{
				Project:         m.projects[projIdx].Slug,
				ExternalID:      extID,
				Status:          statusOrder[statusIdx],
				Short:           short,
				StatusChangedAt: m.today,
			}
			if m.db != nil {
				if _, err := m.db.CreateTask(t); err != nil {
					return err
				}
				return m.reload()
			}
			m.tasks = append(m.tasks, t)
			return nil
		},
	}
	form.refocusInputs()
	return form
}

// newEditTaskForm builds the edit-task form (/task edit). Pre-fills from the
// current task; renaming the external_id is allowed and cascades to logs.
func newEditTaskForm(m *Model, taskExtID string) *cmdForm {
	tk, ok := m.taskByID(taskExtID)
	if !ok {
		return nil
	}
	projects := projectSlugs(m)
	initialProj := 0
	for i, p := range projects {
		if p == tk.Project {
			initialProj = i
			break
		}
	}

	projField := newChoiceField("Project", projects, initialProj, func(idx int) string {
		return projectColor(projects[idx]).Render(projects[idx])
	})
	statusField := newChoiceField("Status", statusOrder, statusIndex(tk.Status), func(idx int) string {
		return statusStyle[statusOrder[idx]].Render(statusIcon[statusOrder[idx]] + " " + statusOrder[idx])
	})

	form := &cmdForm{
		title: "Edit task " + taskExtID + "  ·  /task edit",
		fields: []*formField{
			projField,
			newTextField("ID", tk.ExternalID, 12),
			newTextField("Title", tk.Short, 60),
			statusField,
		},
		onSubmit: func(m *Model, f *cmdForm) error {
			newProj := projects[f.fields[0].idx]
			newExt := strings.TrimSpace(f.fields[1].text.Value())
			newShort := strings.TrimSpace(f.fields[2].text.Value())
			newStatus := statusOrder[f.fields[3].idx]
			if newShort == "" {
				return fmt.Errorf("title is required")
			}
			updated := Task{
				Project:         newProj,
				ExternalID:      newExt,
				Status:          newStatus,
				Short:           newShort,
				Archived:        tk.Archived,
				StatusChangedAt: tk.StatusChangedAt,
			}
			if newStatus != tk.Status {
				updated.StatusChangedAt = m.today
			}
			if m.db != nil {
				if err := m.db.UpdateTask(taskExtID, updated); err != nil {
					return err
				}
				return m.reload()
			}
			// in-memory
			for i := range m.tasks {
				if m.tasks[i].ExternalID == taskExtID {
					m.tasks[i] = updated
					break
				}
			}
			if newExt != taskExtID {
				for i := range m.logs {
					if m.logs[i].TaskID == taskExtID {
						m.logs[i].TaskID = newExt
					}
				}
			}
			return nil
		},
	}
	form.refocusInputs()
	return form
}
