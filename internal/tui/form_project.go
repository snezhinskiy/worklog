package tui

import (
	"fmt"
	"strings"
)

// newProjectForm builds the create-project form (/project create). Slug is
// uppercased; auto-rejects duplicates against the in-memory list before the
// store would.
func newProjectForm() *cmdForm {
	f := &cmdForm{
		title: "New project  ·  /project create",
		fields: []*formField{
			newTextField("Slug", "", 16),
			newTextField("Name", "", 40),
			newTextField("Task prefix", "", 8),
		},
		onSubmit: func(m *Model, f *cmdForm) error {
			slug := strings.ToUpper(strings.TrimSpace(f.fields[0].text.Value()))
			name := strings.TrimSpace(f.fields[1].text.Value())
			prefix := strings.ToUpper(strings.TrimSpace(f.fields[2].text.Value()))
			if slug == "" {
				return fmt.Errorf("slug is required")
			}
			for _, p := range m.projects {
				if p.Slug == slug {
					return fmt.Errorf("project %s already exists", slug)
				}
			}
			p := Project{Slug: slug, Name: name, TaskPrefix: prefix}
			if m.db != nil {
				if err := m.db.CreateProject(p); err != nil {
					return err
				}
				return m.reload()
			}
			m.projects = append(m.projects, p)
			return nil
		},
	}
	f.refocusInputs()
	return f
}

// newEditProjectForm prefills slug + name; slug is editable but renamed in
// tasks referencing the old slug to keep mock data consistent. The DB
// cascade does the same via FK ON UPDATE.
func newEditProjectForm(m *Model, slug string) *cmdForm {
	var orig Project
	for _, p := range m.projects {
		if p.Slug == slug {
			orig = p
			break
		}
	}
	f := &cmdForm{
		title: "Edit project " + slug + "  ·  /project edit",
		fields: []*formField{
			newTextField("Slug", orig.Slug, 16),
			newTextField("Name", orig.Name, 40),
			newTextField("Task prefix", orig.TaskPrefix, 8),
		},
		onSubmit: func(m *Model, f *cmdForm) error {
			newSlug := strings.ToUpper(strings.TrimSpace(f.fields[0].text.Value()))
			newName := strings.TrimSpace(f.fields[1].text.Value())
			newPrefix := strings.ToUpper(strings.TrimSpace(f.fields[2].text.Value()))
			if newSlug == "" {
				return fmt.Errorf("slug is required")
			}
			if newSlug != slug {
				for _, p := range m.projects {
					if p.Slug == newSlug {
						return fmt.Errorf("project %s already exists", newSlug)
					}
				}
			}
			if m.db != nil {
				np := orig
				np.Slug, np.Name, np.TaskPrefix = newSlug, newName, newPrefix
				if err := m.db.UpdateProject(slug, np); err != nil {
					return err
				}
				return m.reload()
			}
			// in-memory: rename project + cascade to tasks
			if newSlug != slug {
				for i := range m.tasks {
					if m.tasks[i].Project == slug {
						m.tasks[i].Project = newSlug
					}
				}
			}
			for i := range m.projects {
				if m.projects[i].Slug == slug {
					m.projects[i].Slug = newSlug
					m.projects[i].Name = newName
					m.projects[i].TaskPrefix = newPrefix
					break
				}
			}
			return nil
		},
	}
	f.refocusInputs()
	return f
}
