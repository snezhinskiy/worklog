package mcpsrv

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/snezhinskiy/worklog/internal/domain"
	"github.com/snezhinskiy/worklog/internal/store"
)

type logCreateIn struct {
	TaskID string  `json:"task_id" jsonschema:"task external id, e.g. AU-3569"`
	Hours  float64 `json:"hours" jsonschema:"hours spent (must be > 0)"`
	Date   string  `json:"date,omitempty" jsonschema:"YYYY-MM-DD; defaults to today"`
	Time   string  `json:"time,omitempty" jsonschema:"HH:MM; defaults to now"`
	Note   string  `json:"note,omitempty" jsonschema:"short freeform note about what was done"`
}

type logCreateOut struct {
	Log LogDTO `json:"log"`
}

type logListIn struct {
	From          string `json:"from,omitempty" jsonschema:"YYYY-MM-DD inclusive; defaults to no lower bound"`
	To            string `json:"to,omitempty" jsonschema:"YYYY-MM-DD inclusive; defaults to no upper bound"`
	TaskID        string `json:"task_id,omitempty" jsonschema:"if set, only return logs for this task"`
	IncludeHidden bool   `json:"include_hidden,omitempty" jsonschema:"include hidden log entries (default: false)"`
}

type logListOut struct {
	Logs []LogDTO `json:"logs"`
}

type logSetHiddenIn struct {
	ID     int64 `json:"id" jsonschema:"log entry id (returned by log_create)"`
	Hidden bool  `json:"hidden" jsonschema:"true to hide, false to unhide"`
}

type logSetHiddenOut struct {
	ID      int64 `json:"id"`
	Hidden  bool  `json:"hidden"`
	Changed bool  `json:"changed"`
}

type logUpdateIn struct {
	ID     int64   `json:"id" jsonschema:"log entry id"`
	TaskID string  `json:"task_id,omitempty" jsonschema:"move the log to a different task (leave empty to keep current)"`
	Date   string  `json:"date,omitempty" jsonschema:"YYYY-MM-DD (leave empty to keep current)"`
	Time   string  `json:"time,omitempty" jsonschema:"HH:MM (leave empty to keep current)"`
	Hours  float64 `json:"hours,omitempty" jsonschema:"hours spent (leave 0 to keep current)"`
	Note   string  `json:"note,omitempty" jsonschema:"new note text (leave empty to keep current)"`
}

type logUpdateOut struct {
	Log LogDTO `json:"log"`
}

type logDeleteIn struct {
	ID int64 `json:"id" jsonschema:"log entry id to permanently remove"`
}

type logDeleteOut struct {
	ID      int64 `json:"id"`
	Deleted bool  `json:"deleted"`
}

func addLogTools(s *mcp.Server, db domain.Store) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "log_create",
		Description: "Log time spent on a task. Date defaults to today, time to now. Hours can be fractional (e.g. 1.5).",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in logCreateIn) (*mcp.CallToolResult, logCreateOut, error) {
		entry := store.LogEntry{
			TaskID: in.TaskID,
			Hours:  in.Hours,
			Time:   in.Time,
			Note:   in.Note,
		}
		if in.Date != "" {
			d, err := time.ParseInLocation(dateLayout, in.Date, time.Local)
			if err != nil {
				return nil, logCreateOut{}, fmt.Errorf("date %q: %w (expected YYYY-MM-DD)", in.Date, err)
			}
			entry.Date = d
		}
		saved, err := db.CreateLog(entry)
		if err != nil {
			return nil, logCreateOut{}, err
		}
		return nil, logCreateOut{Log: logToDTO(saved)}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "log_list",
		Description: "List log entries, optionally filtered by date range and task. Hidden entries are excluded unless include_hidden is true.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in logListIn) (*mcp.CallToolResult, logListOut, error) {
		from, to, err := parseRange(in.From, in.To)
		if err != nil {
			return nil, logListOut{}, err
		}
		logs, err := db.ListLogs(in.IncludeHidden)
		if err != nil {
			return nil, logListOut{}, err
		}
		out := logListOut{Logs: make([]LogDTO, 0, len(logs))}
		for _, l := range logs {
			if in.TaskID != "" && l.TaskID != in.TaskID {
				continue
			}
			if !from.IsZero() && l.Date.Before(from) {
				continue
			}
			if !to.IsZero() && l.Date.After(to) {
				continue
			}
			out.Logs = append(out.Logs, logToDTO(l))
		}
		return nil, out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "log_update",
		Description: "Update fields of an existing log entry by id. Empty fields keep the current value. To move a log to a different task, set task_id. Use log_set_hidden to hide instead of editing.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in logUpdateIn) (*mcp.CallToolResult, logUpdateOut, error) {
		cur, ok, err := findLog(db, in.ID)
		if err != nil {
			return nil, logUpdateOut{}, err
		}
		if !ok {
			return nil, logUpdateOut{}, fmt.Errorf("log %d not found", in.ID)
		}
		updated := cur
		if in.TaskID != "" {
			updated.TaskID = in.TaskID
		}
		if in.Date != "" {
			d, err := time.ParseInLocation(dateLayout, in.Date, time.Local)
			if err != nil {
				return nil, logUpdateOut{}, fmt.Errorf("date %q: %w (expected YYYY-MM-DD)", in.Date, err)
			}
			updated.Date = d
		}
		if in.Time != "" {
			updated.Time = in.Time
		}
		if in.Hours > 0 {
			updated.Hours = in.Hours
		}
		if in.Note != "" {
			updated.Note = in.Note
		}
		if err := db.UpdateLog(updated); err != nil {
			return nil, logUpdateOut{}, err
		}
		fresh, _, _ := findLog(db, in.ID)
		return nil, logUpdateOut{Log: logToDTO(fresh)}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "log_delete",
		Description: "Permanently remove a log entry by id. This is the hard-delete escape hatch — prefer log_set_hidden for routine cleanup since hidden entries can be brought back.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in logDeleteIn) (*mcp.CallToolResult, logDeleteOut, error) {
		if err := db.DeleteLog(in.ID); err != nil {
			return nil, logDeleteOut{}, err
		}
		return nil, logDeleteOut{ID: in.ID, Deleted: true}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "log_set_hidden",
		Description: "Hide or unhide a log entry by its id. Hidden entries don't count toward day reports or appear in default lists.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in logSetHiddenIn) (*mcp.CallToolResult, logSetHiddenOut, error) {
		if err := db.SetLogArchived(in.ID, in.Hidden); err != nil {
			return nil, logSetHiddenOut{}, err
		}
		return nil, logSetHiddenOut{ID: in.ID, Hidden: in.Hidden, Changed: true}, nil
	})
}

// findLog fetches a log by id, scanning the full list (volumes are tiny).
// Returns includeHidden=true so updates and reads-after-write work even on
// archived rows.
func findLog(db domain.Store, id int64) (store.LogEntry, bool, error) {
	all, err := db.ListLogs(true)
	if err != nil {
		return store.LogEntry{}, false, err
	}
	for _, l := range all {
		if l.ID == id {
			return l, true, nil
		}
	}
	return store.LogEntry{}, false, nil
}

// parseRange validates and parses optional from/to date strings. Empty strings
// pass through as zero time (meaning "no bound").
func parseRange(fromS, toS string) (time.Time, time.Time, error) {
	var from, to time.Time
	if fromS != "" {
		t, err := time.ParseInLocation(dateLayout, fromS, time.Local)
		if err != nil {
			return from, to, fmt.Errorf("from %q: %w (expected YYYY-MM-DD)", fromS, err)
		}
		from = t
	}
	if toS != "" {
		t, err := time.ParseInLocation(dateLayout, toS, time.Local)
		if err != nil {
			return from, to, fmt.Errorf("to %q: %w (expected YYYY-MM-DD)", toS, err)
		}
		to = t
	}
	return from, to, nil
}
