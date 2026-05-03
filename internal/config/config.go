// Package config holds user-tunable settings: status definitions, colors,
// thresholds, defaults. Loaded from ~/.config/worklog/config.toml on top of
// baked-in defaults; missing file or any field is fine (defaults win).
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	UI            UI                   `toml:"ui"`
	Stale         StaleKind            `toml:"stale"`
	StalePerStat  map[string]StaleKind `toml:"stale_per_status"`
	Statuses      []Status             `toml:"status"`
	ProjectColors map[string]string    `toml:"project_colors"`
	Keys          Keybindings          `toml:"keys"`
}

// Keybindings exposes the user-tunable shortcuts. Each field is the literal
// key string that bubbletea reports (e.g. "ctrl+r", "m", "/"). Empty values
// fall back to Defaults() in mergeConfig — so a partial config.toml just
// overrides what it sets.
type Keybindings struct {
	// BoardMoveModifier is the modifier paired with ←/→ on the board to move
	// the selected card between status columns. One of: "shift", "ctrl", "alt".
	BoardMoveModifier string `toml:"board_move_modifier"`

	Reload   string `toml:"reload"`   // re-read DB into the model (default: ctrl+r)
	Palette  string `toml:"palette"`  // open the command palette (default: /)
	About    string `toml:"about"`    // open the about overlay (default: i)
	Move     string `toml:"move"`     // open the status-move picker (default: m)
	Activity string `toml:"activity"` // open the new-activity form (default: a)
	Find     string `toml:"find"`     // open the inline search bar (default: f)
}

type UI struct {
	ViewDefault    string  `toml:"view_default"`     // worklog | board
	DayTargetHours float64 `toml:"day_target_hours"`
	BorderStyle    string  `toml:"border_style"`     // rounded | sharp | thick | none
	Colors         Colors  `toml:"colors"`
}

// Colors are lipgloss color strings: ANSI 256 ("220") or hex ("#FFAA00").
type Colors struct {
	Accent   string `toml:"accent"`
	Bright   string `toml:"bright"`
	Text     string `toml:"text"`
	Muted    string `toml:"muted"`
	Dim      string `toml:"dim"`
	Border   string `toml:"border"`
	Hours    string `toml:"hours"`
	ChipBg   string `toml:"chip_bg"`
	RowFocus string `toml:"row_focus"` // bg of the currently-focused header row
}

type StaleKind struct {
	WarnDays  int `toml:"warn_days"`
	AlertDays int `toml:"alert_days"`
}

type Status struct {
	Key     string `toml:"key"`      // canonical id, lower_snake_case
	Label   string `toml:"label"`    // display label (e.g. "IN PROGRESS")
	Icon    string `toml:"icon"`     // single glyph
	Color   string `toml:"color"`    // lipgloss color
	OnBoard bool   `toml:"on_board"` // included as a column on the kanban view
}

// Defaults returns the baked-in defaults.
func Defaults() *Config {
	return &Config{
		UI: UI{
			ViewDefault:    "worklog",
			DayTargetHours: 8,
			BorderStyle:    "rounded",
			Colors: Colors{
				Accent:   "212",
				Bright:   "231",
				Text:     "252",
				Muted:    "244",
				Dim:      "240",
				Border:   "240",
				Hours:    "220",
				ChipBg:   "63",
				RowFocus: "237",
			},
		},
		Keys: Keybindings{
			BoardMoveModifier: "shift",
			Reload:            "ctrl+r",
			Palette:           "/",
			About:             "i",
			Move:              "m",
			Activity:          "a",
			Find:              "f",
		},
		Stale: StaleKind{WarnDays: 2, AlertDays: 3},
		Statuses: []Status{
			{Key: "backlog", Label: "BACKLOG", Icon: "◌", Color: "241", OnBoard: true},
			{Key: "todo", Label: "TODO", Icon: "○", Color: "244", OnBoard: true},
			{Key: "in_progress", Label: "IN PROGRESS", Icon: "◔", Color: "220", OnBoard: true},
			{Key: "to_test", Label: "TO TEST", Icon: "◐", Color: "117", OnBoard: true},
			{Key: "to_deploy", Label: "TO DEPLOY", Icon: "◕", Color: "75", OnBoard: true},
			{Key: "done", Label: "DONE", Icon: "●", Color: "42", OnBoard: false},
			{Key: "on_hold", Label: "ON HOLD", Icon: "⏸", Color: "245", OnBoard: false},
		},
		ProjectColors: map[string]string{
			"AURA":    "33",
			"WORKLOG": "207",
			"SHOP":    "48",
			"INFRA":   "214",
		},
	}
}

// LoadOrDefaults reads the config from path; if the file is missing it returns
// the bare defaults (no error). Anything present in the file overrides the
// matching default field; the Statuses slice is special-cased — if the user
// declares any [[status]], their list fully replaces the defaults.
func LoadOrDefaults(path string) (*Config, error) {
	cfg := Defaults()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	// Decode into a temp struct so we can distinguish "absent" from "zero".
	var raw Config
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	merge(cfg, &raw)
	return cfg, nil
}

// merge applies non-zero fields of src onto dst. Slices and maps from src,
// when non-empty, replace dst's. Color fields override individually.
func merge(dst, src *Config) {
	// UI
	if src.UI.ViewDefault != "" {
		dst.UI.ViewDefault = src.UI.ViewDefault
	}
	if src.UI.DayTargetHours != 0 {
		dst.UI.DayTargetHours = src.UI.DayTargetHours
	}
	if src.UI.BorderStyle != "" {
		dst.UI.BorderStyle = src.UI.BorderStyle
	}
	mergeColor(&dst.UI.Colors.Accent, src.UI.Colors.Accent)
	mergeColor(&dst.UI.Colors.Bright, src.UI.Colors.Bright)
	mergeColor(&dst.UI.Colors.Text, src.UI.Colors.Text)
	mergeColor(&dst.UI.Colors.Muted, src.UI.Colors.Muted)
	mergeColor(&dst.UI.Colors.Dim, src.UI.Colors.Dim)
	mergeColor(&dst.UI.Colors.Border, src.UI.Colors.Border)
	mergeColor(&dst.UI.Colors.Hours, src.UI.Colors.Hours)
	mergeColor(&dst.UI.Colors.ChipBg, src.UI.Colors.ChipBg)
	mergeColor(&dst.UI.Colors.RowFocus, src.UI.Colors.RowFocus)

	// Stale
	if src.Stale.WarnDays != 0 {
		dst.Stale.WarnDays = src.Stale.WarnDays
	}
	if src.Stale.AlertDays != 0 {
		dst.Stale.AlertDays = src.Stale.AlertDays
	}
	if len(src.StalePerStat) > 0 {
		dst.StalePerStat = src.StalePerStat
	}

	// Statuses replace wholesale (user redefines workflow)
	if len(src.Statuses) > 0 {
		dst.Statuses = src.Statuses
	}

	// Project colors merge per key
	if len(src.ProjectColors) > 0 {
		if dst.ProjectColors == nil {
			dst.ProjectColors = map[string]string{}
		}
		for k, v := range src.ProjectColors {
			dst.ProjectColors[k] = v
		}
	}

	mergeKey(&dst.Keys.BoardMoveModifier, src.Keys.BoardMoveModifier)
	mergeKey(&dst.Keys.Reload, src.Keys.Reload)
	mergeKey(&dst.Keys.Palette, src.Keys.Palette)
	mergeKey(&dst.Keys.About, src.Keys.About)
	mergeKey(&dst.Keys.Move, src.Keys.Move)
	mergeKey(&dst.Keys.Activity, src.Keys.Activity)
	mergeKey(&dst.Keys.Find, src.Keys.Find)
}

func mergeKey(dst *string, src string) {
	if src != "" {
		*dst = src
	}
}

func mergeColor(dst *string, src string) {
	if src != "" {
		*dst = src
	}
}

// DefaultPath returns ~/.config/worklog/config.toml (XDG-aware).
func DefaultPath() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "worklog", "config.toml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.toml"
	}
	return filepath.Join(home, ".config", "worklog", "config.toml")
}

// StatusByKey returns the status definition for k, or nil.
func (c *Config) StatusByKey(k string) *Status {
	for i := range c.Statuses {
		if c.Statuses[i].Key == k {
			return &c.Statuses[i]
		}
	}
	return nil
}

// StaleFor returns the threshold for a given status, falling back to global.
func (c *Config) StaleFor(status string) StaleKind {
	if s, ok := c.StalePerStat[status]; ok {
		// merge missing fields with global
		if s.WarnDays == 0 {
			s.WarnDays = c.Stale.WarnDays
		}
		if s.AlertDays == 0 {
			s.AlertDays = c.Stale.AlertDays
		}
		return s
	}
	return c.Stale
}
