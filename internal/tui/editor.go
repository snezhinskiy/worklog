package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/lipgloss"

	"github.com/snezhinskiy/worklog/internal/domain"
	"github.com/snezhinskiy/worklog/internal/store"
)

// statusOrder is now sourced from the active config (see styles.go applyTheme).
// We keep this comment as a pointer for future grep'ers.

const (
	editStatus = iota
	editID
	editTitle
	editDate
	editTime
	editHours
	editNote
	editFieldCount
)

type editForm struct {
	taskExtID string // original id; used as the lookup key on save
	project   string
	logIdx    int // index into Model.logs of the entry being edited (-1 = no log)

	status int // index in statusOrder

	title textinput.Model
	idF   textinput.Model // editable external_id; rename when changed
	date  textinput.Model
	timeF textinput.Model
	hours textinput.Model
	note  textinput.Model

	focusField int
	saved      bool // last action was save (used for transient confirmation later)
}

// enterEdit builds an edit form from the currently selected row.
//   - cursor on rowTask: edits the task; the log fields target the latest log
//     entry of that task within the active range (or are empty if none).
//   - cursor on rowLog:  edits that specific log entry directly; the task
//     fields show its parent task so status/title can be tweaked too.
func (m *Model) enterEdit() {
	if m.cursor >= len(m.view.selectableIdx) {
		return
	}
	rowIdx := m.view.selectableIdx[m.cursor]
	r := m.view.rows[rowIdx]

	var taskExtID string
	logIdx := -1
	switch r.kind {
	case rowLog:
		taskExtID = r.parentTaskID
		logIdx = r.logIdx
	case rowTask:
		if r.task == nil {
			return
		}
		taskExtID = r.task.extID
		logIdx = m.latestLogIdxForTask(taskExtID)
	default:
		return
	}

	tk, ok := m.taskByID(taskExtID)
	if !ok {
		return
	}

	f := editForm{
		taskExtID: tk.ExternalID,
		project:   tk.Project,
		status:    statusIndex(tk.Status),
		logIdx:    logIdx,
	}
	f.title = newInput(tk.Short, 60)
	f.idF = newInput(tk.ExternalID, 16)
	if logIdx >= 0 {
		l := m.logs[logIdx]
		f.date = newInput(l.Date.Format("2006-01-02"), 12)
		f.timeF = newInput(l.Time, 6)
		f.hours = newInput(strconv.FormatFloat(l.Hours, 'f', -1, 64), 6)
		f.note = newInput(l.Note, 60)
	} else {
		f.date = newInput("", 12)
		f.timeF = newInput("", 6)
		f.hours = newInput("", 6)
		f.note = newInput("", 60)
	}
	f.focusField = editStatus
	m.edit = &f
}

// latestLogIdxForTask returns the index in m.logs of the most recent entry
// for taskExtID within the active range, or -1.
func (m Model) latestLogIdxForTask(taskExtID string) int {
	from, to := m.rangeBounds()
	idx := -1
	for i := range m.logs {
		l := m.logs[i]
		if l.TaskID != taskExtID {
			continue
		}
		if l.Date.Before(from) || l.Date.After(to) {
			continue
		}
		if idx == -1 {
			idx = i
			continue
		}
		cur := m.logs[idx]
		if l.Date.After(cur.Date) || (l.Date.Equal(cur.Date) && l.Time > cur.Time) {
			idx = i
		}
	}
	return idx
}

func newInput(value string, width int) textinput.Model {
	ti := textinput.New()
	ti.SetValue(value)
	ti.Width = width
	ti.Prompt = ""
	ti.TextStyle = lipgloss.NewStyle().Foreground(colBright)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colDim)
	return ti
}

func statusIndex(s string) int {
	for i, x := range statusOrder {
		if x == s {
			return i
		}
	}
	return 0
}

// updateEdit routes a key message to the edit form.
func (m *Model) updateEdit(msg tea.KeyMsg) tea.Cmd {
	f := m.edit
	switch msg.String() {
	case "esc":
		m.edit = nil
		return nil
	case "ctrl+s":
		m.saveEdit()
		m.edit = nil
		return nil
	case "tab", "down":
		f.focusField = (f.focusField + 1) % editFieldCount
		m.refocusEditInputs()
		return nil
	case "shift+tab", "up":
		f.focusField = (f.focusField - 1 + editFieldCount) % editFieldCount
		m.refocusEditInputs()
		return nil
	case "left":
		if f.focusField == editStatus {
			f.status = clamp(f.status-1, 0, len(statusOrder)-1)
			return nil
		}
	case "right":
		if f.focusField == editStatus {
			f.status = clamp(f.status+1, 0, len(statusOrder)-1)
			return nil
		}
	}

	// Forward keys to the focused textinput.
	var cmd tea.Cmd
	ti := f.activeInput()
	if ti != nil {
		*ti, cmd = ti.Update(msg)
	}
	return cmd
}

func (f *editForm) activeInput() *textinput.Model {
	switch f.focusField {
	case editTitle:
		return &f.title
	case editID:
		return &f.idF
	case editDate:
		return &f.date
	case editTime:
		return &f.timeF
	case editHours:
		return &f.hours
	case editNote:
		return &f.note
	}
	return nil
}

func (m *Model) refocusEditInputs() {
	f := m.edit
	for _, ti := range []*textinput.Model{&f.title, &f.idF, &f.date, &f.timeF, &f.hours, &f.note} {
		ti.Blur()
	}
	if ti := f.activeInput(); ti != nil {
		ti.Focus()
	}
}

// saveEdit writes the form values back. With a DB present, it calls
// store.UpdateTask + store.UpdateLog and reloads; without one it mutates the
// in-memory slices.
func (m *Model) saveEdit() {
	f := m.edit
	newStatus := statusOrder[f.status]
	newTitle := strings.TrimSpace(f.title.Value())

	if m.db != nil {
		// Persist task changes (status + title).
		var orig Task
		for _, t := range m.tasks {
			if t.ExternalID == f.taskExtID {
				orig = t
				break
			}
		}
		updated := orig
		updated.Status = newStatus
		updated.Short = newTitle
		if newStatus != orig.Status {
			updated.StatusChangedAt = m.today
		}
		// Rename if the ID field changed. UpdateTask uses the old id as
		// the WHERE key; FK CASCADE on logs/activities follows.
		if newID := strings.TrimSpace(f.idF.Value()); newID != "" && newID != f.taskExtID {
			updated.ExternalID = newID
		}
		_ = m.db.UpdateTask(f.taskExtID, updated)

		// Persist log changes (if a log row was attached).
		if f.logIdx >= 0 && f.logIdx < len(m.logs) {
			le := m.logs[f.logIdx]
			if d, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(f.date.Value()), m.today.Location()); err == nil {
				le.Date = d
			}
			if t := strings.TrimSpace(f.timeF.Value()); t != "" {
				le.Time = t
			}
			if h, err := domain.ParseHours(f.hours.Value()); err == nil {
				le.Hours = h
			}
			le.Note = strings.TrimSpace(f.note.Value())
			_ = m.db.UpdateLog(le)
		}
		_ = m.reload()
		return
	}

	// In-memory fallback (snapshot/demo mode).
	newID := strings.TrimSpace(f.idF.Value())
	for i := range m.tasks {
		if m.tasks[i].ExternalID == f.taskExtID {
			m.tasks[i].Status = newStatus
			m.tasks[i].Short = newTitle
			if newID != "" && newID != f.taskExtID {
				m.tasks[i].ExternalID = newID
				// cascade in-memory: rename references in logs and activities
				for j := range m.logs {
					if m.logs[j].TaskID == f.taskExtID {
						m.logs[j].TaskID = newID
					}
				}
				for j := range m.activities {
					if m.activities[j].TaskID == f.taskExtID {
						m.activities[j].TaskID = newID
					}
				}
			}
			break
		}
	}
	if f.logIdx >= 0 && f.logIdx < len(m.logs) {
		l := &m.logs[f.logIdx]
		if d, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(f.date.Value()), m.today.Location()); err == nil {
			l.Date = d
		}
		if t := strings.TrimSpace(f.timeF.Value()); t != "" {
			l.Time = t
		}
		if h, err := domain.ParseHours(f.hours.Value()); err == nil {
			l.Hours = h
		}
		l.Note = strings.TrimSpace(f.note.Value())
	}
	m.recompute()
}

// ── render ──────────────────────────────────────────────────────────────────

func (m Model) renderEditor(width int) string {
	f := m.edit
	if f == nil {
		return ""
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(colBright)
	header := headerStyle.Render("Edit " + f.taskExtID + "  ") + projectColor(f.project).Render(f.project)

	statusChip := statusStyle[statusOrder[f.status]].Render(statusIcon[statusOrder[f.status]] + " " + statusOrder[f.status])
	statusHint := muted.Render("  ←/→ to cycle")
	statusLine := m.editFieldLine("Status", statusChip+statusHint, f.focusField == editStatus)

	titleLine := m.editFieldLine("Title", f.title.View(), f.focusField == editTitle)
	idLine := m.editFieldLine("ID", f.idF.View(), f.focusField == editID)

	var logHeader string
	if f.logIdx >= 0 {
		d := m.logs[f.logIdx].Date
		logHeader = muted.Render(fmt.Sprintf("── Log on %d %s %d ──",
			d.Day(), monthName[d.Month()], d.Year()))
	} else {
		logHeader = muted.Render("── No log entries in this period — fields are empty ──")
	}

	dateLine := m.editFieldLine("Date", f.date.View(), f.focusField == editDate)
	timeLine := m.editFieldLine("Time", f.timeF.View(), f.focusField == editTime)
	hoursLine := m.editFieldLine("Hours", f.hours.View(), f.focusField == editHours)
	noteLine := m.editFieldLine("Note", f.note.View(), f.focusField == editNote)

	help := muted.Render("↑/↓ field · esc cancel · ctrl+s save")

	activitiesBlock := m.renderEditorActivities(f.taskExtID)

	body := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		statusLine,
		idLine,
		titleLine,
		"",
		logHeader,
		dateLine,
		timeLine,
		hoursLine,
		noteLine,
		"",
		activitiesBlock,
		help,
	)

	// Grow the editor up to a comfortable reading width on wide terminals
	// (so long URLs and titles don't wrap as aggressively), but never
	// exceed the available area.
	formW := 120
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

// renderEditorActivities renders the typed activity timeline for the task
// being edited. Empty list collapses to a single muted hint so the editor
// height stays predictable; otherwise every activity gets one or two lines
// (text + URL on its own indented line when both are present).
func (m Model) renderEditorActivities(taskExtID string) string {
	header := muted.Render("── Activities ──")
	hint := muted.Render("  (none yet — press " +
		lipgloss.NewStyle().Foreground(colAccent).Render("a") + " to add)")
	var lines []string
	for _, a := range m.activities {
		if a.TaskID != taskExtID {
			continue
		}
		lines = append(lines, formatActivityLines(a)...)
	}
	if len(lines) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, header, hint, "")
	}
	all := append([]string{header}, lines...)
	all = append(all, "")
	return lipgloss.JoinVertical(lipgloss.Left, all...)
}

// formatActivityLines lays out one activity over one or two lines:
//
//	icon type    text                        — when only text or only url
//	icon type    text
//	             ↳ url                        — when both are present
//
// URLs are rendered as plain coloured text. Terminals with URL detection
// (iTerm2, Terminal.app, WezTerm, etc.) make them cmd-clickable; OSC 8
// hyperlink wrapping was tried but broke lipgloss word-wrap (the wrap
// math doesn't skip OSC escapes, splitting URLs mid-string).
func formatActivityLines(a store.Activity) []string {
	icon := activityIcon(a.Type)
	typ := lipgloss.NewStyle().Foreground(colBright).Render(padRight(a.Type, 7))
	prefix := "  " + icon + " " + typ + "  "
	indent := strings.Repeat(" ", lipgloss.Width(prefix))
	// No Underline — iTerm2 (and similar terminals) draws its own hover
	// underline on detected URLs; ours would stack on top.
	urlStyled := lipgloss.NewStyle().Foreground(colAccent).Render(a.URL)

	switch {
	case a.URL != "" && a.Text != "":
		return []string{
			prefix + text.Render(a.Text),
			indent + muted.Render("↳ ") + urlStyled,
		}
	case a.URL != "":
		return []string{prefix + urlStyled}
	case a.Text != "":
		return []string{prefix + text.Render(a.Text)}
	}
	return []string{prefix + muted.Render("(empty)")}
}

func activityIcon(t string) string {
	switch t {
	case "mr":
		return "⇄"
	case "commit":
		return "⎇"
	case "deploy":
		return "▲"
	case "link":
		return "🔗"
	case "note":
		return "✎"
	}
	return "·"
}

func (m Model) editFieldLine(label, value string, focused bool) string {
	chevron := "  "
	labelStyle := chipLabel
	if focused {
		chevron = focusChevron.Render("❯ ")
		labelStyle = chipLabelFocused
	}
	return chevron + labelStyle.Render(padRight(label, 11)) + "  " + value
}
