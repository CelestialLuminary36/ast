package skill

// Format identifies which on-disk layout produced this Skill. Populated
// by the loader's format-detector so callers can branch on provenance —
// e.g. validate emits format-specific warnings, the API runner refuses to
// expose tools for formats that don't declare them.
type Format string

const (
	FormatAnthropic   Format = "anthropic"   // skill.yaml + instructions.md + tools/
	FormatCursorRules Format = "cursor"      // .cursor/rules/*.mdc or .cursorrules
	FormatAgentsMD    Format = "agents-md"   // AGENTS.md (Codex / general convention)
	FormatFrontmatter Format = "frontmatter" // any .md with YAML frontmatter
)

type Skill struct {
	ID           string
	Name         string
	Path         string
	Format       Format
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
