package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/snezhinskiy/worklog/internal/config"
	"github.com/snezhinskiy/worklog/internal/mcpsrv"
	"github.com/snezhinskiy/worklog/internal/store"
	"github.com/snezhinskiy/worklog/internal/tui"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-version" || os.Args[1] == "version") {
		fmt.Printf("worklog %s\n", mcpsrv.Version)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		runMCP(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		sub := os.Args[1]
		if runCLI(sub, os.Args[2:]) {
			return
		}
		if sub != "open" {
			fmt.Fprintf(os.Stderr, "worklog: unknown command %q\n", sub)
			fmt.Fprintln(os.Stderr, "commands: log, today, pending, status, rename, hide, unhide, export, activity, mcp, open, version")
			os.Exit(2)
		}
		// "open" is an explicit alias for the TUI — strip it so the flag
		// parser below sees the rest as if it were called bare.
		os.Args = append(os.Args[:1], os.Args[2:]...)
	}
	dump := flag.Bool("dump", false, "render once with a fake window size and exit (no TUI loop)")
	w := flag.Int("w", 100, "width for --dump")
	h := flag.Int("h", 40, "height for --dump")
	group := flag.String("group", "day", "day|task — initial grouping in --dump mode")
	rng := flag.String("range", "today", "today|week|month — initial range in --dump mode")
	focus := flag.String("focus", "body", "group|range|project|body — initial focus in --dump mode")
	proj := flag.Int("proj", 0, "project filter index (0=all)")
	editIdx := flag.Int("edit", -1, "if >=0, open editor with this field focused")
	expand := flag.Bool("expand", false, "snapshot with first task expanded (by-task mode)")
	editLog := flag.Int("edit-log", -1, "in --dump: open editor on Nth note of the first task (0..)")
	palette := flag.String("palette", "", "if set, --dump opens the palette with this query (use \"/\" for default)")
	form := flag.String("form", "", "if set: log|task|project — open that form in --dump")
	picker := flag.String("picker", "", "if set: task|log|project — open that picker in --dump")
	pickerQ := flag.String("picker-q", "", "initial query for --picker")
	board := flag.Bool("board", false, "snapshot board view")
	cursorAt := flag.Int("cursor", 0, "in --dump: how many rows to move the cursor down before rendering (tests scroll)")
	search := flag.String("search", "", "if set, --dump opens the search bar pre-filled with this query")
	help := flag.Bool("help-overlay", false, "snapshot the help overlay")
	cfgPath := flag.String("config", config.DefaultPath(), "path to config.toml")
	dbPath := flag.String("db", store.DefaultPath(), "path to worklog.db (sqlite)")
	seed := flag.Bool("seed", false, "force-seed mock data even if DB has rows")
	reset := flag.Bool("reset", false, "delete the DB file before opening (DESTRUCTIVE)")
	demo := flag.Bool("demo", false, "skip the DB entirely; render in-memory mocks (snapshots/dev)")
	flag.Parse()

	cfg, err := config.LoadOrDefaults(*cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	tui.SetDefaultConfig(cfg)

	var db *store.DB
	if !*demo {
		if *reset {
			_ = os.Remove(*dbPath)
		}
		var err error
		db, err = store.Open(*dbPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "db:", err)
			os.Exit(1)
		}
		defer db.Close()
		seeded, err := tui.MaybeSeed(db, *seed)
		if err != nil {
			fmt.Fprintln(os.Stderr, "seed:", err)
			os.Exit(1)
		}
		if seeded && !*dump {
			fmt.Fprintln(os.Stderr, "seeded mock data into", *dbPath)
		}
	}

	if *dump {
		if *editIdx >= 0 {
			fmt.Print(tui.SnapshotEdit(*w, *h, *editIdx))
			return
		}
		if *expand {
			fmt.Print(tui.SnapshotExpanded(*w, *h, *rng))
			return
		}
		if *editLog >= 0 {
			fmt.Print(tui.SnapshotEditOnLog(*w, *h, *editLog))
			return
		}
		if *palette != "" {
			fmt.Print(tui.SnapshotPalette(*w, *h, *palette))
			return
		}
		if *form != "" {
			fmt.Print(tui.SnapshotForm(*w, *h, *form))
			return
		}
		if *picker != "" {
			fmt.Print(tui.SnapshotPicker(*w, *h, *picker, *pickerQ))
			return
		}
		if *board {
			fmt.Print(tui.SnapshotBoard(*w, *h))
			return
		}
		if *help {
			fmt.Print(tui.SnapshotHelp(*w, *h))
			return
		}
		if *search != "" {
			fmt.Print(tui.SnapshotSearch(*w, *h, *search))
			return
		}
		fmt.Print(tui.SnapshotWith(*w, *h, *group, *rng, *focus, *proj, *cursorAt))
		return
	}

	// No tea.WithMouseCellMotion — we don't handle MouseMsg anywhere, and
	// enabling mouse capture stops the terminal from running its own URL
	// detection (iTerm2 cmd+click, etc.) and from selecting text with the
	// mouse, which is annoying for paste-into-stand-up workflows.
	p := tea.NewProgram(tui.New(cfg, db), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
