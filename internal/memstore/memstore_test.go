package memstore

import (
	"testing"

	"github.com/snezhinskiy/worklog/internal/domain"
)

// TestCascadeHide is the proof-of-life: it exercises the contract via the
// interface, not the implementation. If you swap `New()` here for
// `store.Open(":memory:")` the test should still pass.
func TestCascadeHide(t *testing.T) {
	var s domain.Store = New()

	if err := s.CreateProject(domain.Project{Slug: "AURA", Name: "Aurora", TaskPrefix: "AU"}); err != nil {
		t.Fatal(err)
	}
	tk, err := s.CreateTask(domain.Task{Project: "AURA", Short: "wire SSO"})
	if err != nil {
		t.Fatal(err)
	}
	if tk.ExternalID != "AU-1" {
		t.Fatalf("auto id: want AU-1, got %s", tk.ExternalID)
	}
	if _, err := s.CreateLog(domain.LogEntry{TaskID: "AU-1", Hours: 1.5, Note: "spec"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateActivity(domain.Activity{TaskID: "AU-1", Type: "mr", URL: "https://x/1"}); err != nil {
		t.Fatal(err)
	}

	// Hide task with cascade: log + activity should also flip to archived.
	if err := s.SetTaskArchived("AU-1", true, true); err != nil {
		t.Fatal(err)
	}
	logs, _ := s.ListLogs(false)
	if len(logs) != 0 {
		t.Errorf("expected 0 visible logs after cascade hide, got %d", len(logs))
	}
	acts, _ := s.ListActivities("", false)
	if len(acts) != 0 {
		t.Errorf("expected 0 visible activities after cascade hide, got %d", len(acts))
	}

	// Unhide cascading: everything visible again.
	if err := s.SetTaskArchived("AU-1", false, true); err != nil {
		t.Fatal(err)
	}
	logs, _ = s.ListLogs(false)
	acts, _ = s.ListActivities("", false)
	if len(logs) != 1 || len(acts) != 1 {
		t.Errorf("after unhide cascade want 1 log + 1 activity visible, got %d/%d", len(logs), len(acts))
	}
}

func TestValidationRoutedThroughDomain(t *testing.T) {
	var s domain.Store = New()
	_ = s.CreateProject(domain.Project{Slug: "AURA", Name: "Aurora"})
	if _, err := s.CreateTask(domain.Task{Project: "AURA"}); err == nil {
		t.Error("expected ValidateTask to reject empty title")
	}
	if _, err := s.CreateLog(domain.LogEntry{TaskID: "AU-1", Hours: 0}); err == nil {
		t.Error("expected ValidateLog to reject hours=0")
	}
	if _, err := s.CreateActivity(domain.Activity{TaskID: "AU-1", Type: "wrong", URL: "x"}); err == nil {
		t.Error("expected ValidateActivity to reject unknown type")
	}
	if _, err := s.CreateActivity(domain.Activity{TaskID: "AU-1", Type: "mr"}); err == nil {
		t.Error("expected ValidateActivity to require url or text")
	}
}
