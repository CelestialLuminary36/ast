package skill

type Skill struct {
	ID           string
	Name         string
	Path         string
	Instructions string
	ToolDefs     []ToolDef
	Meta         map[string]any
}

// ToolDef mirrors the Anthropic tool definition format.
// If InputSchema is nil/empty, the runner treats the entry as a reference
// to a builtin tool (looked up by Name in the runner's registry).
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
}
