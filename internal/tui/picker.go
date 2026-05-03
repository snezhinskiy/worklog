package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// pickerItem is one selectable row inside a picker overlay. Display is the
// pre-rendered (possibly colored) string shown to the user. Haystack is the
// lowercased text the search query is matched against. ValueStr / valueInt
// hold whatever identifier the onPick callback needs.
type pickerItem struct {
	display  string
	haystack string
	valueStr string
	valueInt int
}

type picker struct {
	title  string
	hint   string
	input  textinput.Model
	cursor int
	items  []pickerItem
	onPick func(*Model, pickerItem)
}

func newPicker(title, hint string, items []pickerItem, onPick func(*Model, pickerItem)) *picker {
	ti := textinput.New()
	ti.Prompt = "🔍  "
	ti.Width = 50
	ti.Focus()
	ti.TextStyle = lipgloss.NewStyle().Foreground(colBright)
	return &picker{title: title, hint: hint, input: ti, items: items, onPick: onPick}
}

func (p *picker) filtered() []pickerItem {
	q := strings.ToLower(strings.TrimSpace(p.input.Value()))
	if q == "" {
		return p.items
	}
	var out []pickerItem
	for _, it := range p.items {
		if strings.Contains(it.haystack, q) {
			out = append(out, it)
		}
	}
	return out
}

// updatePicker routes a key. Returns (cmd, closed).
func (m *Model) updatePicker(msg tea.KeyMsg) (tea.Cmd, bool) {
	p := m.picker
	switch msg.String() {
	case "esc":
		m.picker = nil
		return nil, true
	case "enter":
		matches := p.filtered()
		if len(matches) == 0 || p.cursor >= len(matches) {
			return nil, false
		}
		chosen := matches[p.cursor]
		m.picker = nil
		if p.onPick != nil {
			p.onPick(m, chosen)
		}
		return nil, true
	case "down":
		matches := p.filtered()
		if len(matches) > 0 {
			p.cursor = clamp(p.cursor+1, 0, len(matches)-1)
		}
		return nil, false
	case "up":
		p.cursor = clamp(p.cursor-1, 0, max(0, len(p.filtered())-1))
		return nil, false
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	if mx := len(p.filtered()) - 1; p.cursor > mx {
		p.cursor = max(0, mx)
	}
	return cmd, false
}

func (m Model) renderPicker(width int) string {
	p := m.picker
	if p == nil {
		return ""
	}
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colBright)
	header := titleStyle.Render(p.title)

	matches := p.filtered()
	var rowLines []string
	if len(matches) == 0 {
		rowLines = append(rowLines, muted.Render("  no matches"))
	}
	const visible = 10
	start := 0
	if p.cursor >= visible {
		start = p.cursor - visible + 1
	}
	end := start + visible
	if end > len(matches) {
		end = len(matches)
	}
	for i := start; i < end; i++ {
		mark := "  "
		if i == p.cursor {
			mark = cursorMark.Render("▸ ")
		}
		rowLines = append(rowLines, mark+matches[i].display)
	}
	if len(matches) > visible {
		rowLines = append(rowLines, muted.Render(fmt.Sprintf("  … %d/%d", end, len(matches))))
	}

	hint := p.hint
	if hint == "" {
		hint = "↑/↓ select · enter pick · esc cancel"
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		"  "+p.input.View(),
		"",
		strings.Join(rowLines, "\n"),
		"",
		muted.Render(hint),
	)

	w := 90
	if w > width-4 {
		w = width - 4
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colAccent).
		Padding(1, 2).
		Width(w).
		Render(body)
}

// ── factories ───────────────────────────────────────────────────────────────

// hiddenTaskPickerItems lists archived tasks (used by /task unhide). Reads
// directly from the DB since m.tasks is the visible-only slice.
func hiddenTaskPickerItems(m *Model) []pickerItem {
	if m.db == nil {
		return nil
	}
	all, err := m.db.ListTasks(true)
	if err != nil {
		return nil
	}
	items := make([]pickerItem, 0)
	for _, t := range all {
		if !t.Archived {
			continue
		}
		ext := muted.Render(padRight(t.ExternalID, 8))
		proj := projectColor(t.Project).Render(padRight(t.Project, 8))
		items = append(items, pickerItem{
			display:  ext + "  " + proj + "  " + muted.Render(t.Short),
			haystack: strings.ToLower(t.ExternalID + " " + t.Project + " " + t.Short),
			valueStr: t.ExternalID,
		})
	}
	return items
}

// hiddenProjectPickerItems lists archived projects for /project unhide.
func hiddenProjectPickerItems(m *Model) []pickerItem {
	if m.db == nil {
		return nil
	}
	all, err := m.db.ListProjects(true)
	if err != nil {
		return nil
	}
	items := make([]pickerItem, 0)
	for _, p := range all {
		if !p.Archived {
			continue
		}
		items = append(items, pickerItem{
			display:  muted.Render(padRight(p.Slug, 12)) + "  " + muted.Render(p.Name),
			haystack: strings.ToLower(p.Slug + " " + p.Name),
			valueStr: p.Slug,
		})
	}
	return items
}

// hiddenLogPickerItems lists archived log entries for /log unhide. valueInt
// holds the log row id (cast from int64 — fits comfortably in int for any
// realistic personal-tracker volume).
func hiddenLogPickerItems(m *Model) []pickerItem {
	if m.db == nil {
		return nil
	}
	all, err := m.db.ListLogs(true)
	if err != nil {
		return nil
	}
	items := make([]pickerItem, 0)
	for _, l := range all {
		if !l.Archived {
			continue
		}
		date := fmt.Sprintf("%s %d %s", weekdayShort[l.Date.Weekday()],
			l.Date.Day(), monthShort[l.Date.Month()])
		display := muted.Render(padRight(date, 9)) + "  " +
			muted.Render(padRight(l.Time, 5)) + "  " +
			muted.Render(padRight(l.TaskID, 8)) + "  " +
			muted.Render(padRight(l.Note, 40)) + "  " +
			muted.Render(fmt.Sprintf("%.1fh", l.Hours))
		items = append(items, pickerItem{
			display:  display,
			haystack: strings.ToLower(l.TaskID + " " + l.Note + " " + date),
			valueInt: int(l.ID),
		})
	}
	return items
}

// taskPickerItems lists all tasks (open + done + archived) with rich display.
// Sort: open status first, archived last; within each, by extID.
func taskPickerItems(m *Model) []pickerItem {
	tasks := append([]Task(nil), m.tasks...)
	sort.SliceStable(tasks, func(i, j int) bool {
		ai, aj := taskRank(tasks[i]), taskRank(tasks[j])
		if ai != aj {
			return ai < aj
		}
		return tasks[i].ExternalID < tasks[j].ExternalID
	})
	items := make([]pickerItem, 0, len(tasks))
	for _, t := range tasks {
		statusGlyph := statusStyle[t.Status].Render(statusIcon[t.Status])
		ext := lipgloss.NewStyle().Foreground(colBright).Render(padRight(t.ExternalID, 8))
		proj := projectColor(t.Project).Render(padRight(t.Project, 8))
		title := t.Short
		if t.Status == "done" || t.Archived {
			ext = muted.Render(padRight(t.ExternalID, 8))
			title = muted.Render(t.Short)
		} else {
			title = text.Render(t.Short)
		}
		archMark := ""
		if t.Archived {
			archMark = "  " + muted.Render("[archived]")
		}
		items = append(items, pickerItem{
			display:  statusGlyph + "  " + ext + "  " + proj + "  " + title + archMark,
			haystack: strings.ToLower(t.ExternalID + " " + t.Project + " " + t.Short + " " + t.Status),
			valueStr: t.ExternalID,
		})
	}
	return items
}

func taskRank(t Task) int {
	if t.Archived {
		return 3
	}
	if t.Status == "done" {
		return 2
	}
	return 1
}

func projectPickerItems(m *Model) []pickerItem {
	projs := append([]Project(nil), m.projects...)
	sort.SliceStable(projs, func(i, j int) bool {
		if projs[i].Archived != projs[j].Archived {
			return !projs[i].Archived
		}
		return projs[i].Slug < projs[j].Slug
	})
	items := make([]pickerItem, 0, len(projs))
	for _, p := range projs {
		slug := projectColor(p.Slug).Render(padRight(p.Slug, 12))
		name := text.Render(p.Name)
		archMark := ""
		if p.Archived {
			slug = muted.Render(padRight(p.Slug, 12))
			name = muted.Render(p.Name)
			archMark = "  " + muted.Render("[archived]")
		}
		items = append(items, pickerItem{
			display:  slug + "  " + name + archMark,
			haystack: strings.ToLower(p.Slug + " " + p.Name),
			valueStr: p.Slug,
		})
	}
	return items
}

// logPickerItems lists every log entry, newest first. valueInt = idx in m.logs.
func logPickerItems(m *Model) []pickerItem {
	type pair struct {
		idx int
		l   LogEntry
	}
	pairs := make([]pair, len(m.logs))
	for i, l := range m.logs {
		pairs[i] = pair{i, l}
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if !pairs[i].l.Date.Equal(pairs[j].l.Date) {
			return pairs[i].l.Date.After(pairs[j].l.Date)
		}
		return pairs[i].l.Time > pairs[j].l.Time
	})
	items := make([]pickerItem, 0, len(pairs))
	for _, p := range pairs {
		l := p.l
		tk, _ := m.taskByID(l.TaskID)
		date := fmt.Sprintf("%s %d %s", weekdayShort[l.Date.Weekday()], l.Date.Day(), monthShort[l.Date.Month()])
		display := noteTime.Render(padRight(date, 9)) + "  " +
			noteTime.Render(padRight(l.Time, 5)) + "  " +
			lipgloss.NewStyle().Foreground(colBright).Render(padRight(l.TaskID, 8)) + "  " +
			projectColor(tk.Project).Render(padRight(tk.Project, 8)) + "  " +
			text.Render(padRight(l.Note, 40)) + "  " +
			hours.Render(fmt.Sprintf("%.1fh", l.Hours))
		items = append(items, pickerItem{
			display:  display,
			haystack: strings.ToLower(l.TaskID + " " + tk.Project + " " + l.Note + " " + date),
			valueInt: p.idx,
		})
	}
	return items
}
