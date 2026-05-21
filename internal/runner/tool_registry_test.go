package runner

import (
	"strings"
	"testing"

	"github.com/CelestialLuminary36/agent-skill-test/internal/skill"
)

func TestBuildToolDefs_EmptyDefsReturnsAllBuiltins(t *testing.T) {
	got, err := buildToolDefs(skill.Skill{})
	if err != nil {
		t.Fatalf("buildToolDefs: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("expected 4 builtin tools, got %d", len(got))
	}
	names := map[string]bool{}
	for _, u := range got {
		if u.OfTool == nil {
			t.Fatal("ToolUnionParam.OfTool unexpectedly nil")
		}
		names[u.OfTool.Name] = true
	}
	for _, want := range []string{"read_file", "edit_file", "run_command", "list_files"} {
		if !names[want] {
			t.Errorf("missing builtin tool %q in output", want)
		}
	}
}

func TestBuildToolDefs_BuiltinReferenceWhitelist(t *testing.T) {
	sk := skill.Skill{
		ToolDefs: []skill.ToolDef{
			{Name: "read_file"},
		},
	}
	got, err := buildToolDefs(sk)
	if err != nil {
		t.Fatalf("buildToolDefs: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(got))
	}
	if got[0].OfTool.Name != "read_file" {
		t.Fatalf("name = %q, want read_file", got[0].OfTool.Name)
	}
}

func TestBuildToolDefs_UnknownBuiltinReferenceErrors(t *testing.T) {
	sk := skill.Skill{
		ToolDefs: []skill.ToolDef{
			{Name: "definitely_not_a_builtin"},
		},
	}
	_, err := buildToolDefs(sk)
	if err == nil {
		t.Fatal("expected error for unknown builtin reference, got nil")
	}
	if !strings.Contains(err.Error(), "definitely_not_a_builtin") {
		t.Fatalf("error should name the bad tool, got: %v", err)
	}
}

func TestBuildToolDefs_CustomToolForwarded(t *testing.T) {
	sk := skill.Skill{
		ToolDefs: []skill.ToolDef{
			{
				Name:        "run_test",
				Description: "Run a test package",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"package": map[string]any{"type": "string"},
					},
					"required": []any{"package"},
				},
			},
		},
	}
	got, err := buildToolDefs(sk)
	if err != nil {
		t.Fatalf("buildToolDefs: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(got))
	}
	tp := got[0].OfTool
	if tp.Name != "run_test" {
		t.Fatalf("name = %q, want run_test", tp.Name)
	}
	if tp.InputSchema.Required == nil || len(tp.InputSchema.Required) != 1 || tp.InputSchema.Required[0] != "package" {
		t.Fatalf("required = %v, want [package]", tp.InputSchema.Required)
	}
}
