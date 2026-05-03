package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/snezhinskiy/worklog/internal/domain"
	"github.com/snezhinskiy/worklog/internal/report"
	"github.com/snezhinskiy/worklog/internal/store"
)

// runCLI dispatches non-TUI subcommands. Returns true if the caller should
// stop (a subcommand handled the invocation); false if main should fall
// through to the TUI flow. Unknown subcommands fall through too — the user
// might still be invoking the legacy flag-only mode.
func runCLI(name string, args []string) bool {
	switch name {
	case "log":
		cmdLog(args)
	case "today":
		cmdToday(args)
	case "pending":
		cmdPending(args)
	case "status":
		cmdStatus(args)
	case "rename":
		cmdRename(args)
	case "hide":
		cmdHide(args, true)
	case "unhide":
		cmdHide(args, false)
	case "export":
		cmdExport(args)
	case "activity":
		cmdActivity(args)
	case "open":
		// Explicit alias — fall through so main launches the TUI.
		return false
	default:
		return false
	}
	return true
}

func cmdLog(args []string) {
	fs := flag.NewFlagSet("worklog log", flag.ExitOnError)
	dbPath := fs.String("db", store.DefaultPath(), "path to worklog.db")
	date := fs.String("date", "", "YYYY-MM-DD (default: today)")
	at := fs.String("time", "", "HH:MM (default: now)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: worklog log <task-id> <hours> [note...]")
		fmt.Fprintln(fs.Output(), "  hours: 2, 1.5, 2h, 30m, 1h30m")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)
	rest := fs.Args()
	if len(rest) < 2 {
		fs.Usage()
		os.Exit(2)
	}
	taskID := rest[0]
	hours, err := domain.ParseHours(rest[1])
	if err != nil {
		failCLI(err)
	}
	note := strings.Join(rest[2:], " ")

	d, err := parseDateOpt(*date)
	if err != nil {
		failCLI(err)
	}

	db := openDBOrFail(*dbPath)
	defer db.Close()

	entry, err := db.CreateLog(store.LogEntry{
		TaskID: taskID, Hours: hours, Date: d, Time: *at, Note: note,
	})
	if err != nil {
		failCLI(err)
	}
	fmt.Printf("logged %s · %s %s · %gh", entry.TaskID,
		entry.Date.Format("2006-01-02"), entry.Time, entry.Hours)
	if note != "" {
		fmt.Printf("  %s", note)
	}
	fmt.Println()
}

func cmdToday(args []string) {
	fs := flag.NewFlagSet("worklog today", flag.ExitOnError)
	dbPath := fs.String("db", store.DefaultPath(), "path to worklog.db")
	date := fs.String("date", "", "YYYY-MM-DD (default: today)")
	_ = fs.Parse(args)

	day, err := parseDateOpt(*date)
	if err != nil {
		failCLI(err)
	}
	if day.IsZero() {
		day = startOfDay(time.Now())
	}

	db := openDBOrFail(*dbPath)
	defer db.Close()

	logs, err := db.ListLogs(false)
	if err != nil {
		failCLI(err)
	}
	tasks, err := db.ListTasks(false)
	if err != nil {
		failCLI(err)
	}
	titles := map[string]string{}
	for _, t := range tasks {
		titles[t.ExternalID] = t.Short
	}

	type row struct {
		taskID string
		total  float64
		notes  []string
	}
	groups := map[string]*row{}
	var total float64
	for _, l := range logs {
		if !sameLocalDay(l.Date, day) {
			continue
		}
		r, ok := groups[l.TaskID]
		if !ok {
			r = &row{taskID: l.TaskID}
			groups[l.TaskID] = r
		}
		r.total += l.Hours
		if l.Note != "" {
			r.notes = append(r.notes, l.Note)
		}
		total += l.Hours
	}

	fmt.Printf("%s · %gh\n", day.Format("Mon · 2 Jan 2006"), total)
	if len(groups) == 0 {
		fmt.Println("  (nothing logged)")
		return
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return groups[keys[i]].total > groups[keys[j]].total
	})
	for _, k := range keys {
		r := groups[k]
		fmt.Printf("  %-10s %5sh  %s\n", r.taskID,
			strconv.FormatFloat(r.total, 'g', -1, 64), titles[r.taskID])
		for _, n := range r.notes {
			fmt.Printf("                  · %s\n", n)
		}
	}
}

func cmdPending(args []string) {
	fs := flag.NewFlagSet("worklog pending", flag.ExitOnError)
	dbPath := fs.String("db", store.DefaultPath(), "path to worklog.db")
	proj := fs.String("project", "", "filter by project slug (case-insensitive)")
	all := fs.Bool("all", false, "include hidden tasks")
	_ = fs.Parse(args)

	db := openDBOrFail(*dbPath)
	defer db.Close()

	tasks, err := db.ListTasks(*all)
	if err != nil {
		failCLI(err)
	}

	pending := map[string]bool{
		"backlog":     true,
		"todo":        true,
		"in_progress": true,
		"to_test":     true,
		"to_deploy":   true,
	}
	today := startOfDay(time.Now())

	type item struct {
		t       store.Task
		ageDays int
	}
	var items []item
	for _, t := range tasks {
		if t.Archived || !pending[t.Status] {
			continue
		}
		if *proj != "" && !strings.EqualFold(t.Project, *proj) {
			continue
		}
		age := 0
		if !t.StatusChangedAt.IsZero() {
			age = int(today.Sub(t.StatusChangedAt).Hours() / 24)
			if age < 0 {
				age = 0
			}
		}
		items = append(items, item{t, age})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ageDays != items[j].ageDays {
			return items[i].ageDays > items[j].ageDays
		}
		return items[i].t.ExternalID < items[j].t.ExternalID
	})

	if len(items) == 0 {
		fmt.Println("(nothing pending)")
		return
	}
	for _, it := range items {
		age := ""
		if it.ageDays > 0 {
			age = fmt.Sprintf("%dd", it.ageDays)
		}
		fmt.Printf("  %-10s %-10s %-12s %4s  %s\n",
			it.t.ExternalID, it.t.Project, it.t.Status, age, it.t.Short)
	}
}

func cmdRename(args []string) {
	fs := flag.NewFlagSet("worklog rename", flag.ExitOnError)
	dbPath := fs.String("db", store.DefaultPath(), "path to worklog.db")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: worklog rename <old-id> <new-id>")
		fs.PrintDefaults()
	}
	args = reorderFlagsFirst(args, map[string]bool{"db": true})
	_ = fs.Parse(args)
	rest := fs.Args()
	if len(rest) != 2 {
		fs.Usage()
		os.Exit(2)
	}
	oldID, newID := rest[0], rest[1]
	db := openDBOrFail(*dbPath)
	defer db.Close()

	// Fetch the task (include hidden — rename should work on archived too)
	tasks, err := db.ListTasks(true)
	if err != nil {
		failCLI(err)
	}
	var found *store.Task
	for i := range tasks {
		if tasks[i].ExternalID == oldID {
			found = &tasks[i]
			break
		}
	}
	if found == nil {
		failCLI(fmt.Errorf("task %q not found", oldID))
	}
	updated := *found
	updated.ExternalID = newID
	if err := db.UpdateTask(oldID, updated); err != nil {
		failCLI(err)
	}
	fmt.Printf("renamed %s → %s\n", oldID, newID)
}

func cmdStatus(args []string) {
	fs := flag.NewFlagSet("worklog status", flag.ExitOnError)
	dbPath := fs.String("db", store.DefaultPath(), "path to worklog.db")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: worklog status <task-id> <new-status>")
		fmt.Fprintln(fs.Output(), "  statuses: backlog, todo, in_progress, to_test, to_deploy, on_hold, done")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)
	rest := fs.Args()
	if len(rest) != 2 {
		fs.Usage()
		os.Exit(2)
	}
	db := openDBOrFail(*dbPath)
	defer db.Close()
	if err := db.SetTaskStatus(rest[0], rest[1]); err != nil {
		failCLI(err)
	}
	fmt.Printf("%s · status → %s\n", rest[0], rest[1])
}

func cmdExport(args []string) {
	fs := flag.NewFlagSet("worklog export", flag.ExitOnError)
	dbPath := fs.String("db", store.DefaultPath(), "path to worklog.db")
	fromS := fs.String("from", "", "YYYY-MM-DD inclusive (default: today)")
	toS := fs.String("to", "", "YYYY-MM-DD inclusive (default: same as --from)")
	rng := fs.String("range", "", "today | week | month — overrides --from/--to")
	proj := fs.String("project", "", "filter to one project slug (case-insensitive)")
	notes := fs.Bool("notes", false, "include log notes under each task")
	_ = fs.Parse(args)

	from, to, err := resolveRange(*rng, *fromS, *toS)
	if err != nil {
		failCLI(err)
	}

	db := openDBOrFail(*dbPath)
	defer db.Close()

	logs, err := db.ListLogs(false)
	if err != nil {
		failCLI(err)
	}
	// includeHidden=true so titles still resolve for logs that belong to a
	// task the user has hidden — the log itself may not be hidden.
	tasks, err := db.ListTasks(true)
	if err != nil {
		failCLI(err)
	}

	if *proj != "" {
		taskProj := make(map[string]string, len(tasks))
		for _, t := range tasks {
			taskProj[t.ExternalID] = t.Project
		}
		filtered := logs[:0]
		for _, l := range logs {
			if strings.EqualFold(taskProj[l.TaskID], *proj) {
				filtered = append(filtered, l)
			}
		}
		logs = filtered
	}

	days := report.Group(logs, tasks, from, to)
	fmt.Print(report.Render(days, report.Options{
		From: from, To: to, WithNotes: *notes,
	}))
}

// cmdHide is wired to both `worklog hide` and `worklog unhide`. The boolean
// hidden controls direction so the two paths share validation, error
// messages, and lookup logic.
//
//	worklog hide   task <id>
//	worklog hide   project <slug>
//	worklog hide   log <id>
//	worklog unhide task <id>      ...etc
func cmdHide(args []string, hidden bool) {
	verb := "hide"
	if !hidden {
		verb = "unhide"
	}
	fs := flag.NewFlagSet("worklog "+verb, flag.ExitOnError)
	dbPath := fs.String("db", store.DefaultPath(), "path to worklog.db")
	noCascade := fs.Bool("no-cascade", false, "for task/project: don't propagate the hidden flag to children")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: worklog %s <task|project|log> <id>\n", verb)
		fs.PrintDefaults()
	}
	args = reorderFlagsFirst(args, map[string]bool{"db": true})
	_ = fs.Parse(args)
	rest := fs.Args()
	if len(rest) != 2 {
		fs.Usage()
		os.Exit(2)
	}
	kind, id := rest[0], rest[1]
	db := openDBOrFail(*dbPath)
	defer db.Close()
	cascade := !*noCascade

	var err error
	switch kind {
	case "task":
		err = db.SetTaskArchived(id, hidden, cascade)
	case "project":
		err = db.SetProjectArchived(id, hidden, cascade)
	case "log":
		var n int64
		if _, perr := fmt.Sscanf(id, "%d", &n); perr != nil {
			failCLI(fmt.Errorf("log id %q: expected integer", id))
		}
		err = db.SetLogArchived(n, hidden)
	default:
		fmt.Fprintf(os.Stderr, "worklog %s: unknown kind %q (expected task|project|log)\n", verb, kind)
		os.Exit(2)
	}
	if err != nil {
		failCLI(err)
	}
	fmt.Printf("%s %s %s\n", verb, kind, id)
}

// resolveRange turns the user's --range / --from / --to flags into a concrete
// [from..to] window. --range wins over the explicit dates so the friendly
// shortcut isn't accidentally widened by leftover flags.
func resolveRange(rng, fromS, toS string) (time.Time, time.Time, error) {
	today := startOfDay(time.Now())
	if rng != "" {
		from, to, ok := report.RangeFor(today, rng)
		if !ok {
			return time.Time{}, time.Time{}, fmt.Errorf("range %q: expected today, week, or month", rng)
		}
		return from, to, nil
	}
	from, err := parseDateOpt(fromS)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if from.IsZero() {
		from = today
	}
	to, err := parseDateOpt(toS)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if to.IsZero() {
		to = from
	}
	return from, to, nil
}

// cmdActivity dispatches `worklog activity <add|list>`. Two subcommands so a
// single binary handles both create and read paths without juggling flags.
func cmdActivity(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: worklog activity <add|list> ...")
		os.Exit(2)
	}
	switch args[0] {
	case "add":
		cmdActivityAdd(args[1:])
	case "list":
		cmdActivityList(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "worklog activity: unknown subcommand %q\n", args[0])
		fmt.Fprintln(os.Stderr, "subcommands: add, list")
		os.Exit(2)
	}
}

func cmdActivityAdd(args []string) {
	fs := flag.NewFlagSet("worklog activity add", flag.ExitOnError)
	dbPath := fs.String("db", store.DefaultPath(), "path to worklog.db")
	url := fs.String("url", "", "URL (MR/commit/deploy/link target)")
	textFlag := fs.String("text", "", "freeform text (required for type=note)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: worklog activity add <task-id> <type> [--url U] [--text T]")
		fmt.Fprintln(fs.Output(), "  type: mr, commit, deploy, link, note")
		fs.PrintDefaults()
	}
	args = reorderFlagsFirst(args, map[string]bool{
		"db": true, "url": true, "text": true,
	})
	_ = fs.Parse(args)
	rest := fs.Args()
	if len(rest) < 2 {
		fs.Usage()
		os.Exit(2)
	}
	taskID, typ := rest[0], rest[1]
	a := store.Activity{TaskID: taskID, Type: typ, URL: *url, Text: *textFlag}
	if err := domain.ValidateActivity(a); err != nil {
		failCLI(err)
	}
	db := openDBOrFail(*dbPath)
	defer db.Close()
	a, err := db.CreateActivity(a)
	if err != nil {
		failCLI(err)
	}
	fmt.Printf("activity #%d · %s %s · %s", a.ID, a.Type, a.TaskID, a.URL)
	if a.Text != "" {
		fmt.Printf("  %s", a.Text)
	}
	fmt.Println()
}

func cmdActivityList(args []string) {
	fs := flag.NewFlagSet("worklog activity list", flag.ExitOnError)
	dbPath := fs.String("db", store.DefaultPath(), "path to worklog.db")
	taskID := fs.String("task", "", "scope to a single task id")
	all := fs.Bool("all", false, "include hidden activities")
	_ = fs.Parse(args)
	db := openDBOrFail(*dbPath)
	defer db.Close()
	as, err := db.ListActivities(*taskID, *all)
	if err != nil {
		failCLI(err)
	}
	if len(as) == 0 {
		fmt.Println("(no activities)")
		return
	}
	for _, a := range as {
		fmt.Printf("  #%-4d %-7s %-10s %s", a.ID, a.Type, a.TaskID, a.URL)
		if a.Text != "" {
			fmt.Printf("  %s", a.Text)
		}
		fmt.Println()
	}
}

// reorderFlagsFirst moves any flag tokens (and the value tokens that follow
// non-bool flags) to the front of the slice, leaving positional args at the
// tail. The standard `flag` package stops at the first positional, so this
// lets users write git-style `cmd <pos1> <pos2> --flag v` without surprise.
// valueFlags lists names (without leading dashes) of flags that consume a
// following token; bool flags should be omitted.
func reorderFlagsFirst(args []string, valueFlags map[string]bool) []string {
	var flags, positional []string
	i := 0
	for i < len(args) {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			positional = append(positional, a)
			i++
			continue
		}
		flags = append(flags, a)
		i++
		// "--name=value" form already carries its value
		if strings.Contains(a, "=") {
			continue
		}
		name := strings.TrimLeft(a, "-")
		if valueFlags[name] && i < len(args) {
			flags = append(flags, args[i])
			i++
		}
	}
	return append(flags, positional...)
}

// ── helpers ─────────────────────────────────────────────────────────────────

func parseDateOpt(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("date %q: expected YYYY-MM-DD", s)
	}
	return t, nil
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func sameLocalDay(a, b time.Time) bool {
	a = a.In(time.Local)
	b = b.In(time.Local)
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func openDBOrFail(path string) *store.DB {
	db, err := store.Open(path)
	if err != nil {
		failCLI(err)
	}
	return db
}

func failCLI(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
