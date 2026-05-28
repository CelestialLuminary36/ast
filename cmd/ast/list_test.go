package main

import (
	"os"
	"path/filepath"
	"testing"
)

// writeSkillFile is a tiny helper for the table-driven tests below.
func writeSkillFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestInspectSkillDir_AnthropicOK(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "demo")
	scenariosDir := filepath.Join(root, "scenarios")

	writeSkillFile(t, filepath.Join(skillDir, "skill.yaml"), "id: demo\nname: \"Demo\"\n")
	writeSkillFile(t, filepath.Join(skillDir, "instructions.md"), "Do useful demonstrations of the workflow.\n")
	writeSkillFile(t, filepath.Join(skillDir, "tools", "read_file.json"), `{"name": "read_file"}`)
	writeSkillFile(t, filepath.Join(scenariosDir, "demo", "happy.yaml"), `id: happy
name: happy path
input:
  user_prompt: "do the thing"
`)

	row := inspectSkillDir(skillDir, "demo", scenariosDir)

	if row.id != "demo" {
		t.Errorf("id = %q; want %q", row.id, "demo")
	}
	if row.format != "anthropic" {
		t.Errorf("format = %q; want anthropic", row.format)
	}
	if row.scenarios != 1 {
		t.Errorf("scenarios = %d; want 1", row.scenarios)
	}
	// Status may be OK or WARN depending on whether validateSkill is
	// happy about every detail (it warns about missing skill.yaml when
	// absent; we provided one, but it still emits a "detected format"
	// info-warning). Accept either — what matters is it's not ERROR.
	if row.status == "ERROR" {
		t.Errorf("status = ERROR (%s); want non-error", row.detail)
	}
}

func TestInspectSkillDir_UnrecognisedFormat(t *testing.T) {
	// A directory that has none of skill.yaml / .cursorrules / AGENTS.md /
	// frontmatter files. The loader should error and the row should
	// surface that as ERROR — silently skipping would hide typos.
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "empty")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	row := inspectSkillDir(skillDir, "empty", filepath.Join(root, "scenarios"))

	if row.status != "ERROR" {
		t.Errorf("status = %q; want ERROR", row.status)
	}
	if row.id != "-" || row.format != "-" {
		t.Errorf("expected placeholders for failed load, got id=%q format=%q", row.id, row.format)
	}
	if row.detail == "" {
		t.Error("expected non-empty detail explaining the failure")
	}
}

func TestInspectSkillDir_NoScenarios(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "lonely")

	writeSkillFile(t, filepath.Join(skillDir, "skill.yaml"), "id: lonely\nname: \"Lonely\"\n")
	writeSkillFile(t, filepath.Join(skillDir, "instructions.md"), "Spend time alone, productively.\n")

	row := inspectSkillDir(skillDir, "lonely", filepath.Join(root, "scenarios"))

	if row.scenarios != 0 {
		t.Errorf("scenarios = %d; want 0", row.scenarios)
	}
	// validateSkill will warn about missing scenarios dir, so status is WARN.
	if row.status == "ERROR" {
		t.Errorf("status = ERROR (%s); want WARN", row.detail)
	}
}
