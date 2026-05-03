package mcpsrv

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/snezhinskiy/worklog/internal/domain"
)

// statuses considered "still on the user's plate" for pending_tasks. Anything
// not in this set (done, on_hold, archived) is filtered out.
var pendingStatuses = map[string]bool{
	"backlog":     true,
	"todo":        true,
	"in_progress": true,
	"to_test":     true,
	"to_deploy":   true,
}

type reportDayIn struct {
	Date string `json:"date,omitempty" jsonschema:"YYYY-MM-DD; defaults to today"`
}

type reportDayTask struct {
	TaskID string   `json:"task_id"`
	Short  string   `json:"short,omitempty"`
	Hours  float64  `json:"hours"`
	Notes  []string `json:"notes,omitempty"`
}

type reportDayOut struct {
	Date       string          `json:"date"`
	TotalHours float64         `json:"total_hours"`
	ByTask     []reportDayTask `json:"by_task"`
}

type pendingTasksIn struct {
	Project string `json:"project,omitempty" jsonschema:"if set, restrict to this project slug"`
}

type pendingTasksOut struct {
	Tasks []TaskDTO `json:"tasks"`
}

func addReportTools(s *mcp.Server, db domain.Store) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "report_day",
		Description: "Summarize work logged on a given day (defaults to today). Returns total hours and a per-task breakdown with notes — useful for stand-ups.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in reportDayIn) (*mcp.CallToolResult, reportDayOut, error) {
		day, err := dayOrToday(in.Date)
		if err != nil {
			return nil, reportDayOut{}, err
		}
		// Reports always exclude hidden entries — hidden time isn't time the
		// user wants to acknowledge in stand-ups. Same for hidden tasks
		// (their titles still resolve via lookup though).
		logs, err := db.ListLogs(false)
		if err != nil {
			return nil, reportDayOut{}, err
		}
		tasks, err := db.ListTasks(true)
		if err != nil {
			return nil, reportDayOut{}, err
		}
		titles := make(map[string]string, len(tasks))
		for _, t := range tasks {
			titles[t.ExternalID] = t.Short
		}

		byTask := map[string]*reportDayTask{}
		var total float64
		for _, l := range logs {
			if !sameDay(l.Date, day) {
				continue
			}
			row, ok := byTask[l.TaskID]
			if !ok {
				row = &reportDayTask{TaskID: l.TaskID, Short: titles[l.TaskID]}
				byTask[l.TaskID] = row
			}
			row.Hours += l.Hours
			if l.Note != "" {
				row.Notes = append(row.Notes, l.Note)
			}
			total += l.Hours
		}

		out := reportDayOut{
			Date:       day.Format(dateLayout),
			TotalHours: total,
			ByTask:     make([]reportDayTask, 0, len(byTask)),
		}
		for _, r := range byTask {
			out.ByTask = append(out.ByTask, *r)
		}
		sort.Slice(out.ByTask, func(i, j int) bool {
			return out.ByTask[i].Hours > out.ByTask[j].Hours
		})
		return nil, out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "pending_tasks",
		Description: "List tasks still on the user's plate (anything not done, on_hold, or archived). Optionally filter by project. Useful for 'what's left to do?' queries.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in pendingTasksIn) (*mcp.CallToolResult, pendingTasksOut, error) {
		ts, err := db.ListTasks(false)
		if err != nil {
			return nil, pendingTasksOut{}, err
		}
		out := pendingTasksOut{Tasks: make([]TaskDTO, 0)}
		for _, t := range ts {
			if !pendingStatuses[t.Status] {
				continue
			}
			if in.Project != "" && t.Project != in.Project {
				continue
			}
			out.Tasks = append(out.Tasks, taskToDTO(t))
		}
		return nil, out, nil
	})
}

func dayOrToday(s string) (time.Time, error) {
	if s == "" {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), nil
	}
	d, err := time.ParseInLocation(dateLayout, s, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("date %q: %w (expected YYYY-MM-DD)", s, err)
	}
	return d, nil
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}
