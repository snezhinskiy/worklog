package mcpsrv

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/snezhinskiy/worklog/internal/domain"
	"github.com/snezhinskiy/worklog/internal/store"
)

type projectListIn struct {
	IncludeHidden bool `json:"include_hidden,omitempty" jsonschema:"include hidden projects (default: false)"`
}

type projectListOut struct {
	Projects []ProjectDTO `json:"projects"`
}

type projectCreateIn struct {
	Slug       string `json:"slug" jsonschema:"short uppercase identifier used as project key, e.g. AURA"`
	Name       string `json:"name,omitempty" jsonschema:"human-readable project name"`
	TaskPrefix string `json:"task_prefix,omitempty" jsonschema:"override the auto-generated task id prefix; defaults to slug. Example: slug=WORKLOG, task_prefix=WL → tasks become WL-1, WL-2, …"`
}

type projectCreateOut struct {
	Project ProjectDTO `json:"project"`
}

type projectUpdateIn struct {
	Slug       string `json:"slug" jsonschema:"current slug of the project to update"`
	NewSlug    string `json:"new_slug,omitempty" jsonschema:"rename to this slug if non-empty"`
	Name       string `json:"name,omitempty" jsonschema:"new human-readable name (leave empty to keep current)"`
	TaskPrefix string `json:"task_prefix,omitempty" jsonschema:"new task id prefix (leave empty to keep current)"`
}

type projectUpdateOut struct {
	Project ProjectDTO `json:"project"`
}

type projectSetHiddenIn struct {
	Slug      string `json:"slug" jsonschema:"project slug"`
	Hidden    bool   `json:"hidden" jsonschema:"true to hide, false to unhide"`
	NoCascade bool   `json:"no_cascade,omitempty" jsonschema:"by default the same hidden flag is propagated to all the project's tasks (and their logs and activities); set true to touch only the project row"`
}

type projectSetHiddenOut struct {
	Slug    string `json:"slug"`
	Hidden  bool   `json:"hidden"`
	Changed bool   `json:"changed"`
}

type projectDeleteIn struct {
	Slug string `json:"slug" jsonschema:"project slug to permanently remove. Fails if any task still references it — hide or move tasks first."`
}

type projectDeleteOut struct {
	Slug    string `json:"slug"`
	Deleted bool   `json:"deleted"`
}

func addProjectTools(s *mcp.Server, db domain.Store) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "project_list",
		Description: "List projects. Hidden projects are excluded unless include_hidden is true.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in projectListIn) (*mcp.CallToolResult, projectListOut, error) {
		ps, err := db.ListProjects(in.IncludeHidden)
		if err != nil {
			return nil, projectListOut{}, err
		}
		out := projectListOut{Projects: make([]ProjectDTO, 0, len(ps))}
		for _, p := range ps {
			out.Projects = append(out.Projects, projectToDTO(p))
		}
		return nil, out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "project_create",
		Description: "Create a new project. Slug must be unique. task_prefix overrides the auto-generated task id prefix (e.g. slug=WORKLOG with task_prefix=WL yields WL-1, WL-2). When task_prefix is empty the slug is used.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in projectCreateIn) (*mcp.CallToolResult, projectCreateOut, error) {
		p := store.Project{Slug: in.Slug, Name: in.Name, TaskPrefix: in.TaskPrefix}
		if err := db.CreateProject(p); err != nil {
			return nil, projectCreateOut{}, err
		}
		return nil, projectCreateOut{Project: projectToDTO(p)}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "project_update",
		Description: "Update an existing project: rename slug, change name, or set the task_prefix. Empty fields keep the current value. Renaming the slug cascades to tasks via FK.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in projectUpdateIn) (*mcp.CallToolResult, projectUpdateOut, error) {
		all, err := db.ListProjects(true)
		if err != nil {
			return nil, projectUpdateOut{}, err
		}
		var orig *store.Project
		for i := range all {
			if all[i].Slug == in.Slug {
				orig = &all[i]
				break
			}
		}
		if orig == nil {
			return nil, projectUpdateOut{}, fmt.Errorf("project %s not found", in.Slug)
		}
		updated := *orig
		if in.NewSlug != "" {
			updated.Slug = in.NewSlug
		}
		if in.Name != "" {
			updated.Name = in.Name
		}
		if in.TaskPrefix != "" {
			updated.TaskPrefix = in.TaskPrefix
		}
		if err := db.UpdateProject(in.Slug, updated); err != nil {
			return nil, projectUpdateOut{}, err
		}
		return nil, projectUpdateOut{Project: projectToDTO(updated)}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "project_delete",
		Description: "Permanently remove a project. Fails if any task still references it (FK without ON DELETE CASCADE on tasks.project_slug — that's intentional; orphaning tasks is rarely what you want). Hide or reassign tasks first, then delete the project.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in projectDeleteIn) (*mcp.CallToolResult, projectDeleteOut, error) {
		if err := db.DeleteProject(in.Slug); err != nil {
			return nil, projectDeleteOut{}, err
		}
		return nil, projectDeleteOut{Slug: in.Slug, Deleted: true}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "project_set_hidden",
		Description: "Hide or unhide a project. By default the flag also applies to all the project's tasks (and their logs/activities) so nothing dangles visible — pass no_cascade=true to touch only the project row.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, in projectSetHiddenIn) (*mcp.CallToolResult, projectSetHiddenOut, error) {
		if err := db.SetProjectArchived(in.Slug, in.Hidden, !in.NoCascade); err != nil {
			return nil, projectSetHiddenOut{}, err
		}
		return nil, projectSetHiddenOut{Slug: in.Slug, Hidden: in.Hidden, Changed: true}, nil
	})
}
