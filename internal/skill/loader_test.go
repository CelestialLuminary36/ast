package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile writes content to filepath.Join(dir, name), creating parents.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}

func TestLoadFromDir_NoToolsDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "skill.yaml", "id: s1\nname: S1\n")
	writeFile(t, dir, "instructions.md", "do stuff")

	s, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if len(s.ToolDefs) != 0 {
		t.Fatalf("expected 0 tool defs, got %d", len(s.ToolDefs))
	}
}

func TestLoadFromDir_BuiltinReferenceTool(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "skill.yaml", "id: s1\nname: S1\n")
	writeFile(t, dir, "instructions.md", "do stuff")
	writeFile(t, dir, "tools/read_file.json", `{"name":"read_file"}`)

	s, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if len(s.ToolDefs) != 1 {
		t.Fatalf("expected 1 tool def, got %d", len(s.ToolDefs))
	}
	if s.ToolDefs[0].Name != "read_file" {
		t.Fatalf("name = %q, want read_file", s.ToolDefs[0].Name)
	}
	if len(s.ToolDefs[0].InputSchema) != 0 {
		t.Fatalf("InputSchema should be empty for builtin reference")
	}
}

func TestLoadFromDir_CustomToolWithSchema(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "skill.yaml", "id: s1\nname: S1\n")
	writeFile(t, dir, "instructions.md", "do stuff")
	writeFile(t, dir, "tools/run_test.json", `{
		"name": "run_test",
		"description": "Run the project's test suite",
		"input_schema": {
			"type": "object",
			"properties": {"package": {"type": "string"}},
			"required": ["package"]
		}
	}`)

	s, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if len(s.ToolDefs) != 1 {
		t.Fatalf("expected 1 tool def, got %d", len(s.ToolDefs))
	}
	td := s.ToolDefs[0]
	if td.Name != "run_test" || td.Description == "" || len(td.InputSchema) == 0 {
		t.Fatalf("unexpected tool def: %+v", td)
	}
}

func TestLoadFromDir_MissingName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "skill.yaml", "id: s1\nname: S1\n")
	writeFile(t, dir, "instructions.md", "do stuff")
	writeFile(t, dir, "tools/bad.json", `{"description":"no name"}`)

	_, err := LoadFromDir(dir)
	if err == nil {
		t.Fatal("expected error for missing 'name', got nil")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Fatalf("error message should mention 'name', got: %v", err)
	}
}

func TestLoadFromDir_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "skill.yaml", "id: s1\nname: S1\n")
	writeFile(t, dir, "instructions.md", "do stuff")
	writeFile(t, dir, "tools/bad.json", `{not-json`)

	_, err := LoadFromDir(dir)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestLoadFromDir_IgnoresNonJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "skill.yaml", "id: s1\nname: S1\n")
	writeFile(t, dir, "instructions.md", "do stuff")
	writeFile(t, dir, "tools/README.md", "this is documentation")
	writeFile(t, dir, "tools/read_file.json", `{"name":"read_file"}`)

	s, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if len(s.ToolDefs) != 1 {
		t.Fatalf("expected 1 tool def (.md should be ignored), got %d", len(s.ToolDefs))
	}
}
