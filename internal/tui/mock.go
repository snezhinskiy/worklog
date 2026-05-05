package tui

import (
	"time"

	"github.com/snezhinskiy/worklog/internal/domain"
	"github.com/snezhinskiy/worklog/internal/store"
)

// Re-export store types under their old TUI names so existing renderers
// (forms.go, picker.go, view.go, etc.) keep compiling without per-call
// changes.
type (
	Project  = store.Project
	Task     = store.Task
	LogEntry = store.LogEntry
)

// Frozen "today" used by the in-memory demo. Real DB-backed runs use the
// actual current date.
func mockToday() time.Time {
	return time.Date(2026, 4, 26, 0, 0, 0, 0, time.Local)
}

// mockData returns demo content anchored at the prototype's frozen "today"
// (Apr 26 2026). Used by snapshot dumps so screenshots stay stable.
func mockData() ([]Project, []Task, []LogEntry, []store.Activity) {
	return mockDataFor(mockToday())
}

// mockDataFor returns demo content anchored at the given day. The seed is
// English, all-fictional, and deliberately mixes short and long titles /
// notes / URLs to stress-test column widths and wrapping in the TUI.
func mockDataFor(t time.Time) ([]Project, []Task, []LogEntry, []store.Activity) {
	projects := []Project{
		{Slug: "AURA", Name: "Aurora Identity", TaskPrefix: "AU"},
		{Slug: "WORKLOG", Name: "Worklog CLI", TaskPrefix: "WL"},
		{Slug: "SHOP", Name: "Storefront", TaskPrefix: "SH"},
		{Slug: "INFRA", Name: "Infrastructure", TaskPrefix: "IN"},
	}

	d := func(off int) time.Time { return t.AddDate(0, 0, -off) }

	tasks := []Task{
		// AURA — varied sizes
		{Project: "AURA", ExternalID: "AU-3569", Status: "in_progress",
			Short: "Wire SSO into the new identity service", StatusChangedAt: d(12)},
		{Project: "AURA", ExternalID: "AU-3580", Status: "to_test",
			Short: "Refresh-token TTL rework — investigate why long-lived sessions silently expire after a backend restart and design a migration path that doesn't kick everyone out",
			StatusChangedAt: d(2)},
		{Project: "AURA", ExternalID: "AU-3601", Status: "todo",
			Short: "MFA enrolment", StatusChangedAt: d(1)},
		{Project: "AURA", ExternalID: "AU-3612", Status: "done",
			Short: "Add password breach check on signup", StatusChangedAt: d(0)},
		{Project: "AURA", ExternalID: "AU-3620", Status: "backlog",
			Short: "Audit log export for SOC2", StatusChangedAt: d(4)},

		// WORKLOG — short identifiers, mixed
		{Project: "WORKLOG", ExternalID: "WL-1", Status: "in_progress",
			Short: "TUI prototype", StatusChangedAt: d(0)},
		{Project: "WORKLOG", ExternalID: "WL-2", Status: "todo",
			Short: "MCP server skeleton", StatusChangedAt: d(3)},
		{Project: "WORKLOG", ExternalID: "WL-3", Status: "backlog",
			Short: "CSV / JSON export support", StatusChangedAt: d(5)},
		{Project: "WORKLOG", ExternalID: "WL-4", Status: "to_deploy",
			Short: "Hide / unhide flow", StatusChangedAt: d(1)},

		// SHOP — long-ish
		{Project: "SHOP", ExternalID: "SH-114", Status: "to_deploy",
			Short: "Homepage promo banner with seasonal copy and a fallback for ad-blocker users", StatusChangedAt: d(4)},
		{Project: "SHOP", ExternalID: "SH-120", Status: "on_hold",
			Short: "Cart A/B experiment", StatusChangedAt: d(10)},
		{Project: "SHOP", ExternalID: "SH-128", Status: "in_progress",
			Short: "Checkout: handle 3-D Secure step-up gracefully on mobile", StatusChangedAt: d(2)},
		{Project: "SHOP", ExternalID: "SH-131", Status: "todo",
			Short: "Tax", StatusChangedAt: d(6)},

		// INFRA — terse
		{Project: "INFRA", ExternalID: "IN-42", Status: "todo",
			Short: "Bump runner to 2.330", StatusChangedAt: d(2)},
		{Project: "INFRA", ExternalID: "IN-50", Status: "backlog",
			Short: "Backup pricing review", StatusChangedAt: d(7)},
		{Project: "INFRA", ExternalID: "IN-58", Status: "in_progress",
			Short: "Migrate Postgres 14 → 16 on staging, then prod, with zero-downtime cutover and a tested rollback",
			StatusChangedAt: d(8)},
	}

	day := func(off int) time.Time { return t.AddDate(0, 0, off) }

	logs := []LogEntry{
		// today
		{TaskID: "AU-3569", Date: day(0), Time: "10:32", Hours: 1.5,
			Note: "rebased atop main; resolved conflicts in users_aurora_link migration"},
		{TaskID: "AU-3569", Date: day(0), Time: "16:05", Hours: 2.0,
			Note: "wrote tests for refresh flow"},
		{TaskID: "AU-3612", Date: day(0), Time: "11:00", Hours: 0.5,
			Note: "code review feedback addressed"},
		{TaskID: "WL-1", Date: day(0), Time: "18:40", Hours: 1.0,
			Note: "polished palette UX"},

		// yesterday
		{TaskID: "WL-1", Date: day(-1), Time: "11:00", Hours: 2.5,
			Note: "first cut of the search bar"},
		{TaskID: "WL-4", Date: day(-1), Time: "15:10", Hours: 1.0, Note: "shipped"},

		// 2-3 days back
		{TaskID: "AU-3569", Date: day(-3), Time: "09:50", Hours: 4.0,
			Note: "schema for users_aurora_link plus a hand-written backfill that walks 50M rows in batches"},
		{TaskID: "SH-114", Date: day(-3), Time: "15:00", Hours: 1.5,
			Note: "banner markup + responsive tweaks"},
		{TaskID: "SH-128", Date: day(-3), Time: "16:30", Hours: 2.0, Note: "step-up POC"},

		// 4-5 days back
		{TaskID: "AU-3580", Date: day(-4), Time: "10:15", Hours: 3.0, Note: "TTL RFC draft"},
		{TaskID: "AU-3580", Date: day(-4), Time: "14:30", Hours: 2.5, Note: "impl + unit tests"},
		{TaskID: "IN-42", Date: day(-4), Time: "17:20", Hours: 0.5, Note: ""},
		{TaskID: "AU-3612", Date: day(-5), Time: "11:45", Hours: 3.5,
			Note: "integrated HaveIBeenPwned, decided to debounce"},
		{TaskID: "AU-3612", Date: day(-5), Time: "15:30", Hours: 2.0,
			Note: "fallback when API is unreachable"},

		// week+ back
		{TaskID: "SH-114", Date: day(-6), Time: "10:00", Hours: 4.0, Note: "design signed off"},
		{TaskID: "SH-120", Date: day(-6), Time: "14:00", Hours: 1.0,
			Note: "paused per product — waiting on cohort definition"},
		{TaskID: "IN-58", Date: day(-9), Time: "09:30", Hours: 6.0,
			Note: "ran a full dry-run on staging; replication caught up in 18 minutes"},
		{TaskID: "IN-58", Date: day(-9), Time: "16:00", Hours: 1.5,
			Note: "rollback playbook"},
	}

	activities := []store.Activity{
		// AU-3569: real-feeling MR + commit + deploy + notes
		{TaskID: "AU-3569", Type: "mr",
			URL:  "https://gitlab.example.com/aurora/identity/-/merge_requests/418",
			Text: "wire SSO into identity service"},
		{TaskID: "AU-3569", Type: "commit",
			URL:  "https://gitlab.example.com/aurora/identity/-/commit/a3b1f9e",
			Text: "fix: serialize aurora_link_id as string"},
		{TaskID: "AU-3569", Type: "deploy",
			URL:  "https://ci.example.com/pipelines/198421",
			Text: "stage"},
		{TaskID: "AU-3569", Type: "note",
			Text: "infra team confirmed Vault rotation policy is fine for our 24h tokens"},

		// AU-3580: long URL, terse note
		{TaskID: "AU-3580", Type: "mr",
			URL:  "https://gitlab.example.com/aurora/identity/-/merge_requests/421?diff=files&page=2",
			Text: "TTL rework"},
		{TaskID: "AU-3580", Type: "link",
			URL:  "https://wiki.example.com/spaces/AURA/pages/8821/Refresh-token+TTL+RFC",
			Text: "RFC"},

		// AU-3612: just a link to docs and a deploy
		{TaskID: "AU-3612", Type: "link",
			URL:  "https://haveibeenpwned.com/API/v3",
			Text: "HIBP API docs"},
		{TaskID: "AU-3612", Type: "deploy",
			URL:  "https://ci.example.com/pipelines/199004",
			Text: "prod"},

		// WL-1: short notes only
		{TaskID: "WL-1", Type: "note", Text: "kept palette + picker as one component family"},
		{TaskID: "WL-1", Type: "commit",
			URL: "https://github.com/dima/worklog/commit/4f2a91b",
			Text: "search: smooth-scroll the body"},

		// WL-4: deploy
		{TaskID: "WL-4", Type: "deploy",
			URL: "https://github.com/dima/worklog/releases/tag/v0.3.0", Text: ""},

		// SH-114: design link + MR
		{TaskID: "SH-114", Type: "link",
			URL: "https://figma.com/file/9k2lA/Storefront-Spring-Promo?node-id=14:32",
			Text: "Figma — spring promo"},
		{TaskID: "SH-114", Type: "mr",
			URL:  "https://gitlab.example.com/shop/storefront/-/merge_requests/882",
			Text: "homepage promo banner"},

		// SH-128: bug discussion
		{TaskID: "SH-128", Type: "note",
			Text: "Reproduces only on iOS Safari with low-bandwidth throttling — looks like a race between the 3DS iframe and our retry loop"},

		// IN-58: heavy
		{TaskID: "IN-58", Type: "link",
			URL:  "https://wiki.example.com/x/Postgres+16+Migration+Plan",
			Text: "migration plan"},
		{TaskID: "IN-58", Type: "deploy",
			URL: "https://ci.example.com/pipelines/197210", Text: "staging — dry-run"},
		{TaskID: "IN-58", Type: "note",
			Text: "Replication lag stayed under 200ms throughout the cutover dry-run; logical decoding worked as expected"},
	}

	return projects, tasks, logs, activities
}

// Seed writes mock data into the DB. Only ever called explicitly via the
// --seed flag; we never seed on empty-DB launches because that surprises
// people running worklog for real on a fresh install.
func Seed(db domain.Store) error {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	projects, tasks, logs, activities := mockDataFor(today)
	for _, p := range projects {
		_ = db.CreateProject(p) // ignore "already exists" on re-seed
	}
	for _, t := range tasks {
		if _, err := db.CreateTask(t); err != nil {
			continue
		}
	}
	for _, l := range logs {
		if _, err := db.CreateLog(l); err != nil {
			continue
		}
	}
	for _, a := range activities {
		if _, err := db.CreateActivity(a); err != nil {
			continue
		}
	}
	return nil
}
