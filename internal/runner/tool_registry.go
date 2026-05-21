package runner

import (
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/CelestialLuminary36/agent-skill-test/internal/skill"
)

// builtinTools is the registry of tools the runtime knows how to execute
// (see ToolExecutor.Execute in tools.go). A skill can either:
//   - declare no tools/ directory → all builtins are exposed (backwards compat)
//   - reference builtins by name in tools/*.json with no input_schema
//   - declare fully custom tools with their own input_schema (returns
//     "unknown tool" at runtime since no executor handles them — this is
//     intentional; skill authors should know what's wired up)
var builtinTools = map[string]anthropic.ToolParam{
	"read_file": {
		Name:        "read_file",
		Description: anthropic.String("Read the contents of a file in the workspace."),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"path": map[string]string{"type": "string", "description": "Relative path to the file"},
			},
			Required: []string{"path"},
		},
	},
	"edit_file": {
		Name:        "edit_file",
		Description: anthropic.String("Write or overwrite a file in the workspace with the given content."),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"path":    map[string]string{"type": "string", "description": "Relative path to the file"},
				"content": map[string]string{"type": "string", "description": "Full content to write"},
			},
			Required: []string{"path", "content"},
		},
	},
	"run_command": {
		Name:        "run_command",
		Description: anthropic.String("Run a shell command in the workspace. Dangerous commands are blocked by the sandbox."),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"command": map[string]string{"type": "string", "description": "Shell command to execute"},
			},
			Required: []string{"command"},
		},
	},
	"list_files": {
		Name:        "list_files",
		Description: anthropic.String("List files in a workspace directory (default '.')."),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"path": map[string]string{"type": "string", "description": "Relative directory path (optional)"},
			},
		},
	},
}

// allBuiltins returns every builtin tool wrapped for the messages API.
// Used when a skill has no tools/ directory.
func allBuiltins() []anthropic.ToolUnionParam {
	out := make([]anthropic.ToolUnionParam, 0, len(builtinTools))
	for _, name := range []string{"read_file", "edit_file", "run_command", "list_files"} {
		tp := builtinTools[name]
		out = append(out, anthropic.ToolUnionParam{OfTool: &tp})
	}
	return out
}

// buildToolDefs translates a skill's declared tool whitelist into the slice
// the messages API expects. An empty ToolDefs slice means "skill didn't
// constrain anything" → expose all builtins.
func buildToolDefs(sk skill.Skill) ([]anthropic.ToolUnionParam, error) {
	if len(sk.ToolDefs) == 0 {
		return allBuiltins(), nil
	}

	out := make([]anthropic.ToolUnionParam, 0, len(sk.ToolDefs))
	for _, def := range sk.ToolDefs {
		if len(def.InputSchema) == 0 {
			// builtin reference
			tp, ok := builtinTools[def.Name]
			if !ok {
				return nil, fmt.Errorf("skill references unknown builtin tool %q (available: read_file, edit_file, run_command, list_files)", def.Name)
			}
			out = append(out, anthropic.ToolUnionParam{OfTool: &tp})
			continue
		}
		// custom tool — declared by skill author, no builtin executor wired up
		tp := anthropic.ToolParam{
			Name:        def.Name,
			Description: anthropic.String(def.Description),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: def.InputSchema["properties"],
				Required:   stringSlice(def.InputSchema["required"]),
			},
		}
		out = append(out, anthropic.ToolUnionParam{OfTool: &tp})
	}
	return out, nil
}

// stringSlice converts a json.Unmarshal'd []any into []string. Returns nil
// if the input isn't a slice.
func stringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
