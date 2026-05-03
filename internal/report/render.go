// Package report turns a slice of store.LogEntry plus store.Task lookups into
// a plain-text "what did I do" report. Used by both the CLI export subcommand
// (writes to stdout) and the TUI /export palette command (copies to clipboard).
package report

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/snezhinskiy/worklog/internal/store"
)

// Options controls the output. From/To are inclusive; either may be zero to
// mean "no bound". WithNotes adds an indented per-log line under each task
// summary so you can paste a stand-up update verbatim.
type Options struct {
	From      time.Time
	To        time.Time
	WithNotes bool
}

// TaskRow aggregates time spent on one task within a single day.
type TaskRow struct {
	TaskID string
	Title  string
	Total  float64
	Logs   []store.LogEntry
}

// DayRow is one day's worth of work, broken down by task.
type DayRow struct {
	Date  time.Time
	Total float64
	Tasks []TaskRow
}

// Group rolls logs up into per-day summaries, filtered to [from..to]. Tasks
// inside a day are sorted by descending hours then by id; logs inside a task
// are sorted by time. The tasks slice supplies titles for the report.
func Group(logs []store.LogEntry, tasks []store.Task, from, to time.Time) []DayRow {
	titles := make(map[string]string, len(tasks))
	for _, t := range tasks {
		titles[t.ExternalID] = t.Short
	}

	byDay := map[string]*DayRow{}
	for _, l := range logs {
		if !from.IsZero() && l.Date.Before(from) {
			continue
		}
		if !to.IsZero() && l.Date.After(to) {
			continue
		}
		key := l.Date.Format("2006-01-02")
		d, ok := byDay[key]
		if !ok {
			d = &DayRow{Date: l.Date}
			byDay[key] = d
		}
		var tr *TaskRow
		for i := range d.Tasks {
			if d.Tasks[i].TaskID == l.TaskID {
				tr = &d.Tasks[i]
				break
			}
		}
		if tr == nil {
			d.Tasks = append(d.Tasks, TaskRow{TaskID: l.TaskID, Title: titles[l.TaskID]})
			tr = &d.Tasks[len(d.Tasks)-1]
		}
		tr.Total += l.Hours
		tr.Logs = append(tr.Logs, l)
		d.Total += l.Hours
	}

	days := make([]DayRow, 0, len(byDay))
	for _, d := range byDay {
		sort.SliceStable(d.Tasks, func(i, j int) bool {
			if d.Tasks[i].Total != d.Tasks[j].Total {
				return d.Tasks[i].Total > d.Tasks[j].Total
			}
			return d.Tasks[i].TaskID < d.Tasks[j].TaskID
		})
		for i := range d.Tasks {
			sort.SliceStable(d.Tasks[i].Logs, func(a, b int) bool {
				return d.Tasks[i].Logs[a].Time < d.Tasks[i].Logs[b].Time
			})
		}
		days = append(days, *d)
	}
	sort.Slice(days, func(i, j int) bool {
		return days[i].Date.Before(days[j].Date)
	})
	return days
}

// Render formats grouped days as plain text suitable for pasting into a
// stand-up message or saving as a file. Multi-day reports get a banner with
// the total; single-day reports skip it (the day section already names the
// date).
func Render(days []DayRow, opts Options) string {
	var b strings.Builder

	multiDay := !opts.From.IsZero() && !opts.To.IsZero() && !sameDay(opts.From, opts.To)
	if multiDay {
		var total float64
		for _, d := range days {
			total += d.Total
		}
		fmt.Fprintf(&b, "Worklog · %s .. %s · %sh\n\n",
			opts.From.Format("2006-01-02"), opts.To.Format("2006-01-02"), fmtHours(total))
	}

	for di, d := range days {
		if di > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%s · %sh\n", d.Date.Format("Mon, 2 Jan 2006"), fmtHours(d.Total))
		for _, t := range d.Tasks {
			fmt.Fprintf(&b, "  %-10s %5sh  %s\n", t.TaskID, fmtHours(t.Total), t.Title)
			if opts.WithNotes {
				for _, l := range t.Logs {
					note := l.Note
					if note == "" {
						note = "(no note)"
					}
					tm := l.Time
					if tm == "" {
						tm = "  --"
					}
					fmt.Fprintf(&b, "    %5s  %5sh  %s\n", tm, fmtHours(l.Hours), note)
				}
			}
		}
	}

	if len(days) == 0 {
		b.WriteString("(no entries in range)\n")
	}
	return b.String()
}

func fmtHours(h float64) string {
	return strconv.FormatFloat(h, 'g', -1, 64)
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// RangeFor turns a friendly range keyword (today | week | month) into a
// concrete [from..to] window anchored at today. Semantics match the TUI's
// rangeBounds so that /export <kind> covers the same days the user sees on
// screen for that kind.
//
//   - today: just today
//   - week:  Monday-anchored week containing today (Mon..Sun, full week)
//   - month: rolling 30 days ending today
//
// Returns ok=false for unknown keywords.
func RangeFor(today time.Time, kind string) (from, to time.Time, ok bool) {
	day := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	switch kind {
	case "today", "day":
		return day, day, true
	case "week":
		off := int(day.Weekday()) - 1
		if off < 0 {
			off = 6
		}
		from := day.AddDate(0, 0, -off)
		return from, from.AddDate(0, 0, 6), true
	case "month":
		return day.AddDate(0, 0, -29), day, true
	}
	return time.Time{}, time.Time{}, false
}
