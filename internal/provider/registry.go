package provider

import (
	"fmt"

	"github.com/CelestialLuminary36/ast/internal/skill"
)

// builtinTools is the registry of tools the runtime knows how to execute.
var builtinTools = map[string]ToolDef{
	"read_file": {
		Name:        "read_file",
		Description: "Read the contents of a file in the workspace.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "Relative path to the file"},
			},
			"required": []any{"path"},
		},
	},
	"edit_file": {
		Name:        "edit_file",
		Description: "Write or overwrite a file in the workspace with the given content.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":    map[string]any{"type": "string", "description": "Relative path to the file"},
				"content": map[string]any{"type": "string", "description": "Full content to write"},
			},
			"required": []any{"path", "content"},
		},
	},
	"run_command": {
		Name:        "run_command",
		Description: "Run a shell command in the workspace. Dangerous commands are blocked by the sandbox.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "Shell command to execute"},
			},
			"required": []any{"command"},
		},
	},
	"list_files": {
		Name:        "list_files",
		Description: "List files in a workspace directory (default '.').",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{"type": "string", "description": "Relative directory path (optional)"},
			},
		},
	},
}

// BuiltinToolList returns every builtin tool. Used when a skill has no tools/ directory.
func BuiltinToolList() []ToolDef {
	return []ToolDef{
		builtinTools["read_file"],
		builtinTools["edit_file"],
		builtinTools["run_command"],
		builtinTools["list_files"],
	}
}

// BuildToolList translates a skill's declared tool whitelist into canonical
// tool definitions. An empty ToolDefs slice means "skill didn't constrain
// anything" → expose all builtins.
func BuildToolList(sk skill.Skill) ([]ToolDef, error) {
	if len(sk.ToolDefs) == 0 {
		return BuiltinToolList(), nil
	}

	out := make([]ToolDef, 0, len(sk.ToolDefs))
	for _, def := range sk.ToolDefs {
		if len(def.InputSchema) == 0 {
			// builtin reference
			bt, ok := builtinTools[def.Name]
			if !ok {
				return nil, fmt.Errorf("skill references unknown builtin tool %q (available: read_file, edit_file, run_command, list_files)", def.Name)
			}
			out = append(out, bt)
			continue
		}
		// custom tool — declared by skill author, no builtin executor wired up
		out = append(out, ToolDef{
			Name:        def.Name,
			Description: def.Description,
			InputSchema: def.InputSchema,
		})
	}
	return out, nil
}
