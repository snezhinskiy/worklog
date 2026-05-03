// Snapshot* helpers render the model at a fixed window size and return the
// string output. Used by `worklog --dump …` for headless screenshot tests
// and visual debugging — they all run with the in-memory mock seed (db=nil)
// so they're deterministic across runs.
package tui

import "strings"

// Snapshot renders the model once at the given size — handy for offline preview.
func Snapshot(width, height int) string {
	return SnapshotWith(width, height, "day", "today", "body", 0, 0)
}

// SnapshotSearch opens the search bar with the given query against the
// month range, so the result-count strip and the (no matches) fallback are
// covered by a regular --dump invocation.
func SnapshotSearch(width, height int, query string) string {
	m := New(nil, nil)
	m.width, m.height = width, height
	m.bodyH = height - 13
	if m.bodyH < 5 {
		m.bodyH = 5
	}
	m.rng = rangeMonth
	m.recompute()
	m.openSearch()
	m.search.input.SetValue(query)
	m.searchQuery = strings.ToLower(strings.TrimSpace(query))
	m.cursor = 0
	m.recompute()
	return m.View()
}

func SnapshotWith(width, height int, group, rng, focus string, proj, cursorAt int) string {
	m := New(nil, nil)
	m.width, m.height = width, height
	// Mirror what WindowSizeMsg does in the real loop so scroll math
	// behaves the same in --dump as on a live terminal.
	m.bodyH = height - 13
	if m.bodyH < 5 {
		m.bodyH = 5
	}
	switch group {
	case "task":
		m.group = groupByTask
	}
	switch rng {
	case "week":
		m.rng = rangeWeek
	case "month":
		m.rng = rangeMonth
	}
	switch focus {
	case "group":
		m.focus = focusGroup
	case "range":
		m.focus = focusRange
	case "project":
		m.focus = focusProject
	}
	if proj >= 0 && proj <= len(m.projects) {
		m.projectFilter = proj
	}
	m.recompute()
	if cursorAt > 0 {
		m.moveCursor(cursorAt)
	}
	return m.View()
}

// SnapshotEditOnLog: open by-task week, expand first task, move cursor to its
// second note (logOffset), and open the editor — verifies that Enter on a log
// row picks the right entry.
func SnapshotEditOnLog(width, height, logOffset int) string {
	m := New(nil, nil)
	m.width, m.height = width, height
	m.group = groupByTask
	m.rng = rangeWeek
	m.recompute()
	if len(m.view.selectableIdx) == 0 {
		return m.View()
	}
	first := m.view.rows[m.view.selectableIdx[0]]
	if first.task == nil {
		return m.View()
	}
	m.expanded[first.task.extID] = true
	m.recompute()
	noteCursor := m.firstNoteSelectableIdx(first.task.extID) + logOffset
	if noteCursor >= len(m.view.selectableIdx) {
		noteCursor = len(m.view.selectableIdx) - 1
	}
	m.cursor = noteCursor
	m.enterEdit()
	if m.edit != nil {
		m.refocusEditInputs()
	}
	return m.View()
}

// SnapshotExpanded opens the view in by-task grouping with the first task
// expanded and cursor on its first note.
func SnapshotExpanded(width, height int, rng string) string {
	m := New(nil, nil)
	m.width, m.height = width, height
	m.group = groupByTask
	switch rng {
	case "week":
		m.rng = rangeWeek
	case "month":
		m.rng = rangeMonth
	}
	m.recompute()
	if len(m.view.selectableIdx) > 0 {
		// expand the first task
		first := m.view.rows[m.view.selectableIdx[0]]
		if first.task != nil {
			m.expanded[first.task.extID] = true
			m.recompute()
			if idx := m.firstNoteSelectableIdx(first.task.extID); idx >= 0 {
				m.cursor = idx
			}
		}
	}
	return m.View()
}

// SnapshotPalette dumps the model with the palette open and an optional query.
func SnapshotPalette(width, height int, query string) string {
	m := New(nil, nil)
	m.width, m.height = width, height
	m.recompute()
	m.palette = newPalette()
	if query != "" {
		m.palette.input.SetValue(query)
		m.palette.input.SetCursor(len(query))
	}
	return m.View()
}

// SnapshotForm dumps the model with one of the three command forms open.
func SnapshotForm(width, height int, name string) string {
	m := New(nil, nil)
	m.width, m.height = width, height
	m.recompute()
	switch name {
	case "log":
		m.form = newLogForm(&m)
	case "task":
		m.form = newTaskForm(&m)
	case "project":
		m.form = newProjectForm()
	}
	return m.View()
}

// SnapshotHelp opens the help overlay.
func SnapshotHelp(width, height int) string {
	m := New(nil, nil)
	m.width, m.height = width, height
	m.showHelp = true
	return m.View()
}

// SnapshotBoard renders the model in board mode (cursor on the first card).
func SnapshotBoard(width, height int) string {
	m := New(nil, nil)
	m.width, m.height = width, height
	m.viewMode = viewBoard
	m.recompute()
	return m.View()
}

// SnapshotPicker opens a picker of the requested kind.
func SnapshotPicker(width, height int, kind, query string) string {
	m := New(nil, nil)
	m.width, m.height = width, height
	m.recompute()
	switch kind {
	case "task":
		m.picker = newPicker("Pick a task", "includes done and archived", taskPickerItems(&m), nil)
	case "log":
		m.picker = newPicker("Pick a log entry", "↑/↓ · Enter · Esc", logPickerItems(&m), nil)
	case "project":
		m.picker = newPicker("Pick a project", "", projectPickerItems(&m), nil)
	}
	if m.picker != nil && query != "" {
		m.picker.input.SetValue(query)
		m.picker.input.SetCursor(len(query))
	}
	return m.View()
}

// SnapshotEdit dumps the editor opened on the first selectable task.
func SnapshotEdit(width, height int, focusField int) string {
	m := New(nil, nil)
	m.width, m.height = width, height
	m.recompute()
	m.enterEdit()
	if m.edit != nil {
		m.edit.focusField = focusField
		m.refocusEditInputs()
	}
	return m.View()
}
