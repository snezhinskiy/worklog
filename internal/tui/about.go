package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// about is a passive overlay — no fields, no input. Esc/i/enter dismisses it.
type about struct{}

const aboutText = `worklog — personal time tracker for people who hop between projects.

A small TUI to log work, switch task statuses, and see what's still
open across all your projects. The same data layer backs a CLI for
quick scripting and an MCP server, so you can tell Claude:

    "log 2h on AU-3569 today, was testing the refresh flow"

…and it lands as an entry here.

Why this exists
---------------
Forgetting to update Jira / Linear statuses is a real cost. This
nudges you: a single screen shows everything in flight, ages of
in_progress tasks, and what you actually shipped this week.

Four primitives
---------------
  project    a slug like AURA that groups tasks
  task       something to do; has status (todo → in_progress → … → done)
  log        a unit of work spent on a task on a date, with a note
  activity   a typed event on a task: mr, commit, deploy, link, note

Try inside the TUI
------------------
  /           open the command palette
  ↑/↓ ←/→     navigate everywhere
  ↵           edit the row under the cursor
  →           expand a task to walk its individual log entries
  g r v p     cycle grouping / range / view / project filter
  m           move the task under the cursor to a different status
  a           add an activity (mr, commit, deploy, link, note)
  f           find / filter the body live
  i ?         this screen / full keymap

Outside the TUI
---------------
  worklog log AU-3569 1.5h "fixed migration"
  worklog today | pending | export --range week --notes
  worklog activity add AU-3569 mr --url https://… --text "auth refactor"
  worklog mcp                              # speak MCP over stdio
`

// updateAbout dismisses the overlay on any key.
func (m *Model) updateAbout(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case "esc", "enter", "i", "q", " ":
		m.about = nil
		return nil, true
	}
	return nil, false
}

func (m Model) renderAbout(width int) string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colBright)
	header := titleStyle.Render("About worklog")

	body := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		text.Render(strings.TrimSpace(aboutText)),
		"",
		muted.Render("press any key to close"),
	)

	w := 78
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
