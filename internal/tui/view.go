package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type rowKind int

const (
	rowSection rowKind = iota
	rowTask
	rowLog
)

type renderRow struct {
	kind         rowKind
	text         string
	task         *taskCell
	log          *LogEntry
	parentTaskID string // for rowLog: which task this note belongs to
	logIdx       int    // for rowLog: index into Model.logs (-1 for non-log rows)
}

type taskCell struct {
	statusKey string
	extID     string
	project   string
	short     string
	hours     float64
	notes     []LogEntry // raw entries; renderer decides whether to show date
	noteIdxs  []int      // original indices in Model.logs, parallel to notes
}

type viewData struct {
	rows          []renderRow
	selectableIdx []int
}

func (m *Model) recompute() {
	v := viewData{}
	switch m.group {
	case groupByDay:
		v = m.buildByDay()
	case groupByTask:
		v = m.buildByTask()
	}
	if m.searchQuery != "" {
		v.rows = filterRowsByQuery(v.rows, m.searchQuery)
	}
	for i, r := range v.rows {
		if r.kind == rowTask || r.kind == rowLog {
			v.selectableIdx = append(v.selectableIdx, i)
		}
	}
	m.view = v
	if m.cursor >= len(v.selectableIdx) {
		m.cursor = len(v.selectableIdx) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.adjustScroll()
}

// ── grouping: by day ─────────────────────────────────────────────────────────

func (m *Model) buildByDay() viewData {
	logs := m.filteredLogs()

	type dayBucket struct {
		date  time.Time
		tasks map[string]*taskCell
		order []string
		total float64
	}
	days := map[string]*dayBucket{}
	var dayOrder []string
	for _, idx := range logs {
		l := m.logs[idx]
		key := l.Date.Format("2006-01-02")
		b, ok := days[key]
		if !ok {
			b = &dayBucket{date: l.Date, tasks: map[string]*taskCell{}}
			days[key] = b
			dayOrder = append(dayOrder, key)
		}
		t, ok := b.tasks[l.TaskID]
		if !ok {
			tk, _ := m.taskByID(l.TaskID)
			t = &taskCell{statusKey: tk.Status, extID: tk.ExternalID, project: tk.Project, short: tk.Short}
			b.tasks[l.TaskID] = t
			b.order = append(b.order, l.TaskID)
		}
		t.hours += l.Hours
		t.notes = append(t.notes, l)
		t.noteIdxs = append(t.noteIdxs, idx)
		b.total += l.Hours
	}
	sort.Strings(dayOrder)

	v := viewData{}
	for _, dk := range dayOrder {
		b := days[dk]
		v.rows = append(v.rows, renderRow{kind: rowSection, text: renderDayHeader(b.date, b.total)})
		for _, tid := range b.order {
			tc := *b.tasks[tid]
			v.rows = append(v.rows, renderRow{kind: rowTask, task: &tc, logIdx: -1})
			if m.expanded[tc.extID] {
				for i := range tc.notes {
					n := tc.notes[i]
					v.rows = append(v.rows, renderRow{
						kind:         rowLog,
						log:          &n,
						parentTaskID: tc.extID,
						logIdx:       tc.noteIdxs[i],
					})
				}
			}
		}
		v.rows = append(v.rows, renderRow{kind: rowSection, text: ""})
	}
	if len(v.rows) == 0 {
		v.rows = append(v.rows, renderRow{kind: rowSection, text: muted.Render("  No entries in this period.")})
	}
	return v
}

// ── grouping: flat by task ───────────────────────────────────────────────────

func (m *Model) buildByTask() viewData {
	logs := m.filteredLogs()
	cells := map[string]*taskCell{}
	var order []string
	var grand float64
	for _, idx := range logs {
		l := m.logs[idx]
		tk, _ := m.taskByID(l.TaskID)
		t, ok := cells[l.TaskID]
		if !ok {
			t = &taskCell{statusKey: tk.Status, extID: tk.ExternalID, project: tk.Project, short: tk.Short}
			cells[l.TaskID] = t
			order = append(order, l.TaskID)
		}
		t.hours += l.Hours
		t.notes = append(t.notes, l)
		t.noteIdxs = append(t.noteIdxs, idx)
		grand += l.Hours
	}
	sort.Slice(order, func(i, j int) bool { return cells[order[i]].hours > cells[order[j]].hours })

	v := viewData{}
	v.rows = append(v.rows, renderRow{kind: rowSection, text: renderFlatHeader(grand, m.rng)})
	for _, id := range order {
		tc := *cells[id]
		// sort notes by date+time but keep noteIdxs in lockstep
		type noteWithIdx struct {
			N   LogEntry
			Idx int
		}
		paired := make([]noteWithIdx, len(tc.notes))
		for i := range tc.notes {
			paired[i] = noteWithIdx{tc.notes[i], tc.noteIdxs[i]}
		}
		sort.SliceStable(paired, func(i, j int) bool {
			if !paired[i].N.Date.Equal(paired[j].N.Date) {
				return paired[i].N.Date.Before(paired[j].N.Date)
			}
			return paired[i].N.Time < paired[j].N.Time
		})
		for i := range paired {
			tc.notes[i] = paired[i].N
			tc.noteIdxs[i] = paired[i].Idx
		}
		v.rows = append(v.rows, renderRow{kind: rowTask, task: &tc, logIdx: -1})
		if m.expanded[tc.extID] {
			for i := range tc.notes {
				n := tc.notes[i]
				v.rows = append(v.rows, renderRow{
					kind:         rowLog,
					log:          &n,
					parentTaskID: tc.extID,
					logIdx:       tc.noteIdxs[i],
				})
			}
		}
	}
	if len(order) == 0 {
		v.rows = append(v.rows, renderRow{kind: rowSection, text: muted.Render("  No entries in this period.")})
	}
	return v
}

// ── headers / chrome ─────────────────────────────────────────────────────────

var weekdayName = map[time.Weekday]string{
	time.Monday: "Monday", time.Tuesday: "Tuesday", time.Wednesday: "Wednesday",
	time.Thursday: "Thursday", time.Friday: "Friday", time.Saturday: "Saturday", time.Sunday: "Sunday",
}
var weekdayShort = map[time.Weekday]string{
	time.Monday: "Mon", time.Tuesday: "Tue", time.Wednesday: "Wed",
	time.Thursday: "Thu", time.Friday: "Fri", time.Saturday: "Sat", time.Sunday: "Sun",
}
var monthName = map[time.Month]string{
	time.January: "Jan", time.February: "Feb", time.March: "Mar",
	time.April: "Apr", time.May: "May", time.June: "Jun", time.July: "Jul",
	time.August: "Aug", time.September: "Sep", time.October: "Oct",
	time.November: "Nov", time.December: "Dec",
}
var monthShort = monthName

func renderDayHeader(d time.Time, totalH float64) string {
	left := day.Render(fmt.Sprintf("%s · %d %s %d", weekdayName[d.Weekday()], d.Day(), monthName[d.Month()], d.Year()))
	right := total.Render(fmt.Sprintf("%.1fh", totalH))
	return left + "  " + muted.Render("·") + "  " + right
}

func renderFlatHeader(totalH float64, rng rangeMode) string {
	return day.Render("All tasks · "+rng.String()) + "  " + muted.Render("·") + "  " + total.Render(fmt.Sprintf("%.1fh", totalH))
}

// ── task row ─────────────────────────────────────────────────────────────────

func (m Model) renderTaskRow(tc *taskCell, selected, expanded, withDate bool) string {
	mark := "  "
	if selected {
		mark = cursorMark.Render("▸ ")
	}
	icon := statusStyle[tc.statusKey].Render(statusIcon[tc.statusKey])

	// Disclosure triangle: ▾ only when expanded — keeps cursor (▸) visually distinct.
	disclosure := "  "
	if expanded {
		disclosure = muted.Render("▾ ")
	}

	extStyle := rowNormal
	shortStyle := text
	if selected {
		extStyle = rowSelected
		shortStyle = rowSelected
	}
	ext := extStyle.Render(padRight(tc.extID, 8))
	proj := projectColor(tc.project).Render(padRight(tc.project, 8))
	hrs := hours.Render(fmt.Sprintf("%4.1fh", tc.hours))
	short := shortStyle.Render(tc.short)

	line := mark + disclosure + icon + "  " + ext + "  " + proj + "  " + hrs + "  " + short

	// When expanded, notes are separate selectable rows — skip the inline preview.
	if expanded || !selected || len(tc.notes) == 0 {
		return line
	}
	var b strings.Builder
	b.WriteString(line)
	for _, n := range tc.notes {
		b.WriteString("\n")
		var prefix string
		if withDate {
			d := fmt.Sprintf("%s %d %s", weekdayShort[n.Date.Weekday()], n.Date.Day(), monthShort[n.Date.Month()])
			prefix = noteTime.Render(padRight(d, 9)) + "  "
		}
		b.WriteString("           " + prefix +
			noteTime.Render(n.Time) + "  " +
			noteText.Render(n.Note) + "  " +
			muted.Render(fmt.Sprintf("%.1fh", n.Hours)))
	}
	return b.String()
}

// renderLogRow renders a single, individually selectable note row beneath an
// expanded task. Indent matches the inline preview so it lines up.
func (m Model) renderLogRow(l *LogEntry, selected, withDate bool) string {
	mark := "    "
	if selected {
		mark = "  " + cursorMark.Render("▸ ")
	}
	var datePart string
	if withDate {
		d := fmt.Sprintf("%s %d %s", weekdayShort[l.Date.Weekday()], l.Date.Day(), monthShort[l.Date.Month()])
		datePart = noteTime.Render(padRight(d, 9)) + "  "
	}
	noteStyle := noteText
	if selected {
		noteStyle = rowSelected
	}
	return mark + "       " + datePart +
		noteTime.Render(padRight(l.Time, 5)) + "  " +
		noteStyle.Render(l.Note) + "  " +
		muted.Render(fmt.Sprintf("%.1fh", l.Hours))
}

func padRight(s string, n int) string {
	if len([]rune(s)) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len([]rune(s)))
}

// ── chips ────────────────────────────────────────────────────────────────────

// renderChip draws a single chip. When the chip is the current value AND its
// row owns input focus, it gets a stronger (accent) background so the user
// can see exactly where the cursor sits.
func renderChip(label string, active, rowFocused bool) string {
	switch {
	case active && rowFocused:
		return chipActiveSel.Render(label)
	case active:
		return chipActive.Render(label)
	default:
		return chipInactive.Render(label)
	}
}

// renderChipRow renders one chip row with its label and a focus chevron.
// The focus indicator is the chevron + bold label; the active chip itself
// gets a stronger background (chipActiveSel, accent color) to show that the
// cursor is "sitting on" that value.
func (m Model) renderChipRow(label string, focused bool, chips ...string) string {
	chevron := "  "
	labelStyle := chipLabel
	if focused {
		chevron = focusChevron.Render("❯ ")
		labelStyle = chipLabelFocused
	}
	parts := []string{chevron + labelStyle.Render(padRight(label, 11))}
	parts = append(parts, chips...)
	return strings.Join(parts, "  ")
}

func (m Model) renderHeader(width int) string {
	title := titleBar.Render("worklog")
	from, to := m.rangeBounds()
	var sub string
	if from.Equal(to) {
		sub = fmt.Sprintf("%s · %d %s %d",
			weekdayName[from.Weekday()], from.Day(), monthName[from.Month()], from.Year())
	} else {
		sub = fmt.Sprintf("%d %s — %d %s %d",
			from.Day(), monthName[from.Month()],
			to.Day(), monthName[to.Month()], to.Year())
	}
	if want := m.activeProject(); want != "" {
		sub += "  " + muted.Render("·") + "  " + projectColor(want).Render(want)
	}
	titleLine := "  " + title + "  " + muted.Render("·") + "  " + day.Render(sub)

	viewFocus := m.focus == focusView
	groupFocus := m.focus == focusGroup
	rangeFocus := m.focus == focusRange
	projFocus := m.focus == focusProject

	viewLine := m.renderChipRow("View", viewFocus,
		renderChip("worklog", m.viewMode == viewWorklog, viewFocus),
		renderChip("board", m.viewMode == viewBoard, viewFocus),
	)
	groupLine := m.renderChipRow("Group by", groupFocus,
		renderChip("day", m.group == groupByDay, groupFocus),
		renderChip("task", m.group == groupByTask, groupFocus),
	)
	rangeLine := m.renderChipRow("Range", rangeFocus,
		renderChip("today", m.rng == rangeToday, rangeFocus),
		renderChip("week", m.rng == rangeWeek, rangeFocus),
		renderChip("month", m.rng == rangeMonth, rangeFocus),
	)
	projChips := []string{renderChip("all", m.projectFilter == 0, projFocus)}
	for i, p := range m.projects {
		projChips = append(projChips, renderChip(p.Slug, m.projectFilter == i+1, projFocus))
	}
	projectLine := m.renderChipRow("Projects", projFocus, projChips...)

	return lipgloss.JoinVertical(lipgloss.Left,
		titleLine,
		"",
		viewLine,
		groupLine,
		rangeLine,
		projectLine,
	)
}

// ── full View ────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	innerW := m.width - 4
	if innerW < 40 {
		innerW = 40
	}

	header := m.renderHeader(innerW)
	sep := dim.Render(strings.Repeat("─", innerW))

	headerH := lipgloss.Height(header)
	footer := m.renderFooter(innerW)
	footerH := lipgloss.Height(footer)
	bodyH := m.height - 2 - headerH - 2 - footerH - 2
	if bodyH < 5 {
		bodyH = 5
	}

	var body string
	switch {
	case m.showHelp:
		body = m.renderHelp(innerW)
	case m.about != nil:
		body = m.renderAbout(innerW)
	case m.form != nil:
		body = m.renderForm(innerW)
	case m.picker != nil:
		body = m.renderPicker(innerW)
	case m.palette != nil:
		body = m.renderPalette(innerW)
	case m.edit != nil:
		body = m.renderEditor(innerW)
	case m.viewMode == viewBoard:
		body = m.renderBoard(innerW)
	default:
		// The search bar (when open) is a sticky header on top of the
		// body — it must not be subject to scroll slicing.
		var searchHeader []string
		rowBudget := bodyH
		if m.search != nil {
			searchHeader = []string{
				m.renderSearchBar(innerW),
				dim.Render(strings.Repeat("─", innerW)),
			}
			rowBudget = bodyH - 2
			if rowBudget < 1 {
				rowBudget = 1
			}
		}

		cursorRowIdx := -1
		if len(m.view.selectableIdx) > 0 && m.cursor < len(m.view.selectableIdx) {
			cursorRowIdx = m.view.selectableIdx[m.cursor]
		}
		var rowLines []string
		for i, r := range m.view.rows {
			switch r.kind {
			case rowSection:
				rowLines = append(rowLines, r.text)
			case rowTask:
				selected := i == cursorRowIdx && m.focus == focusBody
				expanded := r.task != nil && m.expanded[r.task.extID]
				rowLines = append(rowLines, m.renderTaskRow(r.task, selected, expanded, m.group == groupByTask))
			case rowLog:
				selected := i == cursorRowIdx && m.focus == focusBody
				rowLines = append(rowLines, m.renderLogRow(r.log, selected, m.group == groupByTask))
			}
		}
		if m.searchQuery != "" && len(rowLines) == 0 {
			rowLines = []string{muted.Render("  (no matches)")}
		}

		// Apply scroll offset to the row content only. The exact rowBudget
		// matches what View() actually uses, so re-clamp the saved offset
		// (set by Update against an approximation) against it.
		off := m.scrollOffset
		if len(rowLines) > rowBudget {
			if maxOff := len(rowLines) - rowBudget; off > maxOff {
				off = maxOff
			}
			if off < 0 {
				off = 0
			}
			rowLines = rowLines[off : off+rowBudget]
		}

		body = strings.Join(append(searchHeader, rowLines...), "\n")
		body = clampHeight(body, bodyH)
	}
	_ = bodyH

	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		sep,
		body,
		sep,
		footer,
	)

	return frame.Width(m.width - 2).Render(content)
}

// renderFooter is a hand-rolled, single-line key hint strip.
// Keeps it short, English, and with arrow+word combined ("↑/up").
// When m.toast is set, the hint strip is replaced by the toast for one
// frame — it gets cleared on the next keypress.
func (m Model) renderFooter(width int) string {
	if m.toast != "" {
		return lipgloss.NewStyle().Foreground(colAccent).Render("✓ " + m.toast)
	}
	k := m.cfg.Keys
	items := []string{
		"↑/↓ up/down", "←/→ left/right",
		"↵ edit",
		k.Move + " move", k.Activity + " activity", k.Find + " find",
		k.Palette + " command", "? help", "q quit",
	}
	sep := muted.Render(" • ")
	var b strings.Builder
	for i, it := range items {
		if i > 0 {
			b.WriteString(sep)
		}
		b.WriteString(muted.Render(it))
	}
	return b.String()
}

func (m Model) renderHelp(width int) string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colBright)
	header := titleStyle.Render("Keys")

	section := func(title string, rows [][2]string) string {
		out := []string{lipgloss.NewStyle().Foreground(colMuted).Bold(true).Render(title)}
		for _, r := range rows {
			out = append(out, "  "+lipgloss.NewStyle().Foreground(colBright).Render(padRight(r[0], 18))+text.Render(r[1]))
		}
		return strings.Join(out, "\n")
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		section("navigation", [][2]string{
			{"↑/up   ↓/down", "move cursor / focus"},
			{"←/left →/right", "change chip · expand/collapse task"},
			{"tab", "toggle header / body"},
			{"PgUp / PgDn", "page up / page down"},
		}),
		"",
		section("actions", [][2]string{
			{"↵ enter", "edit the row under the cursor"},
			{m.cfg.Keys.Move, "move task to a different status"},
			{m.cfg.Keys.Activity, "add a typed activity (mr/commit/deploy/link/note)"},
			{m.cfg.Keys.Find, "find / filter the body"},
			{m.cfg.Keys.Palette, "command palette"},
			{m.cfg.Keys.About, "about"},
			{m.cfg.Keys.Reload, "reload from disk (sync changes from MCP / CLI)"},
			{"q · ctrl+c", "quit"},
		}),
		"",
		section("one-key cycles (wrap-around)", [][2]string{
			{"v", "cycle view (worklog · board)"},
			{"g", "cycle grouping (day · task)"},
			{"r", "cycle range (today · week · month)"},
			{"p", "cycle project filter"},
		}),
		"",
		section("on the board", [][2]string{
			{"↑↓ ←→", "move the cursor between cards / columns"},
			{"shift+← / shift+→", "move the selected card across columns"},
			{"s · S", "same — forward / backward (alias for shift+arrow)"},
			{"↵ enter", "edit the selected task"},
		}),
		"",
		section("inside editor / form", [][2]string{
			{"↑/↓", "previous / next field"},
			{"←/→", "cycle choice on choice fields"},
			{"↵ enter", "next field; submit on last"},
			{"esc", "cancel"},
			{"ctrl+enter · ctrl+s", "save"},
		}),
		"",
		muted.Render("press esc, ? or ↵ to close"),
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

func clampHeight(body string, maxLines int) string {
	lines := strings.Split(body, "\n")
	if len(lines) <= maxLines {
		return body
	}
	return strings.Join(lines[:maxLines], "\n")
}
