package tui

// form.go is the generic form infrastructure: field types, the cmdForm
// container, key routing (updateForm), submit, and renderForm. Concrete
// constructors live in form_project.go, form_task.go, form_log.go.

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// formFieldKind tells the renderer how to draw a field and which keys to honour.
type formFieldKind int

const (
	fieldText formFieldKind = iota
	fieldChoice
)

type formField struct {
	label string
	kind  formFieldKind

	// text fields
	text textinput.Model

	// choice fields: cycle through options[idx], render via display(idx) if set
	options []string
	display func(idx int) string
	idx     int

	// onChange is invoked after the value or selection changes — used by /log
	// to refresh the task list when the project flips.
	onChange func(*Model, *cmdForm)
}

// cmdForm is the generic palette-spawned form. Each command builds one of
// these with its fields, an optional submit hook, and an optional title.
type cmdForm struct {
	title    string
	fields   []*formField
	focus    int
	onSubmit func(*Model, *cmdForm) error
	saved    bool
	errMsg   string
}

func newTextField(label, value string, width int) *formField {
	ti := textinput.New()
	ti.SetValue(value)
	ti.Width = width
	ti.Prompt = ""
	ti.TextStyle = lipgloss.NewStyle().Foreground(colBright)
	return &formField{label: label, kind: fieldText, text: ti}
}

func newChoiceField(label string, options []string, idx int, display func(idx int) string) *formField {
	if idx < 0 || idx >= len(options) {
		idx = 0
	}
	return &formField{label: label, kind: fieldChoice, options: options, display: display, idx: idx}
}

func (f *cmdForm) activeField() *formField {
	if f.focus < 0 || f.focus >= len(f.fields) {
		return nil
	}
	return f.fields[f.focus]
}

func (f *cmdForm) refocusInputs() {
	for i, fl := range f.fields {
		if fl.kind == fieldText {
			if i == f.focus {
				fl.text.Focus()
			} else {
				fl.text.Blur()
			}
		}
	}
}

// updateForm routes one key message to the active form. Returns a tea.Cmd
// (for textinput cursor blink) and a bool: true if the form should close.
func (m *Model) updateForm(msg tea.KeyMsg) (tea.Cmd, bool) {
	f := m.form
	switch msg.String() {
	case "esc":
		return nil, true
	case "ctrl+s", "ctrl+enter":
		return nil, m.submitForm()
	case "tab", "down":
		f.focus = (f.focus + 1) % len(f.fields)
		f.refocusInputs()
		return nil, false
	case "shift+tab", "up":
		f.focus = (f.focus - 1 + len(f.fields)) % len(f.fields)
		f.refocusInputs()
		return nil, false
	case "enter":
		// Enter advances field; on the last field, submits.
		if f.focus == len(f.fields)-1 {
			return nil, m.submitForm()
		}
		f.focus++
		f.refocusInputs()
		return nil, false
	case "left":
		if af := f.activeField(); af != nil && af.kind == fieldChoice {
			af.idx = clamp(af.idx-1, 0, len(af.options)-1)
			if af.onChange != nil {
				af.onChange(m, f)
			}
			return nil, false
		}
	case "right":
		if af := f.activeField(); af != nil && af.kind == fieldChoice {
			af.idx = clamp(af.idx+1, 0, len(af.options)-1)
			if af.onChange != nil {
				af.onChange(m, f)
			}
			return nil, false
		}
	}
	// forward to focused textinput
	if af := f.activeField(); af != nil && af.kind == fieldText {
		var cmd tea.Cmd
		af.text, cmd = af.text.Update(msg)
		return cmd, false
	}
	return nil, false
}

func (m *Model) submitForm() bool {
	if m.form == nil {
		return true
	}
	if m.form.onSubmit != nil {
		if err := m.form.onSubmit(m, m.form); err != nil {
			m.form.errMsg = err.Error()
			return false
		}
	}
	m.recompute()
	return true
}

// ── render ──────────────────────────────────────────────────────────────────

func (m Model) renderForm(width int) string {
	f := m.form
	if f == nil {
		return ""
	}

	header := lipgloss.NewStyle().Bold(true).Foreground(colBright).Render(f.title)

	lines := []string{header, ""}
	for i, fl := range f.fields {
		var value string
		switch fl.kind {
		case fieldText:
			value = fl.text.View()
		case fieldChoice:
			if fl.display != nil {
				value = fl.display(fl.idx) + "  " + muted.Render("←/→")
			} else if len(fl.options) > 0 {
				value = chipActive.Render(fl.options[fl.idx]) + "  " + muted.Render("←/→")
			}
		}
		lines = append(lines, m.editFieldLine(fl.label, value, i == f.focus))
	}
	lines = append(lines, "")
	if f.errMsg != "" {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
		lines = append(lines, errStyle.Render("⚠ "+f.errMsg))
		lines = append(lines, "")
	}
	lines = append(lines, muted.Render("↑/↓ field · enter next/submit · esc cancel"))

	body := lipgloss.JoinVertical(lipgloss.Left, lines...)

	formW := 70
	if formW > width-4 {
		formW = width - 4
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colAccent).
		Padding(1, 2).
		Width(formW).
		Render(body)
}

// ── shared form helpers ─────────────────────────────────────────────────────
// projectSlugs/openTasksOfProject/taskLabels are used by both the task and
// log form constructors — they live here in the infrastructure file so the
// per-entity files don't have to import each other.

func projectSlugs(m *Model) []string {
	out := make([]string, len(m.projects))
	for i, p := range m.projects {
		out[i] = p.Slug
	}
	return out
}

func openTasksOfProject(m *Model, slug string) []Task {
	var out []Task
	for _, t := range m.tasks {
		if t.Project == slug && t.Status != "done" {
			out = append(out, t)
		}
	}
	return out
}

func taskLabels(tasks []Task) []string {
	if len(tasks) == 0 {
		return []string{"(no open tasks)"}
	}
	out := make([]string, len(tasks))
	for i, t := range tasks {
		out[i] = t.ExternalID + " — " + t.Short
	}
	return out
}
