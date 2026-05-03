package mcpsrv

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/snezhinskiy/worklog/internal/domain"
	"github.com/snezhinskiy/worklog/internal/store"
)

// ActivityDTO is the wire shape exposed via MCP. CreatedAt is RFC3339 so an
// LLM can reason about ordering.
type ActivityDTO struct {
	ID        int64  `json:"id"`
	TaskID    string `json:"task_id"`
	Type      string `json:"type"`
	URL       string `json:"url,omitempty"`
	Text      string `json:"text,omitempty"`
	Archived  bool   `json:"archived"`
	CreatedAt string `json:"created_at,omitempty"`
}

func activityToDTO(a store.Activity) ActivityDTO {
	d := ActivityDTO{
		ID:       a.ID,
		TaskID:   a.TaskID,
		Type:     a.Type,
		URL:      a.URL,
		Text:     a.Text,
		Archived: a.Archived,
	}
	if !a.CreatedAt.IsZero() {
		d.CreatedAt = a.CreatedAt.UTC().Format(time.RFC3339)
	}
	return d
}

type activityCreateIn struct {
	TaskID string `json:"task_id" jsonschema:"task external id (e.g. AU-3569)"`
	Type   string `json:"type" jsonschema:"one of: mr, commit, deploy, link, note"`
	URL    string `json:"url,omitempty" jsonschema:"link to MR/commit/deploy/etc.; required for url-bearing types in practice"`
	Text   string `json:"text,omitempty" jsonschema:"freeform note (required for type=note; may accompany URL on others)"`
}

type activityCreateOut struct {
	Activity ActivityDTO `json:"activity"`
}

type activityListIn struct {
	TaskID        string `json:"task_id,omitempty" jsonschema:"if set, only activities for this task; otherwise all"`
	IncludeHidden bool   `json:"include_hidden,omitempty" jsonschema:"include hidden activities (default: false)"`
}

type activityListOut struct {
	Activities []ActivityDTO `json:"activities"`
}

type activitySetHiddenIn struct {
	ID     int64 `json:"id" jsonschema:"activity id (returned by activity_create)"`
	Hidden bool  `json:"hidden" jsonschema:"true to hide, false to unhide"`
}

type activitySetHiddenOut struct {
	ID      int64 `json:"id"`
	Hidden  bool  `json:"hidden"`
	Changed bool  `json:"changed"`
}

type activityUpdateIn struct {
	ID   int64  `json:"id" jsonschema:"activity id"`
	Type string `json:"type,omitempty" jsonschema:"new type (mr/commit/deploy/link/note); leave empty to keep current"`
	URL  string `json:"url,omitempty" jsonschema:"new URL; leave empty to keep current. Pass a single space to clear"`
	Text string `json:"text,omitempty" jsonschema:"new freeform text; leave empty to keep current. Pass a single space to clear"`
}

type activityUpdateOut struct {
	Activity ActivityDTO `json:"activity"`
}

type activityDeleteIn struct {
	ID int64 `json:"id" jsonschema:"activity id to permanently remove"`
}

type activityDeleteOut struct {
	ID      int64 `json:"id"`
	Deleted bool  `json:"deleted"`
}

func addActivityTools(s *mcp.Server, db domain.Store) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "activity_create",
		Description: "Record a typed activity on a task (merge request, commit, deploy, link, note). At least one of url or text must be set.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in activityCreateIn) (*mcp.CallToolResult, activityCreateOut, error) {
		a := store.Activity{TaskID: in.TaskID, Type: in.Type, URL: in.URL, Text: in.Text}
		if err := domain.ValidateActivity(a); err != nil {
			return nil, activityCreateOut{}, err
		}
		a, err := db.CreateActivity(a)
		if err != nil {
			return nil, activityCreateOut{}, err
		}
		return nil, activityCreateOut{Activity: activityToDTO(a)}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "activity_list",
		Description: "List activities, optionally scoped to a task. Hidden activities are excluded unless include_hidden is true.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in activityListIn) (*mcp.CallToolResult, activityListOut, error) {
		as, err := db.ListActivities(in.TaskID, in.IncludeHidden)
		if err != nil {
			return nil, activityListOut{}, err
		}
		out := activityListOut{Activities: make([]ActivityDTO, 0, len(as))}
		for _, a := range as {
			out.Activities = append(out.Activities, activityToDTO(a))
		}
		return nil, out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "activity_update",
		Description: "Update fields of an existing activity by id. Empty fields keep the current value; pass a single space to deliberately clear url or text. Use activity_set_hidden to hide instead of editing.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in activityUpdateIn) (*mcp.CallToolResult, activityUpdateOut, error) {
		cur, ok, err := findActivity(db, in.ID)
		if err != nil {
			return nil, activityUpdateOut{}, err
		}
		if !ok {
			return nil, activityUpdateOut{}, fmt.Errorf("activity %d not found", in.ID)
		}
		updated := cur
		if in.Type != "" {
			if !domain.IsActivityType(in.Type) {
				return nil, activityUpdateOut{}, fmt.Errorf("type %q: expected one of mr, commit, deploy, link, note", in.Type)
			}
			updated.Type = in.Type
		}
		// Treat a single space as "clear this field" since omitempty in
		// JSON makes "" mean "unset" — we can't distinguish empty from
		// absent otherwise.
		if in.URL != "" {
			if in.URL == " " {
				updated.URL = ""
			} else {
				updated.URL = in.URL
			}
		}
		if in.Text != "" {
			if in.Text == " " {
				updated.Text = ""
			} else {
				updated.Text = in.Text
			}
		}
		if err := domain.ValidateActivity(updated); err != nil {
			return nil, activityUpdateOut{}, err
		}
		if err := db.UpdateActivity(updated); err != nil {
			return nil, activityUpdateOut{}, err
		}
		fresh, _, _ := findActivity(db, in.ID)
		return nil, activityUpdateOut{Activity: activityToDTO(fresh)}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "activity_delete",
		Description: "Permanently remove an activity by id. Hard-delete escape hatch — prefer activity_set_hidden for routine cleanup.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in activityDeleteIn) (*mcp.CallToolResult, activityDeleteOut, error) {
		if err := db.DeleteActivity(in.ID); err != nil {
			return nil, activityDeleteOut{}, err
		}
		return nil, activityDeleteOut{ID: in.ID, Deleted: true}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "activity_set_hidden",
		Description: "Hide or unhide an activity by id. Hidden activities don't appear in default lists or in the task editor.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in activitySetHiddenIn) (*mcp.CallToolResult, activitySetHiddenOut, error) {
		if err := db.SetActivityArchived(in.ID, in.Hidden); err != nil {
			return nil, activitySetHiddenOut{}, err
		}
		return nil, activitySetHiddenOut{ID: in.ID, Hidden: in.Hidden, Changed: true}, nil
	})
}

// findActivity fetches an activity by id (includes hidden so updates work
// on archived rows too).
func findActivity(db domain.Store, id int64) (store.Activity, bool, error) {
	all, err := db.ListActivities("", true)
	if err != nil {
		return store.Activity{}, false, err
	}
	for _, a := range all {
		if a.ID == id {
			return a, true, nil
		}
	}
	return store.Activity{}, false, nil
}

