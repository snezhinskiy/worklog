package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Update dispatches a single tea.Msg. Routing order is overlay → body:
// any active overlay (help, about, form, picker, palette, edit, search)
// consumes the key first; only when no overlay is open does the body
// keymap (cfg.Keys + the static keymap) run.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.Width = msg.Width
		// Body height budget = total - chrome (border + padding + chips
		// header + separators + footer). The exact figure is computed in
		// View(); this is the matching approximation Update can use to
		// keep scroll math consistent.
		m.bodyH = msg.Height - 13
		if m.bodyH < 5 {
			m.bodyH = 5
		}
		m.adjustScroll()
		return m, nil
	case tea.KeyMsg:
		// Allow Ctrl+C as global escape hatch from any overlay.
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		// Toast is purely informational — any keypress acknowledges it
		// without consuming the key (the real handler still runs below).
		m.toast = ""
		// Overlay routing: help > about > form > picker > palette > edit > body.
		if m.showHelp {
			switch msg.String() {
			case "esc", "?", "enter", " ":
				m.showHelp = false
			}
			return m, nil
		}
		if m.about != nil {
			cmd, _ := m.updateAbout(msg)
			return m, cmd
		}
		if m.form != nil {
			cmd, closed := m.updateForm(msg)
			if closed {
				m.form = nil
			}
			return m, cmd
		}
		if m.picker != nil {
			cmd, _ := m.updatePicker(msg)
			return m, cmd
		}
		if m.palette != nil {
			cmd, _ := m.updatePalette(msg)
			return m, cmd
		}
		if m.edit != nil {
			cmd := m.updateEdit(msg)
			return m, cmd
		}
		if m.search != nil {
			cmd, closed := m.updateSearch(msg)
			if closed {
				m.search = nil
			}
			return m, cmd
		}

		// Body keymap. Single-key shortcuts read from cfg.Keys so users
		// can rebind them via config.toml without touching the source.
		k := m.cfg.Keys
		s := msg.String()
		if k.Reload != "" && s == k.Reload {
			n := len(m.tasks)
			ll := len(m.logs)
			aa := len(m.activities)
			if err := m.reload(); err != nil {
				m.toast = "reload failed: " + err.Error()
			} else {
				m.toast = fmt.Sprintf("reloaded · %d tasks · %d logs · %d activities (was %d/%d/%d)",
					len(m.tasks), len(m.logs), len(m.activities), n, ll, aa)
			}
			return m, nil
		}
		if k.Palette != "" && s == k.Palette {
			m.palette = newPalette()
			return m, nil
		}
		if k.About != "" && s == k.About {
			m.about = &about{}
			return m, nil
		}
		if k.Move != "" && s == k.Move && m.focus == focusBody {
			m.openStatusPicker(m.currentTaskExtID())
			return m, nil
		}
		if k.Find != "" && s == k.Find && m.focus == focusBody && m.viewMode != viewBoard {
			m.openSearch()
			return m, nil
		}
		if k.Activity != "" && s == k.Activity && m.focus == focusBody {
			if extID := m.currentTaskExtID(); extID != "" {
				m.form = newActivityForm(&m, extID)
			}
			return m, nil
		}
		// Board mode hijacks body navigation; otherwise fall through to the
		// shared chip/body keymap below.
		if m.viewMode == viewBoard && m.focus == focusBody {
			if cmd, handled := m.boardKey(msg.String()); handled {
				return m, cmd
			}
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Help):
			m.showHelp = true

		// Single-key shortcut: first press moves focus to the row,
		// subsequent presses (focus already on the row) cycle the value.
		case key.Matches(msg, m.keys.View):
			if m.focus == focusView {
				m.cycleView(true)
			} else {
				m.focus = focusView
			}
		case key.Matches(msg, m.keys.Group):
			if m.focus == focusGroup {
				m.cycleGroup(+1, true)
			} else {
				m.focus = focusGroup
			}
		case key.Matches(msg, m.keys.Range):
			if m.focus == focusRange {
				m.cycleRange(+1, true)
			} else {
				m.focus = focusRange
			}
		case key.Matches(msg, m.keys.Proj):
			if m.focus == focusProject {
				m.cycleProject(+1, true)
			} else {
				m.focus = focusProject
			}

		case key.Matches(msg, m.keys.Tab):
			// Tab toggles between the header (chips) and the body, instead of
			// stepping through each chip one at a time. Returning to the
			// header lands on whichever chip was last active.
			if m.focus == focusBody {
				if m.lastHeaderFocus == focusBody {
					m.lastHeaderFocus = focusView
				}
				m.focus = m.lastHeaderFocus
			} else {
				m.lastHeaderFocus = m.focus
				m.focus = focusBody
			}

		case key.Matches(msg, m.keys.Enter):
			if m.focus == focusBody {
				m.enterEdit()
				if m.edit != nil {
					m.refocusEditInputs()
				}
			}

		case key.Matches(msg, m.keys.Up):
			m.handleUp()
		case key.Matches(msg, m.keys.Down):
			m.handleDown()
		case key.Matches(msg, m.keys.Left):
			m.handleLeft()
		case key.Matches(msg, m.keys.Right):
			m.handleRight()
		case key.Matches(msg, m.keys.PgDn):
			if m.focus == focusBody {
				m.moveCursor(5)
			}
		case key.Matches(msg, m.keys.PgUp):
			if m.focus == focusBody {
				m.moveCursor(-5)
			}
		}
	}
	return m, nil
}
