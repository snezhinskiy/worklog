package tui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/snezhinskiy/worklog/internal/config"
)

// All visual styles are package-level so existing renderers can reference
// them as `chipActive.Render(...)` without plumbing a theme everywhere.
// They are populated by applyTheme(), which is called once in New() with
// the active config.

var (
	frame            lipgloss.Style
	titleBar         lipgloss.Style
	dim              lipgloss.Style
	muted            lipgloss.Style
	text             lipgloss.Style
	hours            lipgloss.Style
	day              lipgloss.Style
	total            lipgloss.Style
	chipActive       lipgloss.Style
	chipActiveSel    lipgloss.Style
	chipInactive     lipgloss.Style
	chipLabel        lipgloss.Style
	chipLabelFocused lipgloss.Style
	rowFocusBg       lipgloss.Style
	cursorMark       lipgloss.Style
	focusChevron     lipgloss.Style
	noteTime         lipgloss.Style
	noteText         lipgloss.Style
	rowSelected      lipgloss.Style
	rowNormal        lipgloss.Style

	colBorder lipgloss.Color
	colDim    lipgloss.Color
	colMuted  lipgloss.Color
	colText   lipgloss.Color
	colBright lipgloss.Color
	colAccent lipgloss.Color
	colChipBG lipgloss.Color
	colHours  lipgloss.Color

	statusStyle map[string]lipgloss.Style
	statusIcon  map[string]string
	statusLabel map[string]string
	statusOrder []string

	boardColumnKeys []string

	projectStyles map[string]lipgloss.Style
)

func init() { applyTheme(newTheme(config.Defaults())) }

// applyTheme rebuilds the package-level styles from a freshly-built theme
// (which itself reflects the active config).
func applyTheme(t *theme) {
	frame = t.frame
	titleBar = t.titleBar
	dim = t.dim
	muted = t.muted
	text = t.text
	hours = t.hours
	day = t.day
	total = t.total
	chipActive = t.chipActive
	chipActiveSel = t.chipActiveSel
	chipInactive = t.chipInactive
	chipLabel = t.chipLabel
	chipLabelFocused = t.chipLabelF
	rowFocusBg = t.rowFocusBg
	cursorMark = t.cursorMark
	focusChevron = t.focusChev
	noteTime = t.noteTime
	noteText = t.noteText
	rowSelected = t.rowSelected
	rowNormal = t.rowNormal

	c := t.cfg.UI.Colors
	colBorder = lipgloss.Color(c.Border)
	colDim = lipgloss.Color(c.Dim)
	colMuted = lipgloss.Color(c.Muted)
	colText = lipgloss.Color(c.Text)
	colBright = lipgloss.Color(c.Bright)
	colAccent = lipgloss.Color(c.Accent)
	colChipBG = lipgloss.Color(c.ChipBg)
	colHours = lipgloss.Color(c.Hours)

	statusStyle = make(map[string]lipgloss.Style, len(t.cfg.Statuses))
	statusIcon = make(map[string]string, len(t.cfg.Statuses))
	statusLabel = make(map[string]string, len(t.cfg.Statuses))
	statusOrder = nil
	boardColumnKeys = nil
	for _, s := range t.cfg.Statuses {
		statusStyle[s.Key] = lipgloss.NewStyle().Foreground(lipgloss.Color(s.Color))
		statusIcon[s.Key] = s.Icon
		statusLabel[s.Key] = s.Label
		statusOrder = append(statusOrder, s.Key)
		if s.OnBoard {
			boardColumnKeys = append(boardColumnKeys, s.Key)
		}
	}

	projectStyles = t.projectColors
}

// projectColor renders a project slug in its configured color (bold).
// Falls back to the bright accent for unknown slugs.
func projectColor(slug string) lipgloss.Style {
	if s, ok := projectStyles[slug]; ok {
		return s
	}
	return lipgloss.NewStyle().Foreground(colText).Bold(true)
}
