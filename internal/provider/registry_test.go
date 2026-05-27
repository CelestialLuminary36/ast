package provider

import (
	"strings"
	"testing"

	"github.com/hhy/ast/internal/skill"
)

func TestBuildToolList_EmptyDefsReturnsAllBuiltins(t *testing.T) {
	got, err := BuildToolList(skill.Skill{})
	if err != nil {
		t.Fatalf("BuildToolList: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("expected 4 builtin tools, got %d", len(got))
	}
	names := map[string]bool{}
	for _, td := range got {
		names[td.Name] = true
	}
	for _, want := range []string{"read_file", "edit_file", "run_command", "list_files"} {
		if !names[want] {
			t.Errorf("missing builtin tool %q in output", want)
		}
	}
}

func TestBuildToolList_BuiltinReferenceWhitelist(t *testing.T) {
	sk := skill.Skill{
		ToolDefs: []skill.ToolDef{
			{Name: "read_file"},
		},
	}
	got, err := BuildToolList(sk)
	if err != nil {
		t.Fatalf("BuildToolList: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(got))
	}
	if got[0].Name != "read_file" {
		t.Fatalf("name = %q, want read_file", got[0].Name)
	}
}

func TestBuildToolList_UnknownBuiltinReferenceErrors(t *testing.T) {
	sk := skill.Skill{
		ToolDefs: []skill.ToolDef{
			{Name: "definitely_not_a_builtin"},
		},
	}
	_, err := BuildToolList(sk)
	if err == nil {
		t.Fatal("expected error for unknown builtin reference, got nil")
	}
	if !strings.Contains(err.Error(), "definitely_not_a_builtin") {
		t.Fatalf("error should name the bad tool, got: %v", err)
	}
}

func TestBuildToolList_CustomToolForwarded(t *testing.T) {
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
	got, err := BuildToolList(sk)
	if err != nil {
		t.Fatalf("BuildToolList: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(got))
	}
	if got[0].Name != "run_test" {
		t.Fatalf("name = %q, want run_test", got[0].Name)
	}
}
