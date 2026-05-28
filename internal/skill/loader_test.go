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

func TestLoadFromDir_AnthropicFormat_TagsFormat(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "skill.yaml", "id: s1\nname: S1\n")
	writeFile(t, dir, "instructions.md", "do stuff")

	s, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if s.Format != FormatAnthropic {
		t.Fatalf("Format = %q, want %q", s.Format, FormatAnthropic)
	}
}

func TestLoadFromDir_CursorLegacyFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".cursorrules", "Always use TypeScript strict mode.")

	s, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if s.Format != FormatCursorRules {
		t.Fatalf("Format = %q, want %q", s.Format, FormatCursorRules)
	}
	if !strings.Contains(s.Instructions, "TypeScript strict") {
		t.Fatalf("instructions missing body, got: %q", s.Instructions)
	}
	if len(s.ToolDefs) != 0 {
		t.Fatalf("Cursor format must not invent tool defs, got %d", len(s.ToolDefs))
	}
}

func TestLoadFromDir_CursorMDC(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".cursor/rules/01-style.mdc", `---
description: Code style rules
globs: ["**/*.ts"]
alwaysApply: true
---

Use 2-space indentation.
`)
	writeFile(t, dir, ".cursor/rules/02-imports.mdc", `Sort imports alphabetically.`)

	s, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if s.Format != FormatCursorRules {
		t.Fatalf("Format = %q, want %q", s.Format, FormatCursorRules)
	}
	if !strings.Contains(s.Instructions, "2-space indentation") {
		t.Fatalf("first rule body missing from instructions")
	}
	if !strings.Contains(s.Instructions, "alphabetically") {
		t.Fatalf("second rule body missing from instructions")
	}
	if d, _ := s.Meta["description"].(string); d != "Code style rules" {
		t.Fatalf("description from first mdc frontmatter missing, got %v", s.Meta["description"])
	}
}

func TestLoadFromDir_AgentsMD(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "AGENTS.md", `---
id: my-agent
name: My Agent
---

# Agent Instructions

Be careful.
`)

	s, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if s.Format != FormatAgentsMD {
		t.Fatalf("Format = %q, want %q", s.Format, FormatAgentsMD)
	}
	if s.ID != "my-agent" {
		t.Fatalf("ID from frontmatter = %q, want my-agent", s.ID)
	}
	if s.Name != "My Agent" {
		t.Fatalf("Name from frontmatter = %q, want 'My Agent'", s.Name)
	}
	if !strings.Contains(s.Instructions, "Be careful") {
		t.Fatalf("body missing, got: %q", s.Instructions)
	}
	if strings.HasPrefix(s.Instructions, "---") {
		t.Fatalf("instructions should not start with frontmatter fence")
	}
}

func TestLoadFromDir_AgentsMD_NoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "AGENTS.md", "# Just a plain markdown agent file.\n\nNo frontmatter here.")

	s, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if s.Format != FormatAgentsMD {
		t.Fatalf("Format = %q, want agents-md", s.Format)
	}
	if !strings.Contains(s.Instructions, "Just a plain markdown") {
		t.Fatalf("body missing")
	}
	// ID/Name fall back to dir basename.
	if s.ID == "" || s.Name == "" {
		t.Fatalf("ID/Name should fall back to dir basename, got %q / %q", s.ID, s.Name)
	}
}

func TestLoadFromDir_FrontmatterFallback(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "rules.md", `---
name: weird-format
description: Custom layout
---

Body text here.
`)

	s, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if s.Format != FormatFrontmatter {
		t.Fatalf("Format = %q, want %q", s.Format, FormatFrontmatter)
	}
	if s.Name != "weird-format" {
		t.Fatalf("Name = %q, want weird-format", s.Name)
	}
	if !strings.Contains(s.Instructions, "Body text here") {
		t.Fatalf("body missing")
	}
}

func TestLoadFromDir_NoRecognisableFormat(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "random.txt", "not a skill")

	_, err := LoadFromDir(dir)
	if err == nil {
		t.Fatal("expected error for unrecognisable layout, got nil")
	}
	if !strings.Contains(err.Error(), "no recognisable skill format") {
		t.Fatalf("error message should explain what was looked for, got: %v", err)
	}
}

func TestLoadFromDir_DetectionPriority(t *testing.T) {
	// When skill.yaml AND AGENTS.md both exist, Anthropic wins because it's
	// the most specific (it carries id/name/version/tools — losing that
	// fidelity to fall back to plain markdown would be wrong).
	dir := t.TempDir()
	writeFile(t, dir, "skill.yaml", "id: anthropic-one\nname: Anthropic One\n")
	writeFile(t, dir, "instructions.md", "Anthropic instructions.")
	writeFile(t, dir, "AGENTS.md", "Agents instructions.")

	s, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	if s.Format != FormatAnthropic {
		t.Fatalf("Format = %q, want anthropic (most-specific format must win)", s.Format)
	}
	if !strings.Contains(s.Instructions, "Anthropic") {
		t.Fatalf("anthropic instructions must take precedence")
	}
}

func TestSplitFrontmatter(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantFM   string
		wantBody string
	}{
		{
			name:     "well-formed LF",
			input:    "---\nname: x\n---\nbody\n",
			wantFM:   "name: x",
			wantBody: "body\n",
		},
		{
			name:     "well-formed CRLF (Windows checkout)",
			input:    "---\r\nname: x\r\n---\r\nbody\r\n",
			wantFM:   "name: x",
			wantBody: "body\n",
		},
		{
			name:     "no frontmatter returns body verbatim",
			input:    "just text\n",
			wantFM:   "",
			wantBody: "just text\n",
		},
		{
			name:     "unclosed frontmatter is not a fence",
			input:    "---\nname: x\n\nbody\n",
			wantFM:   "",
			wantBody: "---\nname: x\n\nbody\n",
		},
		{
			name:     "BOM is stripped before fence check",
			input:    "\xef\xbb\xbf---\nname: x\n---\nbody",
			wantFM:   "name: x",
			wantBody: "body",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fm, body := splitFrontmatter([]byte(c.input))
			if string(fm) != c.wantFM {
				t.Errorf("frontmatter = %q, want %q", fm, c.wantFM)
			}
			if body != c.wantBody {
				t.Errorf("body = %q, want %q", body, c.wantBody)
			}
		})
	}
}
