package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseHours parses the hour-input formats accepted across CLI, TUI, and
// MCP. Accepts plain numbers ("2", "1.5"), Go duration syntax ("2h",
// "30m", "1h30m"), and bare-suffix forms ("2h", "45m") for ergonomics.
// Returns hours as float64.
func ParseHours(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("hours: empty")
	}
	// Pure number first — "2" should not be parsed by ParseDuration as 2ns.
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f, nil
	}
	// time.ParseDuration handles "2h", "30m", "1h30m", "1h15m30s".
	if d, err := time.ParseDuration(s); err == nil {
		return d.Hours(), nil
	}
	return 0, fmt.Errorf("hours %q: expected forms like 2, 1.5, 2h, 30m, 1h30m", s)
}

// IsActivityType reports whether t is one of the known activity types.
func IsActivityType(t string) bool {
	for _, k := range ActivityTypes {
		if k == t {
			return true
		}
	}
	return false
}

// ValidateActivity checks the type-and-payload invariants every UI agrees
// on: type must be known, and at least one of URL or Text must be set
// (otherwise the activity has nothing to display). It does NOT check
// TaskID — that's an insertion-time concern for the store.
func ValidateActivity(a Activity) error {
	if !IsActivityType(a.Type) {
		return fmt.Errorf("type %q: expected one of %s", a.Type, strings.Join(ActivityTypes, ", "))
	}
	if a.URL == "" && a.Text == "" {
		return errors.New("activity must have at least url or text")
	}
	return nil
}

// ValidateProject checks project invariants applied at create/update time.
func ValidateProject(p Project) error {
	if strings.TrimSpace(p.Slug) == "" {
		return errors.New("project slug is required")
	}
	return nil
}

// ValidateTask checks task invariants applied at create/update time.
// Status is left to the workflow layer — defaults are filled in when empty.
func ValidateTask(t Task) error {
	if strings.TrimSpace(t.Project) == "" {
		return errors.New("task project is required")
	}
	if strings.TrimSpace(t.Short) == "" {
		return errors.New("task title is required")
	}
	return nil
}

// ValidateLog checks log-entry invariants applied at create/update time.
func ValidateLog(l LogEntry) error {
	if strings.TrimSpace(l.TaskID) == "" {
		return errors.New("log task_id is required")
	}
	if l.Hours <= 0 {
		return errors.New("hours must be > 0")
	}
	return nil
}
