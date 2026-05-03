package mcpsrv

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/snezhinskiy/worklog/internal/domain"
	"github.com/snezhinskiy/worklog/internal/store"
)

type taskListIn struct {
	Project       string `json:"project,omitempty" jsonschema:"if set, only return tasks of this project slug"`
	Status        string `json:"status,omitempty" jsonschema:"if set, only return tasks with this status (e.g. in_progress)"`
	IncludeHidden bool   `json:"include_hidden,omitempty" jsonschema:"include hidden tasks (default: false)"`
}

type taskSetHiddenIn struct {
	ExternalID string `json:"external_id" jsonschema:"task id, e.g. AU-3569"`
	Hidden     bool   `json:"hidden" jsonschema:"true to hide, false to unhide"`
	NoCascade  bool   `json:"no_cascade,omitempty" jsonschema:"by default the same hidden flag is propagated to the task's logs and activities; set true to touch only the task row"`
}

type taskSetHiddenOut struct {
	Task TaskDTO `json:"task"`
}

type taskRenameIn struct {
	ExternalID    string `json:"external_id" jsonschema:"current task id"`
	NewExternalID string `json:"new_external_id" jsonschema:"new task id (e.g. AU-3569 from Jira); logs and activities follow via FK CASCADE"`
}

type taskRenameOut struct {
	Task TaskDTO `json:"task"`
}

type taskUpdateIn struct {
	ExternalID string `json:"external_id" jsonschema:"task id to update"`
	Short      string `json:"short,omitempty" jsonschema:"new title (leave empty to keep current)"`
	Status     string `json:"status,omitempty" jsonschema:"new status (leave empty to keep current). canonical values: backlog, todo, in_progress, to_test, to_deploy, on_hold, done"`
}

type taskUpdateOut struct {
	Task TaskDTO `json:"task"`
}

type taskDeleteIn struct {
	ExternalID string `json:"external_id" jsonschema:"task id to permanently remove. Logs and activities cascade away too via FK ON DELETE CASCADE."`
}

type taskDeleteOut struct {
	ExternalID string `json:"external_id"`
	Deleted    bool   `json:"deleted"`
}

type taskListOut struct {
	Tasks []TaskDTO `json:"tasks"`
}

type taskCreateIn struct {
	Project    string `json:"project" jsonschema:"project slug; the task will be assigned an id like SLUG-N if external_id is empty"`
	Short      string `json:"short" jsonschema:"task title"`
	Status     string `json:"status,omitempty" jsonschema:"initial status; defaults to todo. canonical values: backlog, todo, in_progress, to_test, to_deploy, on_hold, done"`
	ExternalID string `json:"external_id,omitempty" jsonschema:"override the auto-generated id (e.g. AU-3569 for an existing Jira ticket)"`
}

type taskCreateOut struct {
	Task TaskDTO `json:"task"`
}

type taskSetStatusIn struct {
	ExternalID string `json:"external_id" jsonschema:"task id, e.g. AU-3569"`
	Status     string `json:"status" jsonschema:"new status. canonical values: backlog, todo, in_progress, to_test, to_deploy, on_hold, done"`
}

type taskSetStatusOut struct {
	Task TaskDTO `json:"task"`
}

func addTaskTools(s *mcp.Server, db domain.Store) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "task_list",
		Description: "List tasks. Optional filters: project slug, status. Hidden tasks are excluded unless include_hidden is true.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in taskListIn) (*mcp.CallToolResult, taskListOut, error) {
		ts, err := db.ListTasks(in.IncludeHidden)
		if err != nil {
			return nil, taskListOut{}, err
		}
		out := taskListOut{Tasks: make([]TaskDTO, 0, len(ts))}
		for _, t := range ts {
			if in.Project != "" && !strings.EqualFold(t.Project, in.Project) {
				continue
			}
			if in.Status != "" && t.Status != in.Status {
				continue
			}
			out.Tasks = append(out.Tasks, taskToDTO(t))
		}
		return nil, out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "task_create",
		Description: "Create a task in a project. If external_id is empty, an id like SLUG-N is generated.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in taskCreateIn) (*mcp.CallToolResult, taskCreateOut, error) {
		t, err := db.CreateTask(store.Task{
			Project:    in.Project,
			ExternalID: in.ExternalID,
			Status:     in.Status,
			Short:      in.Short,
		})
		if err != nil {
			return nil, taskCreateOut{}, err
		}
		return nil, taskCreateOut{Task: taskToDTO(t)}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "task_set_status",
		Description: "Move a task to a new status. Bumps the status_changed_at timestamp used by the board to highlight stale cards.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in taskSetStatusIn) (*mcp.CallToolResult, taskSetStatusOut, error) {
		if err := db.SetTaskStatus(in.ExternalID, in.Status); err != nil {
			return nil, taskSetStatusOut{}, err
		}
		t, ok, err := findTask(db, in.ExternalID)
		if err != nil {
			return nil, taskSetStatusOut{}, err
		}
		if !ok {
			return nil, taskSetStatusOut{}, fmt.Errorf("task %s not found after update", in.ExternalID)
		}
		return nil, taskSetStatusOut{Task: taskToDTO(t)}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "task_update",
		Description: "Update a task's title (short) and/or status. Empty fields keep the current value. For changing the id use task_rename; for hiding use task_set_hidden.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in taskUpdateIn) (*mcp.CallToolResult, taskUpdateOut, error) {
		cur, ok, err := findTask(db, in.ExternalID)
		if err != nil {
			return nil, taskUpdateOut{}, err
		}
		if !ok {
			return nil, taskUpdateOut{}, fmt.Errorf("task %s not found", in.ExternalID)
		}
		updated := cur
		if in.Short != "" {
			updated.Short = in.Short
		}
		if in.Status != "" && in.Status != cur.Status {
			updated.Status = in.Status
			updated.StatusChangedAt = time.Now()
		}
		if err := db.UpdateTask(in.ExternalID, updated); err != nil {
			return nil, taskUpdateOut{}, err
		}
		fresh, _, _ := findTask(db, in.ExternalID)
		return nil, taskUpdateOut{Task: taskToDTO(fresh)}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "task_rename",
		Description: "Rename a task — change its external_id. Useful when adopting a Jira-style ticket id after the task was created with an auto-generated one. Logs and activities follow via FK CASCADE.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in taskRenameIn) (*mcp.CallToolResult, taskRenameOut, error) {
		cur, ok, err := findTask(db, in.ExternalID)
		if err != nil {
			return nil, taskRenameOut{}, err
		}
		if !ok {
			return nil, taskRenameOut{}, fmt.Errorf("task %s not found", in.ExternalID)
		}
		cur.ExternalID = in.NewExternalID
		if err := db.UpdateTask(in.ExternalID, cur); err != nil {
			return nil, taskRenameOut{}, err
		}
		fresh, ok, err := findTask(db, in.NewExternalID)
		if err != nil {
			return nil, taskRenameOut{}, err
		}
		if !ok {
			return nil, taskRenameOut{}, fmt.Errorf("task %s not found after rename", in.NewExternalID)
		}
		return nil, taskRenameOut{Task: taskToDTO(fresh)}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "task_delete",
		Description: "Permanently remove a task and all its logs and activities (FK ON DELETE CASCADE). Hard-delete escape hatch — prefer task_set_hidden for routine cleanup.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in taskDeleteIn) (*mcp.CallToolResult, taskDeleteOut, error) {
		if err := db.DeleteTask(in.ExternalID); err != nil {
			return nil, taskDeleteOut{}, err
		}
		return nil, taskDeleteOut{ExternalID: in.ExternalID, Deleted: true}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "task_set_hidden",
		Description: "Hide or unhide a task. By default the same flag also applies to the task's logs and activities so they don't dangle visible — pass no_cascade=true to touch only the task row. Hidden rows stay in the DB but are excluded from default lists, search, and reports.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in taskSetHiddenIn) (*mcp.CallToolResult, taskSetHiddenOut, error) {
		if err := db.SetTaskArchived(in.ExternalID, in.Hidden, !in.NoCascade); err != nil {
			return nil, taskSetHiddenOut{}, err
		}
		t, ok, err := findTask(db, in.ExternalID)
		if err != nil {
			return nil, taskSetHiddenOut{}, err
		}
		if !ok {
			return nil, taskSetHiddenOut{}, fmt.Errorf("task %s not found after update", in.ExternalID)
		}
		return nil, taskSetHiddenOut{Task: taskToDTO(t)}, nil
	})
}

// findTask is a small helper used by tools that need to return the post-write
// state of a single task. The store doesn't expose a single-row getter yet so
// we filter the list in memory; volumes are tiny.
func findTask(db domain.Store, extID string) (store.Task, bool, error) {
	// Pass includeHidden=true so callers (like task_set_hidden) can read
	// back the task they just archived.
	ts, err := db.ListTasks(true)
	if err != nil {
		return store.Task{}, false, err
	}
	for _, t := range ts {
		if t.ExternalID == extID {
			return t, true, nil
		}
	}
	return store.Task{}, false, nil
}
