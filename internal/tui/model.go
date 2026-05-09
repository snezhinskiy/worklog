// Package tui is the bubbletea-based interactive interface. The package is
// split into:
//   - model.go     — types, Model state, lifecycle (New / reload / Init),
//                    pure read-only accessors.
//   - update.go    — the Update method: overlay routing + body keymap.
//   - nav.go       — cursor / scroll / cycle helpers driven by Update.
//   - snapshot.go  — Snapshot* renderers used by `worklog --dump` for
//                    headless screenshot tests.
//   - view.go      — the main View() (renders the worklog body).
//   - editor.go, palette.go, picker.go, search.go, forms.go, board.go,
//     activities.go, about.go, export.go, mock.go, theme.go, styles.go —
//     overlay/sub-component code, each largely self-contained.
package tui

import (
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/snezhinskiy/worklog/internal/config"
	"github.com/snezhinskiy/worklog/internal/domain"
	"github.com/snezhinskiy/worklog/internal/store"
)

// ── modes & focus ───────────────────────────────────────────────────────────

type groupMode int

const (
	groupByDay groupMode = iota
	groupByTask
)

func (g groupMode) String() string {
	return []string{"day", "task"}[g]
}

type rangeMode int

const (
	rangeToday rangeMode = iota
	rangeWeek
	rangeMonth
)

func (r rangeMode) String() string {
	return []string{"today", "week", "month"}[r]
}

type focusArea int

const (
	focusView focusArea = iota
	focusGroup
	focusRange
	focusProject
	focusBody
)

const focusCount = 5

type viewMode int

const (
	viewWorklog viewMode = iota
	viewBoard
)

func (v viewMode) String() string {
	return []string{"worklog", "board"}[v]
}

type boardCursor struct {
	col, row int
}

// ── model ───────────────────────────────────────────────────────────────────

type Model struct {
	cfg *config.Config
	db  domain.Store // when set, mutations go through the store + reload(); when nil, in-memory only

	projects   []Project
	tasks      []Task
	logs       []LogEntry
	activities []store.Activity

	width, height int

	group         groupMode
	rng           rangeMode
	projectFilter int // 0 = "all", else 1+i into projects
	cursor        int
	focus         focusArea
	// lastHeaderFocus remembers which chip the user was on before tabbing
	// down to the body, so tabbing back returns there instead of always
	// landing on the first chip.
	lastHeaderFocus focusArea

	today time.Time

	help help.Model
	keys keymap

	expanded map[string]bool // task extID → expanded (notes selectable)

	view     viewData
	edit     *editForm
	palette  *palette
	picker   *picker
	form     *cmdForm
	about    *about
	showHelp bool
	toast    string // single-line confirmation shown in the footer; cleared on the next keypress

	search      *searchBar // non-nil while the inline filter is open
	searchQuery string     // active filter; recompute() prunes rows that don't match
	paletteArgs string     // text after the command key in the palette, consumed by the next Open() callback

	// Scroll state for the worklog body. bodyH is the row-count budget set
	// by the most recent WindowSizeMsg (approximation of the View()-time
	// figure — we don't have access to it from Update); scrollOffset is
	// the index of the topmost rendered row in m.view.rows.
	bodyH        int
	scrollOffset int

	viewMode viewMode
	boardCur boardCursor
}

// ── keymap ──────────────────────────────────────────────────────────────────

type keymap struct {
	Up, Down, Left, Right, Tab key.Binding
	Enter                      key.Binding
	PgUp, PgDn                 key.Binding
	View, Group, Range, Proj   key.Binding
	Quit, Help                 key.Binding
}

func defaultKeymap() keymap {
	return keymap{
		// j/k/h/l also work but aren't shown in the footer to keep it tidy.
		Up:    key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑", "up")),
		Down:  key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓", "down")),
		Left:  key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←", "left")),
		Right: key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→", "right")),
		Tab:   key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "toggle header / body")),
		Enter: key.NewBinding(key.WithKeys("enter"), key.WithHelp("↵", "edit")),
		PgUp:  key.NewBinding(key.WithKeys("pgup", "ctrl+u"), key.WithHelp("PgUp", "page up")),
		PgDn:  key.NewBinding(key.WithKeys("pgdown", "ctrl+d"), key.WithHelp("PgDn", "page down")),
		View:  key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "cycle view")),
		Group: key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "cycle group")),
		Range: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "cycle range")),
		Proj:  key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "cycle project")),
		Help:  key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:  key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

func (k keymap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Left, k.Right, k.Enter, k.Help, k.Quit}
}

func (k keymap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Tab, k.PgUp, k.PgDn},
		{k.Group, k.Range},
		{k.Help, k.Quit},
	}
}

// ── lifecycle ───────────────────────────────────────────────────────────────

// activeCfg is set by SetDefaultConfig so Snapshot* helpers (which take no
// cfg argument) pick up whatever main loaded.
var activeCfg *config.Config

// SetDefaultConfig stores cfg as the fallback used by New(nil) and snapshots.
func SetDefaultConfig(cfg *config.Config) { activeCfg = cfg }

// New builds a fresh model. cfg may be nil — defaults (or the value set via
// SetDefaultConfig) are applied. When db is non-nil, data is loaded from it
// and mutations are persisted; when nil, the model stays in-memory with the
// mock seed (used by snapshot dumps).
func New(cfg *config.Config, db domain.Store) Model {
	if cfg == nil {
		if activeCfg != nil {
			cfg = activeCfg
		} else {
			cfg = config.Defaults()
		}
	}
	applyTheme(newTheme(cfg))

	m := Model{
		cfg:      cfg,
		db:       db,
		group:    groupByDay,
		rng:      rangeToday,
		focus:    focusBody,
		help:     help.New(),
		keys:     defaultKeymap(),
		expanded: map[string]bool{},
	}
	if db != nil {
		// real run: today is real today, data comes from DB
		now := time.Now()
		m.today = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		_ = m.reload()
	} else {
		// snapshot/demo: in-memory mock data with a frozen date
		m.today = mockToday()
		m.projects, m.tasks, m.logs, m.activities = mockData()
	}
	if cfg.UI.ViewDefault == "board" {
		m.viewMode = viewBoard
	}
	m.recompute()
	return m
}

// reload fetches projects/tasks/logs from the store into the model. No-op
// when db is nil. Called after every persisted mutation so the rendered
// list always reflects what's on disk.
func (m *Model) reload() error {
	if m.db == nil {
		return nil
	}
	// The main view never surfaces hidden rows. Pickers that need them
	// (e.g. the /unhide flow) call ListXxx(true) directly.
	projects, err := m.db.ListProjects(false)
	if err != nil {
		return err
	}
	tasks, err := m.db.ListTasks(false)
	if err != nil {
		return err
	}
	logs, err := m.db.ListLogs(false)
	if err != nil {
		return err
	}
	activities, err := m.db.ListActivities("", false)
	if err != nil {
		return err
	}
	m.projects = projects
	m.tasks = tasks
	m.logs = logs
	m.activities = activities
	m.recompute()
	return nil
}

func (m Model) Init() tea.Cmd { return nil }

// ── data slicing ────────────────────────────────────────────────────────────

func (m Model) rangeBounds() (time.Time, time.Time) {
	switch m.rng {
	case rangeToday:
		return m.today, m.today
	case rangeWeek:
		off := int(m.today.Weekday()) - 1
		if off < 0 {
			off = 6
		}
		from := m.today.AddDate(0, 0, -off)
		to := from.AddDate(0, 0, 6)
		return from, to
	case rangeMonth:
		// Rolling 30 days ending today, not the calendar month. Picks up
		// late-April work even when "today" is May 2; matches the mental
		// model of "what did I do this past month" for a personal tracker.
		from := m.today.AddDate(0, 0, -29)
		return from, m.today
	}
	return m.today, m.today
}

func (m Model) taskByID(id string) (Task, bool) {
	for _, t := range m.tasks {
		if t.ExternalID == id {
			return t, true
		}
	}
	return Task{}, false
}

func (m Model) activeProject() string {
	if m.projectFilter == 0 {
		return ""
	}
	return m.projects[m.projectFilter-1].Slug
}

// filteredLogs returns indices into m.logs for entries that match the active
// range and project filter, sorted chronologically.
func (m Model) filteredLogs() []int {
	from, to := m.rangeBounds()
	want := m.activeProject()
	var out []int
	for i, l := range m.logs {
		if l.Date.Before(from) || l.Date.After(to) {
			continue
		}
		if want != "" {
			if tk, ok := m.taskByID(l.TaskID); !ok || tk.Project != want {
				continue
			}
		}
		out = append(out, i)
	}
	sort.SliceStable(out, func(a, b int) bool {
		la, lb := m.logs[out[a]], m.logs[out[b]]
		if !la.Date.Equal(lb.Date) {
			return la.Date.Before(lb.Date)
		}
		return la.Time < lb.Time
	})
	return out
}
