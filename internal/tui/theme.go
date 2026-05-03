package tui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/snezhinskiy/worklog/internal/config"
)

// theme holds all lipgloss styles and lookups derived from the active config.
// Replaces the old hardcoded global maps in styles.go — those are kept around
// only as fallback/seed values for tests.
type theme struct {
	cfg *config.Config

	// chrome
	frame        lipgloss.Style
	titleBar     lipgloss.Style
	dim          lipgloss.Style
	muted        lipgloss.Style
	text         lipgloss.Style
	hours        lipgloss.Style
	day          lipgloss.Style
	total        lipgloss.Style
	chipActive    lipgloss.Style
	chipActiveSel lipgloss.Style // active chip on a row that currently has focus
	chipInactive  lipgloss.Style
	chipLabel     lipgloss.Style
	chipLabelF    lipgloss.Style
	rowFocusBg    lipgloss.Style
	cursorMark   lipgloss.Style
	focusChev    lipgloss.Style
	noteTime     lipgloss.Style
	noteText     lipgloss.Style
	rowSelected  lipgloss.Style
	rowNormal    lipgloss.Style

	// status
	statusByKey map[string]config.Status
	statusOrder []string
	boardCols   []string

	// project colors
	projectColors map[string]lipgloss.Style
}

func newTheme(cfg *config.Config) *theme {
	c := cfg.UI.Colors
	t := &theme{
		cfg: cfg,

		frame:        lipgloss.NewStyle().Border(borderStyle(cfg.UI.BorderStyle)).BorderForeground(lipgloss.Color(c.Border)).Padding(0, 1),
		titleBar:     lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(c.Bright)),
		dim:          lipgloss.NewStyle().Foreground(lipgloss.Color(c.Dim)),
		muted:        lipgloss.NewStyle().Foreground(lipgloss.Color(c.Muted)),
		text:         lipgloss.NewStyle().Foreground(lipgloss.Color(c.Text)),
		hours:        lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(c.Hours)),
		day:          lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(c.Bright)),
		total:        lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(c.Hours)),
		chipActive:    lipgloss.NewStyle().Background(lipgloss.Color(c.ChipBg)).Foreground(lipgloss.Color(c.Bright)).Bold(true).Padding(0, 1),
		chipActiveSel: lipgloss.NewStyle().Background(lipgloss.Color(c.Accent)).Foreground(lipgloss.Color(c.Bright)).Bold(true).Padding(0, 1),
		chipInactive:  lipgloss.NewStyle().Foreground(lipgloss.Color(c.Muted)).Padding(0, 1),
		chipLabel:    lipgloss.NewStyle().Foreground(lipgloss.Color(c.Muted)),
		chipLabelF:   lipgloss.NewStyle().Foreground(lipgloss.Color(c.Bright)).Bold(true),
		rowFocusBg:   lipgloss.NewStyle().Background(lipgloss.Color(c.RowFocus)),
		cursorMark:   lipgloss.NewStyle().Foreground(lipgloss.Color(c.Accent)).Bold(true),
		focusChev:    lipgloss.NewStyle().Foreground(lipgloss.Color(c.Accent)).Bold(true),
		noteTime:     lipgloss.NewStyle().Foreground(lipgloss.Color(c.Dim)),
		noteText:     lipgloss.NewStyle().Foreground(lipgloss.Color(c.Text)).Italic(true),
		rowSelected:  lipgloss.NewStyle().Foreground(lipgloss.Color(c.Bright)).Bold(true),
		rowNormal:    lipgloss.NewStyle().Foreground(lipgloss.Color(c.Text)),

		statusByKey:   make(map[string]config.Status, len(cfg.Statuses)),
		projectColors: make(map[string]lipgloss.Style, len(cfg.ProjectColors)),
	}
	for _, s := range cfg.Statuses {
		t.statusByKey[s.Key] = s
		t.statusOrder = append(t.statusOrder, s.Key)
		if s.OnBoard {
			t.boardCols = append(t.boardCols, s.Key)
		}
	}
	for slug, color := range cfg.ProjectColors {
		t.projectColors[slug] = lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true)
	}
	return t
}

func borderStyle(name string) lipgloss.Border {
	switch name {
	case "sharp":
		return lipgloss.NormalBorder()
	case "thick":
		return lipgloss.ThickBorder()
	case "none":
		return lipgloss.HiddenBorder()
	default:
		return lipgloss.RoundedBorder()
	}
}

// status returns the visual style for a status key (defaults to muted).
func (t *theme) status(key string) lipgloss.Style {
	if s, ok := t.statusByKey[key]; ok {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(s.Color))
	}
	return t.muted
}

// statusIcon returns the glyph (defaults to ?).
func (t *theme) statusIcon(key string) string {
	if s, ok := t.statusByKey[key]; ok {
		return s.Icon
	}
	return "?"
}

// statusLabel returns the human label.
func (t *theme) statusLabel(key string) string {
	if s, ok := t.statusByKey[key]; ok {
		return s.Label
	}
	return key
}

// project returns a colored style for a project slug; falls back to bright.
func (t *theme) project(slug string) lipgloss.Style {
	if s, ok := t.projectColors[slug]; ok {
		return s
	}
	return t.titleBar
}
