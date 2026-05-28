package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAddEntry(t *testing.T) {
	r := &Report{Project: "test", SkillName: "s", SkillPath: "./s"}
	r.AddEntry(Entry{ScenarioID: "a", Passed: true})
	r.AddEntry(Entry{ScenarioID: "b", Passed: false})
	r.AddEntry(Entry{ScenarioID: "c", Passed: true})

	if r.Passed != 2 {
		t.Errorf("Passed = %d, want 2", r.Passed)
	}
	if r.Failed != 1 {
		t.Errorf("Failed = %d, want 1", r.Failed)
	}
	if len(r.Entries) != 3 {
		t.Errorf("len(Entries) = %d, want 3", len(r.Entries))
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	r := &Report{
		Project:   "test-proj",
		SkillName: "go-reviewer",
		SkillPath: "./skills/go-reviewer",
		Timestamp: time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC),
	}
	r.AddEntry(Entry{
		ScenarioID:   "nil-check",
		Passed:       true,
		Output:       "review done",
		ExecutedCmds: []string{"go vet ./..."},
		MutatedFiles: []string{"pkg/foo.go"},
		Duration:     time.Second * 3,
	})
	r.AddEntry(Entry{
		ScenarioID:   "forbidden-edit",
		Passed:       false,
		Output:       "sorry, can't touch that",
		Errors:       []string{"forbidden file modified: go.mod"},
		Duration:     time.Second * 1,
	})

	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")

	if err := Save(path, r); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Project != r.Project {
		t.Errorf("Project = %q, want %q", loaded.Project, r.Project)
	}
	if loaded.Passed != 1 || loaded.Failed != 1 {
		t.Errorf("Passed/Failed = %d/%d, want 1/1", loaded.Passed, loaded.Failed)
	}
	if len(loaded.Entries) != 2 {
		t.Fatalf("len(Entries) = %d, want 2", len(loaded.Entries))
	}
	if loaded.Entries[0].ScenarioID != "nil-check" {
		t.Errorf("Entries[0].ScenarioID = %q", loaded.Entries[0].ScenarioID)
	}
	if loaded.Entries[1].ScenarioID != "forbidden-edit" {
		t.Errorf("Entries[1].ScenarioID = %q", loaded.Entries[1].ScenarioID)
	}
}

func TestSaveMarkdown(t *testing.T) {
	r := &Report{
		Project:   "test-proj",
		SkillName: "go-reviewer",
		SkillPath: "./skills/go-reviewer",
		Timestamp: time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC),
	}
	r.AddEntry(Entry{
		ScenarioID:   "nil-check",
		Passed:       true,
		Duration:     time.Second,
		MutatedFiles: []string{"pkg/foo.go"},
	})
	r.AddEntry(Entry{
		ScenarioID: "bad-edit",
		Passed:     false,
		Errors:     []string{"forbidden file: go.mod"},
		Duration:   time.Second,
	})

	dir := t.TempDir()
	path := filepath.Join(dir, "report.md")

	if err := r.SaveMarkdown(path); err != nil {
		t.Fatalf("SaveMarkdown: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	md := string(data)

	// Basic structure checks — not a full MD parser, just verify
	// the key content made it into the file.
	want := []string{
		"# Skill Check Report",
		"test-proj",
		"go-reviewer",
		"## Scenarios",
		"nil-check",
		"PASS",
		"bad-edit",
		"FAIL",
		"forbidden file: go.mod",
		"TOTAL",
	}
	for _, s := range want {
		if !strings.Contains(md, s) {
			t.Errorf("markdown missing %q", s)
		}
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/report.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}
