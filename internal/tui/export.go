package tui

import (
	"fmt"
	"time"

	"github.com/atotto/clipboard"

	"github.com/snezhinskiy/worklog/internal/report"
)

// runExport renders the requested range as plain text, copies it to the
// system clipboard, and sets m.toast to a confirmation line. The active
// project filter is always honoured so the exported text matches what the
// user is looking at. kind == "" means "use the current view's range" so
// /export with no args is WYSIWYG.
func (m *Model) runExport(kind string) {
	var from, to time.Time
	var rangeLabel string
	if kind == "" {
		from, to = m.rangeBounds()
		rangeLabel = m.rngLabel()
	} else {
		var ok bool
		from, to, ok = report.RangeFor(m.today, kind)
		if !ok {
			m.toast = "unknown export range: " + kind
			return
		}
		rangeLabel = kind
	}

	wantProj := m.activeProject() // "" when filter is "all"
	logs := m.logs
	if wantProj != "" {
		filtered := make([]LogEntry, 0, len(logs))
		// log → task lookup so we can match logs to a project even though
		// LogEntry doesn't carry a project slug directly.
		taskProj := make(map[string]string, len(m.tasks))
		for _, t := range m.tasks {
			taskProj[t.ExternalID] = t.Project
		}
		for _, l := range logs {
			if taskProj[l.TaskID] == wantProj {
				filtered = append(filtered, l)
			}
		}
		logs = filtered
	}

	days := report.Group(logs, m.tasks, from, to)
	text := report.Render(days, report.Options{From: from, To: to, WithNotes: true})
	if err := clipboard.WriteAll(text); err != nil {
		m.toast = "clipboard: " + err.Error()
		return
	}
	var total float64
	for _, d := range days {
		total += d.Total
	}
	scope := rangeLabel
	if wantProj != "" {
		scope += " · " + wantProj
	}
	m.toast = fmt.Sprintf("exported %s · %gh · copied (%d chars)",
		scope, total, len(text))
}

// rngLabel returns a short human label for the current TUI range — used in
// the /export toast so the user can see what was actually exported.
func (m Model) rngLabel() string {
	switch m.rng {
	case rangeToday:
		return "today"
	case rangeWeek:
		return "week"
	case rangeMonth:
		return "month"
	}
	return "view"
}
