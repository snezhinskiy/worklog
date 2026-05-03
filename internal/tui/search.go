package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// searchBar is an inline filter input rendered at the top of the body. While
// it's open, m.searchQuery feeds recompute() so only matching rowTask/rowLog
// entries (and their parent section headers) survive into m.view.rows.
//
// Lifecycle:
//   - openSearch() installs it; cursor and pre-search position are stashed
//   - typing updates m.searchQuery and re-runs recompute
//   - enter closes and keeps the cursor on the filtered match
//   - esc closes and restores the cursor to where the user opened search
type searchBar struct {
	input       textinput.Model
	savedCursor int
}

func newSearchBar() *searchBar {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Width = 40
	ti.Focus()
	ti.TextStyle = lipgloss.NewStyle().Foreground(colBright)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colDim)
	ti.Placeholder = "type to filter; esc to cancel; ↵ to commit"
	return &searchBar{input: ti}
}

// openSearch activates the search bar. Stashes the cursor so esc can restore
// it. If the palette passed args (via m.paletteArgs, e.g. "/search AU-3569"),
// pre-fills the input with them so the filter applies immediately.
func (m *Model) openSearch() {
	m.search = newSearchBar()
	m.search.savedCursor = m.cursor
	if initial := strings.TrimSpace(m.paletteArgs); initial != "" {
		m.search.input.SetValue(initial)
		m.search.input.SetCursor(len(initial))
		m.searchQuery = strings.ToLower(initial)
	} else {
		m.searchQuery = ""
	}
	m.recompute()
}

// updateSearch routes keys while the search bar owns the screen. Returns
// (cmd, closed) — closed=true means the caller should clear m.search and
// resume normal body handling.
func (m *Model) updateSearch(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case "esc":
		// Cancel: restore cursor and drop the filter.
		m.searchQuery = ""
		m.recompute()
		m.cursor = m.search.savedCursor
		if m.cursor >= len(m.view.selectableIdx) {
			m.cursor = len(m.view.selectableIdx) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		m.adjustScroll()
		return nil, true
	case "enter":
		// Commit: drop the filter but keep the cursor on the row the user
		// landed on. Map the filtered cursor back into the unfiltered view
		// so it doesn't snap to row 0.
		var rowID int = -1
		if m.cursor >= 0 && m.cursor < len(m.view.selectableIdx) {
			rowIdx := m.view.selectableIdx[m.cursor]
			rowID = rowIdentity(m.view.rows[rowIdx])
		}
		m.searchQuery = ""
		m.recompute()
		if rowID >= 0 {
			for i, sIdx := range m.view.selectableIdx {
				if rowIdentity(m.view.rows[sIdx]) == rowID {
					m.cursor = i
					break
				}
			}
		}
		m.adjustScroll()
		return nil, true
	case "up":
		m.moveCursor(-1)
		return nil, false
	case "down":
		m.moveCursor(+1)
		return nil, false
	}

	prev := m.searchQuery
	var cmd tea.Cmd
	m.search.input, cmd = m.search.input.Update(msg)
	m.searchQuery = strings.ToLower(strings.TrimSpace(m.search.input.Value()))
	if m.searchQuery != prev {
		// Query changed → re-filter. Reset cursor to the top of the
		// (possibly empty) filtered list so the user sees the first match.
		m.cursor = 0
		m.recompute()
	}
	return cmd, false
}

// rowIdentity returns a stable id for a row so the enter-commit path can
// re-locate it after recompute drops the filter. Logs use their stored ID;
// tasks use a hash of their extID; sections are non-selectable so we never
// hit them here, but return -1 for safety.
func rowIdentity(r renderRow) int {
	switch r.kind {
	case rowTask:
		if r.task != nil {
			// Tasks are sparse enough that any consistent identifier
			// suffices; an FNV-style fold of the extID is enough to
			// disambiguate them in a single recompute.
			return -1 - hashStr(r.task.extID)
		}
	case rowLog:
		if r.log != nil {
			return int(r.log.ID)
		}
	}
	return -1
}

func hashStr(s string) int {
	h := 0
	for _, c := range s {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return h
}

// renderSearchBar is the one-line strip rendered above the filtered body.
// Shows the input plus a tiny result-count summary so the user can see at a
// glance whether anything matched.
func (m Model) renderSearchBar(width int) string {
	icon := lipgloss.NewStyle().Foreground(colAccent).Render("🔎 ")
	q := m.search.input.View()
	count := len(m.view.selectableIdx)
	suffix := muted.Render(fmt.Sprintf(" · %d results · esc cancel · ↵ commit", count))
	return icon + q + suffix
}

// filterRowsByQuery drops rowTask/rowLog rows whose haystack doesn't contain
// q. Section headers are kept only when at least one row in their span
// survives — this avoids "empty Tuesday" headers in the results.
func filterRowsByQuery(rows []renderRow, q string) []renderRow {
	if q == "" {
		return rows
	}
	q = strings.ToLower(q)
	out := make([]renderRow, 0, len(rows))
	i := 0
	for i < len(rows) {
		r := rows[i]
		if r.kind != rowSection {
			if rowMatches(r, q) {
				out = append(out, r)
			}
			i++
			continue
		}
		// Look ahead to the end of this section (next section or EOF) and
		// collect surviving rows.
		j := i + 1
		var kept []renderRow
		for j < len(rows) && rows[j].kind != rowSection {
			if rowMatches(rows[j], q) {
				kept = append(kept, rows[j])
			}
			j++
		}
		if len(kept) > 0 {
			out = append(out, r)
			out = append(out, kept...)
		}
		i = j
	}
	return out
}

func rowMatches(r renderRow, q string) bool {
	var hay string
	switch r.kind {
	case rowTask:
		if r.task != nil {
			hay = strings.ToLower(r.task.extID + " " + r.task.project + " " +
				r.task.short + " " + r.task.statusKey)
		}
	case rowLog:
		if r.log != nil {
			hay = strings.ToLower(r.parentTaskID + " " + r.log.Note + " " +
				r.log.Time + " " + r.log.Date.Format("2006-01-02"))
		}
	}
	return hay != "" && strings.Contains(hay, q)
}
