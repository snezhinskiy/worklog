package tui

// ── focus arrow keys ────────────────────────────────────────────────────────

func (m *Model) handleUp() {
	switch m.focus {
	case focusBody:
		if m.cursor == 0 {
			m.focus = focusProject
		} else {
			m.moveCursor(-1)
		}
	case focusProject:
		m.focus = focusRange
	case focusRange:
		m.focus = focusGroup
	case focusGroup:
		m.focus = focusView
	case focusView:
		// already top
	}
}

func (m *Model) handleDown() {
	switch m.focus {
	case focusView:
		m.focus = focusGroup
	case focusGroup:
		m.focus = focusRange
	case focusRange:
		m.focus = focusProject
	case focusProject:
		m.focus = focusBody
	case focusBody:
		m.moveCursor(+1)
	}
}

func (m *Model) handleLeft() {
	switch m.focus {
	case focusView:
		m.cycleView(false)
	case focusGroup:
		m.cycleGroup(-1, false)
	case focusRange:
		m.cycleRange(-1, false)
	case focusProject:
		m.cycleProject(-1, false)
	case focusBody:
		m.collapseFromCursor()
	}
}

func (m *Model) handleRight() {
	switch m.focus {
	case focusView:
		m.cycleView(false)
	case focusGroup:
		m.cycleGroup(+1, false)
	case focusRange:
		m.cycleRange(+1, false)
	case focusProject:
		m.cycleProject(+1, false)
	case focusBody:
		m.expandFromCursor()
	}
}

// ── expand / collapse ───────────────────────────────────────────────────────

func (m *Model) toggleView() {
	if m.viewMode == viewWorklog {
		m.viewMode = viewBoard
	} else {
		m.viewMode = viewWorklog
	}
}

// expandFromCursor: → on a task expands it and jumps to the first note;
// → on an already-expanded task or on a note row is a no-op.
func (m *Model) expandFromCursor() {
	if m.cursor >= len(m.view.selectableIdx) {
		return
	}
	rowIdx := m.view.selectableIdx[m.cursor]
	r := m.view.rows[rowIdx]
	if r.kind != rowTask || r.task == nil {
		return
	}
	if !m.expanded[r.task.extID] {
		m.expanded[r.task.extID] = true
		m.recompute()
	}
	if idx := m.firstNoteSelectableIdx(r.task.extID); idx >= 0 {
		m.cursor = idx
		m.adjustScroll()
	}
}

// collapseFromCursor: ← on a note collapses its parent task and moves cursor
// back to the task header. ← on a task collapses if expanded, else no-op.
func (m *Model) collapseFromCursor() {
	if m.cursor >= len(m.view.selectableIdx) {
		return
	}
	rowIdx := m.view.selectableIdx[m.cursor]
	r := m.view.rows[rowIdx]
	switch r.kind {
	case rowLog:
		taskID := r.parentTaskID
		m.expanded[taskID] = false
		m.recompute()
		if idx := m.taskSelectableIdx(taskID); idx >= 0 {
			m.cursor = idx
			m.adjustScroll()
		}
	case rowTask:
		if r.task != nil && m.expanded[r.task.extID] {
			m.expanded[r.task.extID] = false
			m.recompute()
		}
	}
}

// taskSelectableIdx returns the cursor index for the first task row with the
// given extID, or -1.
func (m *Model) taskSelectableIdx(extID string) int {
	for i, idx := range m.view.selectableIdx {
		r := m.view.rows[idx]
		if r.kind == rowTask && r.task != nil && r.task.extID == extID {
			return i
		}
	}
	return -1
}

// firstNoteSelectableIdx returns the cursor index for the first note row of
// the given task, or -1.
func (m *Model) firstNoteSelectableIdx(extID string) int {
	for i, idx := range m.view.selectableIdx {
		r := m.view.rows[idx]
		if r.kind == rowLog && r.parentTaskID == extID {
			return i
		}
	}
	return -1
}

// ── cycle helpers ───────────────────────────────────────────────────────────

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (m *Model) resetExpansion() {
	m.expanded = map[string]bool{}
}

// cycle handlers: arrow nav clamps (wrap=false), one-key shortcut wraps.
func wrapOrClamp(v, n, delta int, wrap bool) int {
	if wrap {
		return ((v+delta)%n + n) % n
	}
	return clamp(v+delta, 0, n-1)
}

func (m *Model) cycleGroup(delta int, wrap bool) {
	m.group = groupMode(wrapOrClamp(int(m.group), 2, delta, wrap))
	m.cursor = 0
	m.resetExpansion()
	m.recompute()
}

func (m *Model) cycleRange(delta int, wrap bool) {
	m.rng = rangeMode(wrapOrClamp(int(m.rng), 3, delta, wrap))
	m.cursor = 0
	m.resetExpansion()
	m.recompute()
}

func (m *Model) cycleProject(delta int, wrap bool) {
	m.projectFilter = wrapOrClamp(m.projectFilter, len(m.projects)+1, delta, wrap)
	m.cursor = 0
	m.resetExpansion()
	m.recompute()
}

func (m *Model) cycleView(wrap bool) {
	if wrap {
		m.viewMode = (m.viewMode + 1) % 2
	} else {
		// only two values — toggle
		m.toggleView()
	}
}

// ── cursor & scroll ─────────────────────────────────────────────────────────

func (m *Model) moveCursor(delta int) {
	n := len(m.view.selectableIdx)
	if n == 0 {
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= n {
		m.cursor = n - 1
	}
	m.adjustScroll()
}

// adjustScroll keeps the cursor row inside the visible window. Smooth-scroll:
// only shifts when the cursor would otherwise fall above the top or below the
// bottom of the body. Called after any cursor or layout change.
func (m *Model) adjustScroll() {
	total := len(m.view.rows)
	bodyH := m.bodyH
	if bodyH <= 0 || total <= bodyH {
		m.scrollOffset = 0
		return
	}
	if m.cursor < 0 || m.cursor >= len(m.view.selectableIdx) {
		// no valid cursor — leave scroll where it was, just clamp
	} else {
		cursorRow := m.view.selectableIdx[m.cursor]
		if cursorRow < m.scrollOffset {
			m.scrollOffset = cursorRow
		} else if cursorRow >= m.scrollOffset+bodyH {
			m.scrollOffset = cursorRow - bodyH + 1
		}
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	if maxOff := total - bodyH; m.scrollOffset > maxOff {
		m.scrollOffset = maxOff
	}
}
