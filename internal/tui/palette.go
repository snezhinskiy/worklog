package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type paletteCmd struct {
	Key  string                // e.g. "/log"
	Args string                // hint after the key
	Desc string                // human description
	Open func(*Model) *cmdForm // returns the form to install (nil = TODO stub)
}

type palette struct {
	input  textinput.Model
	cmds   []paletteCmd
	cursor int
}

func paletteCommands() []paletteCmd {
	return []paletteCmd{
		// /log
		{"/log create", "<task> <duration>", "log work",
			func(m *Model) *cmdForm { return newLogForm(m) }},
		{"/log edit", "", "edit a log entry (search)",
			func(m *Model) *cmdForm {
				m.picker = newPicker("Pick a log entry", "↑/↓ · enter · esc",
					logPickerItems(m),
					func(m *Model, it pickerItem) {
						m.form = newEditLogForm(m, it.valueInt)
					})
				return nil
			}},
		{"/log hide", "", "hide a log entry",
			func(m *Model) *cmdForm {
				m.picker = newPicker("Hide a log entry — pick one", "↑/↓ · enter · esc",
					logPickerItems(m),
					func(m *Model, it pickerItem) {
						if m.db == nil || it.valueInt < 0 || it.valueInt >= len(m.logs) {
							return
						}
						_ = m.db.SetLogArchived(m.logs[it.valueInt].ID, true)
						_ = m.reload()
					})
				return nil
			}},
		{"/log unhide", "", "restore a hidden log entry",
			func(m *Model) *cmdForm {
				m.picker = newPicker("Unhide a log entry — pick one", "↑/↓ · enter · esc",
					hiddenLogPickerItems(m),
					func(m *Model, it pickerItem) {
						if m.db == nil {
							return
						}
						_ = m.db.SetLogArchived(int64(it.valueInt), false)
						_ = m.reload()
					})
				return nil
			}},
		// /task
		{"/task create", "<project> <title>", "create a task",
			func(m *Model) *cmdForm { return newTaskForm(m) }},
		{"/task edit", "", "edit a task (search, includes closed)",
			func(m *Model) *cmdForm {
				m.picker = newPicker("Pick a task", "includes done and archived",
					taskPickerItems(m),
					func(m *Model, it pickerItem) {
						m.form = newEditTaskForm(m, it.valueStr)
					})
				return nil
			}},
		{"/task hide", "", "hide a task",
			func(m *Model) *cmdForm {
				m.picker = newPicker("Hide a task — pick one", "↑/↓ · enter · esc",
					taskPickerItems(m),
					func(m *Model, it pickerItem) {
						if m.db == nil {
							return
						}
						_ = m.db.SetTaskArchived(it.valueStr, true, true)
						_ = m.reload()
					})
				return nil
			}},
		{"/task unhide", "", "restore a hidden task",
			func(m *Model) *cmdForm {
				m.picker = newPicker("Unhide a task — pick one", "↑/↓ · enter · esc",
					hiddenTaskPickerItems(m),
					func(m *Model, it pickerItem) {
						if m.db == nil {
							return
						}
						_ = m.db.SetTaskArchived(it.valueStr, false, true)
						_ = m.reload()
					})
				return nil
			}},
		// /project
		{"/project create", "<slug> [name]", "create a project",
			func(m *Model) *cmdForm { return newProjectForm() }},
		{"/project edit", "", "edit a project (search)",
			func(m *Model) *cmdForm {
				m.picker = newPicker("Pick a project", "",
					projectPickerItems(m),
					func(m *Model, it pickerItem) {
						m.form = newEditProjectForm(m, it.valueStr)
					})
				return nil
			}},
		{"/project hide", "", "hide a project",
			func(m *Model) *cmdForm {
				m.picker = newPicker("Hide a project — pick one", "↑/↓ · enter · esc",
					projectPickerItems(m),
					func(m *Model, it pickerItem) {
						if m.db == nil {
							return
						}
						_ = m.db.SetProjectArchived(it.valueStr, true, true)
						_ = m.reload()
					})
				return nil
			}},
		{"/project unhide", "", "restore a hidden project",
			func(m *Model) *cmdForm {
				m.picker = newPicker("Unhide a project — pick one", "↑/↓ · enter · esc",
					hiddenProjectPickerItems(m),
					func(m *Model, it pickerItem) {
						if m.db == nil {
							return
						}
						_ = m.db.SetProjectArchived(it.valueStr, false, true)
						_ = m.reload()
					})
				return nil
			}},
		// /activity
		{"/activity add", "<task-id>?", "log a typed activity (mr / commit / deploy / link / note) on a task",
			func(m *Model) *cmdForm {
				extID := strings.TrimSpace(m.paletteArgs)
				if extID == "" {
					extID = m.currentTaskExtID()
				}
				if extID == "" {
					m.toast = "no task under cursor — pass /activity add <task-id>"
					return nil
				}
				return newActivityForm(m, extID)
			}},
		// /export
		{"/export", "", "copy current view (range + project filter) to clipboard",
			func(m *Model) *cmdForm { m.runExport(""); return nil }},
		{"/export day", "", "copy today's entries to clipboard",
			func(m *Model) *cmdForm { m.runExport("day"); return nil }},
		{"/export week", "", "copy this week's entries to clipboard",
			func(m *Model) *cmdForm { m.runExport("week"); return nil }},
		{"/export month", "", "copy last 30 days to clipboard",
			func(m *Model) *cmdForm { m.runExport("month"); return nil }},
		{"/search", "", "filter the body live (alias for f)",
			func(m *Model) *cmdForm { m.openSearch(); return nil }},
		// Note: /move is intentionally not exposed via the palette — the
		// `m` shortcut on the body covers the same flow without typing.
		{"/about", "", "what is this",
			func(m *Model) *cmdForm { m.about = &about{}; return nil }},
	}
}

func newPalette() *palette {
	ti := textinput.New()
	ti.Prompt = ""
	ti.SetValue("/")
	ti.SetCursor(1)
	ti.Width = 40
	ti.Focus()
	ti.TextStyle = lipgloss.NewStyle().Foreground(colBright)
	return &palette{input: ti, cmds: paletteCommands()}
}

func (p *palette) filtered() []paletteCmd {
	q := strings.ToLower(strings.TrimPrefix(p.input.Value(), "/"))
	if q == "" {
		return p.cmds
	}
	var out []paletteCmd
	for _, c := range p.cmds {
		if strings.Contains(strings.ToLower(c.Key+" "+c.Desc), q) {
			out = append(out, c)
		}
	}
	return out
}

// updatePalette routes a key to the palette. Returns (cmd, closed).
// When closed, m.palette has been cleared and possibly m.form set.
func (m *Model) updatePalette(msg tea.KeyMsg) (tea.Cmd, bool) {
	p := m.palette
	switch msg.String() {
	case "esc":
		m.palette = nil
		return nil, true
	case "enter":
		raw := strings.TrimSpace(strings.TrimPrefix(p.input.Value(), "/"))
		// Direct match: "/<cmd-key> <args>" — take the longest command
		// key that's a prefix of the input, treat the tail as args.
		// Falls back to the cursor's choice from the filtered list when
		// nothing matches as a prefix.
		var chosen *paletteCmd
		args := ""
		bestKeyLen := 0
		for i := range p.cmds {
			key := strings.TrimPrefix(p.cmds[i].Key, "/")
			if raw == key && len(key) > bestKeyLen {
				chosen = &p.cmds[i]
				args = ""
				bestKeyLen = len(key)
			} else if strings.HasPrefix(raw, key+" ") && len(key) > bestKeyLen {
				chosen = &p.cmds[i]
				args = strings.TrimSpace(raw[len(key):])
				bestKeyLen = len(key)
			}
		}
		if chosen == nil {
			matches := p.filtered()
			if len(matches) == 0 || p.cursor >= len(matches) {
				return nil, false
			}
			chosen = &matches[p.cursor]
		}
		m.palette = nil
		m.paletteArgs = args
		if chosen.Open != nil {
			m.form = chosen.Open(m)
		}
		m.paletteArgs = ""
		return nil, true
	case "down":
		matches := p.filtered()
		p.cursor = clamp(p.cursor+1, 0, max(0, len(matches)-1))
		return nil, false
	case "up":
		p.cursor = clamp(p.cursor-1, 0, len(p.filtered())-1)
		return nil, false
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	// keep the leading "/"
	if !strings.HasPrefix(p.input.Value(), "/") {
		p.input.SetValue("/" + p.input.Value())
		p.input.SetCursor(len(p.input.Value()))
	}
	// reclamp cursor if filter result shrank
	if mx := len(p.filtered()) - 1; p.cursor > mx {
		p.cursor = max(0, mx)
	}
	return cmd, false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m Model) renderPalette(width int) string {
	p := m.palette
	if p == nil {
		return ""
	}
	header := lipgloss.NewStyle().Bold(true).Foreground(colBright).Render("Command")
	inputLine := "  " + p.input.View()

	matches := p.filtered()
	var rowLines []string
	if len(matches) == 0 {
		rowLines = append(rowLines, muted.Render("  no matches"))
	}
	for i, c := range matches {
		mark := "  "
		keyStyle := rowNormal
		if i == p.cursor {
			mark = cursorMark.Render("▸ ")
			keyStyle = rowSelected
		}
		row := mark +
			keyStyle.Render(padRight(c.Key, 17)) + " " +
			muted.Render(padRight(c.Args, 22)) + "  " +
			text.Render(c.Desc)
		rowLines = append(rowLines, row)
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		inputLine,
		"",
		strings.Join(rowLines, "\n"),
		"",
		muted.Render("↑/↓ select · ↵ run · esc close"),
	)

	w := 80
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
