package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// boardCard is one tile on the kanban view.
type boardCard struct {
	task   Task
	hours  float64 // total hours logged across all time
	staleD int     // days since the task's status last changed
}

// buildBoardColumns groups non-archived tasks by status. Respects the project
// filter; tasks whose status isn't on the board (on_board = false) are skipped.
func (m *Model) buildBoardColumns() map[string][]boardCard {
	want := m.activeProject()
	hoursByTask := map[string]float64{}
	for _, l := range m.logs {
		hoursByTask[l.TaskID] += l.Hours
	}

	cols := map[string][]boardCard{}
	for _, t := range m.tasks {
		if t.Archived {
			continue
		}
		if want != "" && t.Project != want {
			continue
		}
		if !contains(boardColumnKeys, t.Status) {
			continue
		}
		stale := 0
		if !t.StatusChangedAt.IsZero() {
			stale = int(m.today.Sub(t.StatusChangedAt).Hours() / 24)
			if stale < 0 {
				stale = 0
			}
		}
		cols[t.Status] = append(cols[t.Status], boardCard{
			task:   t,
			hours:  hoursByTask[t.ExternalID],
			staleD: stale,
		})
	}
	// Stalest cards float to the top so the angry red ones nag first.
	for k := range cols {
		sort.SliceStable(cols[k], func(i, j int) bool {
			if cols[k][i].staleD != cols[k][j].staleD {
				return cols[k][i].staleD > cols[k][j].staleD
			}
			return cols[k][i].task.ExternalID < cols[k][j].task.ExternalID
		})
	}
	return cols
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// ── stale styling ───────────────────────────────────────────────────────────

var (
	staleWarnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true) // orange
	staleAlertStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true) // red
)

// staleBadge renders e.g. "⚠ 12d" or "2d" or "" depending on age + thresholds.
// Thresholds may be overridden per-status via cfg.StaleFor.
func (m Model) staleBadge(statusKey string, days int) string {
	if days <= 0 {
		return ""
	}
	th := m.cfg.StaleFor(statusKey)
	switch {
	case th.AlertDays > 0 && days >= th.AlertDays:
		return staleAlertStyle.Render(fmt.Sprintf("⚠ %dd", days))
	case th.WarnDays > 0 && days >= th.WarnDays:
		return staleWarnStyle.Render(fmt.Sprintf("%dd", days))
	default:
		return muted.Render(fmt.Sprintf("%dd", days))
	}
}

// staleBar returns a colored left accent bar character matching the staleness.
func (m Model) staleBar(statusKey string, days int, selected bool) string {
	th := m.cfg.StaleFor(statusKey)
	switch {
	case th.AlertDays > 0 && days >= th.AlertDays:
		return staleAlertStyle.Render("▎")
	case th.WarnDays > 0 && days >= th.WarnDays:
		return staleWarnStyle.Render("▎")
	case selected:
		return cursorMark.Render("▎")
	default:
		return " "
	}
}

// ── render ──────────────────────────────────────────────────────────────────

const (
	boardMinColW = 18
	boardGap     = 2
)

// renderBoard is the entrypoint used by view.go when viewMode == viewBoard.
func (m Model) renderBoard(width int) string {
	cols := (&m).buildBoardColumns()
	n := len(boardColumnKeys)
	if n == 0 {
		return muted.Render("No statuses with on_board = true in config.")
	}

	totalGap := boardGap * (n - 1)
	colW := (width - totalGap) / n

	if colW < boardMinColW {
		return m.renderBoardVertical(width, cols)
	}

	var rendered []string
	for ci, key := range boardColumnKeys {
		rendered = append(rendered, m.renderBoardColumn(key, cols[key], colW, ci))
	}
	parts := joinWithGap(rendered, boardGap)
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func joinWithGap(cols []string, gap int) []string {
	if gap <= 0 {
		return cols
	}
	out := make([]string, 0, len(cols)*2-1)
	pad := strings.Repeat(" ", gap)
	for i, c := range cols {
		if i > 0 {
			out = append(out, pad)
		}
		out = append(out, c)
	}
	return out
}

func (m Model) renderBoardColumn(statusKey string, cards []boardCard, width, colIdx int) string {
	st := statusStyle[statusKey]
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		st.Bold(true).Render(statusIcon[statusKey]+" "+statusLabel[statusKey]),
		"  ",
		muted.Render(fmt.Sprintf("· %d", len(cards))),
	)
	separator := dim.Render(strings.Repeat("─", width))

	if len(cards) == 0 {
		empty := muted.Render("  (empty)")
		return lipgloss.NewStyle().Width(width).Render(
			lipgloss.JoinVertical(lipgloss.Left, header, separator, "", empty),
		)
	}

	var cardLines []string
	for ri, c := range cards {
		selected := m.focus == focusBody && colIdx == m.boardCur.col && ri == m.boardCur.row
		cardLines = append(cardLines, m.renderBoardCard(c, width, selected))
		cardLines = append(cardLines, "")
	}
	body := strings.Join(cardLines, "\n")
	return lipgloss.NewStyle().Width(width).Render(
		lipgloss.JoinVertical(lipgloss.Left, header, separator, "", body),
	)
}

// renderBoardCard draws one card filling the given width.
func (m Model) renderBoardCard(c boardCard, width int, selected bool) string {
	bar := m.staleBar(c.task.Status, c.staleD, selected)

	idStyle := lipgloss.NewStyle().Bold(true).Foreground(colBright)
	if selected {
		idStyle = idStyle.Foreground(colAccent)
	}
	id := idStyle.Render(c.task.ExternalID)
	badge := m.staleBadge(c.task.Status, c.staleD)

	gap := width - 2 - lipgloss.Width(id) - lipgloss.Width(badge)
	if gap < 1 {
		gap = 1
	}
	titleLine := id + strings.Repeat(" ", gap) + badge

	titleStyle := text
	if selected {
		titleStyle = rowSelected
	}
	title := wrapTwoLines(c.task.Short, width-2, titleStyle)

	proj := projectColor(c.task.Project).Render(c.task.Project)
	hrs := ""
	if c.hours > 0 {
		hrs = hours.Render(fmt.Sprintf("%.1fh", c.hours))
	}
	footGap := width - 2 - lipgloss.Width(proj) - lipgloss.Width(hrs)
	if footGap < 1 {
		footGap = 1
	}
	footer := proj + strings.Repeat(" ", footGap) + hrs

	body := lipgloss.JoinVertical(lipgloss.Left, titleLine, title, footer)
	var out []string
	for _, ln := range strings.Split(body, "\n") {
		out = append(out, bar+" "+ln)
	}
	return strings.Join(out, "\n")
}

// wrapTwoLines word-wraps s into at most 2 visible lines, ellipsizing.
func wrapTwoLines(s string, width int, st lipgloss.Style) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}
	if width < 4 {
		width = 4
	}
	var lines []string
	cur := ""
	for _, w := range words {
		if cur == "" {
			cur = w
			continue
		}
		if lipgloss.Width(cur)+1+lipgloss.Width(w) <= width {
			cur += " " + w
			continue
		}
		lines = append(lines, cur)
		cur = w
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	if len(lines) > 2 {
		lines = lines[:2]
		last := []rune(lines[1])
		// trim until it fits with ellipsis
		for lipgloss.Width(string(last)+"…") > width && len(last) > 0 {
			last = last[:len(last)-1]
		}
		lines[1] = string(last) + "…"
	}
	for i := range lines {
		lines[i] = st.Render(lines[i])
	}
	return strings.Join(lines, "\n")
}

// renderBoardVertical is the fallback for narrow terminals.
func (m Model) renderBoardVertical(width int, cols map[string][]boardCard) string {
	var sections []string
	for ci, key := range boardColumnKeys {
		st := statusStyle[key]
		sections = append(sections,
			st.Bold(true).Render(statusIcon[key]+" "+statusLabel[key])+
				muted.Render(fmt.Sprintf("  · %d", len(cols[key]))))
		sections = append(sections, dim.Render(strings.Repeat("─", width)))
		if len(cols[key]) == 0 {
			sections = append(sections, muted.Render("  (empty)"))
		} else {
			for ri, c := range cols[key] {
				selected := m.focus == focusBody && ci == m.boardCur.col && ri == m.boardCur.row
				mark := "  "
				if selected {
					mark = cursorMark.Render("▸ ")
				}
				ext := lipgloss.NewStyle().Foreground(colBright).Bold(true).Render(padRight(c.task.ExternalID, 10))
				proj := projectColor(c.task.Project).Render(padRight(c.task.Project, 8))
				hrs := hours.Render(fmt.Sprintf("%4.1fh", c.hours))
				badge := m.staleBadge(c.task.Status, c.staleD)
				sections = append(sections,
					mark+ext+proj+"  "+hrs+"  "+text.Render(c.task.Short)+"  "+badge)
			}
		}
		sections = append(sections, "")
	}
	return strings.Join(sections, "\n")
}

// ── move shortcut ───────────────────────────────────────────────────────────

// currentTaskExtID returns the external_id of the task under the body or
// board cursor, or "" if the cursor isn't on a task (e.g. group header).
// Used by the `m` shortcut so the same helper covers both view modes.
func (m *Model) currentTaskExtID() string {
	if m.viewMode == viewBoard {
		cols := m.buildBoardColumns()
		if m.boardCur.col < 0 || m.boardCur.col >= len(boardColumnKeys) {
			return ""
		}
		cards := cols[boardColumnKeys[m.boardCur.col]]
		if m.boardCur.row < 0 || m.boardCur.row >= len(cards) {
			return ""
		}
		return cards[m.boardCur.row].task.ExternalID
	}
	if m.cursor < 0 || m.cursor >= len(m.view.selectableIdx) {
		return ""
	}
	rowIdx := m.view.selectableIdx[m.cursor]
	r := m.view.rows[rowIdx]
	switch r.kind {
	case rowLog:
		return r.parentTaskID
	case rowTask:
		if r.task != nil {
			return r.task.extID
		}
	}
	return ""
}

// openStatusPicker installs a picker listing all configured statuses; on
// confirm it sets the task's status and reloads. Persists via store.DB if
// present, falls back to in-memory mutation for snapshot/demo runs.
func (m *Model) openStatusPicker(extID string) {
	if extID == "" {
		return
	}
	items := make([]pickerItem, 0, len(statusOrder))
	for _, st := range statusOrder {
		chip := statusStyle[st].Render(statusIcon[st] + " " + st)
		items = append(items, pickerItem{
			display:  chip,
			haystack: strings.ToLower(st),
			valueStr: st,
		})
	}
	m.picker = newPicker("Move "+extID+" to status", "↑/↓ · enter · esc",
		items,
		func(m *Model, it pickerItem) {
			if m.db != nil {
				_ = m.db.SetTaskStatus(extID, it.valueStr)
				_ = m.reload()
				return
			}
			for i := range m.tasks {
				if m.tasks[i].ExternalID == extID {
					m.tasks[i].Status = it.valueStr
					m.tasks[i].StatusChangedAt = m.today
					break
				}
			}
			m.recompute()
		})
}

// ── navigation ──────────────────────────────────────────────────────────────

// boardKey routes a key when board view + body focus are both active.
// Returns (cmd, handled). When not handled, the caller falls through to the
// shared keymap.
func (m *Model) boardKey(s string) (tea.Cmd, bool) {
	mod := m.cfg.Keys.BoardMoveModifier
	if mod == "" {
		mod = "shift"
	}
	switch s {
	case mod + "+left":
		m.boardMoveCard(-1)
		return nil, true
	case mod + "+right":
		m.boardMoveCard(+1)
		return nil, true
	case "left", "h":
		m.boardMove(-1, 0)
		return nil, true
	case "right", "l":
		m.boardMove(+1, 0)
		return nil, true
	case "up", "k":
		m.boardMove(0, -1)
		return nil, true
	case "down", "j":
		m.boardMove(0, +1)
		return nil, true
	case "enter":
		m.openBoardEditor()
		return nil, true
	case "s":
		m.boardMoveCard(+1)
		return nil, true
	case "S":
		m.boardMoveCard(-1)
		return nil, true
	}
	return nil, false
}

func (m *Model) boardMove(dx, dy int) {
	cols := m.buildBoardColumns()
	n := len(boardColumnKeys)
	if n == 0 {
		return
	}
	newCol := clamp(m.boardCur.col+dx, 0, n-1)
	colCards := cols[boardColumnKeys[newCol]]
	if len(colCards) == 0 {
		m.boardCur = boardCursor{col: newCol, row: 0}
		return
	}
	newRow := clamp(m.boardCur.row+dy, 0, len(colCards)-1)
	if dx != 0 {
		// when changing column, just clamp existing row to new column's size
		newRow = clamp(m.boardCur.row, 0, len(colCards)-1)
	}
	m.boardCur = boardCursor{col: newCol, row: newRow}
}

// boardMoveCard shifts the selected card to the previous/next on-board column
// (skipping any statuses that exist in config but aren't on_board=true).
// StatusChangedAt resets so the staleness counter starts over. The board
// cursor follows the card to its new column.
func (m *Model) boardMoveCard(delta int) {
	cols := m.buildBoardColumns()
	if m.boardCur.col >= len(boardColumnKeys) {
		return
	}
	colKey := boardColumnKeys[m.boardCur.col]
	colCards := cols[colKey]
	if m.boardCur.row >= len(colCards) {
		return
	}
	card := colCards[m.boardCur.row]
	newColIdx := clamp(m.boardCur.col+delta, 0, len(boardColumnKeys)-1)
	if newColIdx == m.boardCur.col {
		return
	}
	newStatus := boardColumnKeys[newColIdx]
	if m.db != nil {
		_ = m.db.SetTaskStatus(card.task.ExternalID, newStatus)
		_ = m.reload()
	} else {
		for i := range m.tasks {
			if m.tasks[i].ExternalID == card.task.ExternalID {
				m.tasks[i].Status = newStatus
				m.tasks[i].StatusChangedAt = m.today
				break
			}
		}
		m.recompute()
	}
	// Recompute and place the cursor on the moved card in the new column.
	cols = m.buildBoardColumns()
	newCol := cols[newStatus]
	newRow := 0
	for i, c := range newCol {
		if c.task.ExternalID == card.task.ExternalID {
			newRow = i
			break
		}
	}
	m.boardCur = boardCursor{col: newColIdx, row: newRow}
}

func (m *Model) openBoardEditor() {
	cols := m.buildBoardColumns()
	if m.boardCur.col >= len(boardColumnKeys) {
		return
	}
	colCards := cols[boardColumnKeys[m.boardCur.col]]
	if m.boardCur.row >= len(colCards) {
		return
	}
	m.form = newEditTaskForm(m, colCards[m.boardCur.row].task.ExternalID)
}
