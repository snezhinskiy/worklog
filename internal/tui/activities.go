package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/snezhinskiy/worklog/internal/domain"
	"github.com/snezhinskiy/worklog/internal/store"
)

// newActivityForm builds the create-activity form for a specific task. The
// type choice cycles through ActivityTypes; URL and Text are both optional
// but at least one must be non-empty so the entry has a payload.
func newActivityForm(m *Model, taskExtID string) *cmdForm {
	if taskExtID == "" {
		return nil
	}
	typeField := newChoiceField("Type", store.ActivityTypes, 0, func(idx int) string {
		t := store.ActivityTypes[idx]
		icon := lipgloss.NewStyle().Foreground(colAccent).Render(activityIcon(t))
		return icon + " " + t
	})
	f := &cmdForm{
		title: "New activity on " + taskExtID + "  ·  /activity add",
		fields: []*formField{
			typeField,
			newTextField("URL", "", 60),
			newTextField("Text", "", 60),
		},
		onSubmit: func(m *Model, f *cmdForm) error {
			typ := store.ActivityTypes[f.fields[0].idx]
			url := strings.TrimSpace(f.fields[1].text.Value())
			textVal := strings.TrimSpace(f.fields[2].text.Value())
			a := store.Activity{TaskID: taskExtID, Type: typ, URL: url, Text: textVal}
			if err := domain.ValidateActivity(a); err != nil {
				return err
			}
			if m.db != nil {
				if _, err := m.db.CreateActivity(a); err != nil {
					return err
				}
				return m.reload()
			}
			// In-memory fallback: append; new IDs aren't meaningful here.
			a.ID = int64(len(m.activities) + 1)
			m.activities = append(m.activities, a)
			return nil
		},
	}
	f.refocusInputs()
	return f
}
