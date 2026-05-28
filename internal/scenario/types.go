package scenario

type Scenario struct {
	ID          string      `yaml:"id"`
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Metadata    Metadata    `yaml:"metadata"`
	Environment EnvConfig   `yaml:"environment"`
	Input       InputConfig `yaml:"input"`
	Assert      AssertConfig `yaml:"assert"`
}

type Metadata struct {
	Tags []string `yaml:"tags"`
	Tier string   `yaml:"tier"`
	// Generated is true for scenarios produced by `ast gen`. Reports surface
	// these separately so a passing run on a self-generated test is not
	// mistaken for proof of compliance — the same model that wrote the
	// scenario also has to satisfy it.
	Generated   bool   `yaml:"generated,omitempty"`
	GeneratedAt string `yaml:"generated_at,omitempty"`
	GeneratedBy string `yaml:"generated_by,omitempty"` // model id
}

type EnvConfig struct {
	FixtureDir string `yaml:"fixture_dir"`
	InitScript string `yaml:"init_script"`
}

type InputConfig struct {
	UserPrompt string `yaml:"user_prompt"`
}

type AssertConfig struct {
	FileMutations    FileMutationAssert  `yaml:"file_mutations"`
	CommandExecution CommandAssert       `yaml:"command_execution"`
	OutputText       TextAssert          `yaml:"output_text"`
	FileContent      []FileContentAssert `yaml:"file_content"`
}

type FileMutationAssert struct {
	Allowed   []string `yaml:"allowed"`
	Forbidden []string `yaml:"forbidden"`
}

type CommandAssert struct {
	MustHave    []CommandRule `yaml:"must_have"`
	MustNotHave []CommandRule `yaml:"must_not_have"`
}

type CommandRule struct {
	Contains string `yaml:"contains"`
	MinCount int    `yaml:"min_count"`
}

type TextAssert struct {
	MustInclude    []string `yaml:"must_include"`
	MustNotInclude []string `yaml:"must_not_include"`
}

// FileContentAssert checks the post-run contents of mutated files. Each
// rule selects files by glob (matched against MutatedFiles) and applies
// must_match / must_not_match regular expressions against the contents
// captured by the runner.
//
// Example:
//
//   assert:
//     file_content:
//       - glob: "*_test.go"
//         must_match:
//           - "errors\\.Is\\(.*ErrEmptyInput\\)"
//         must_not_match:
//           - "testify"
type FileContentAssert struct {
	Glob          string   `yaml:"glob"`
	MustMatch     []string `yaml:"must_match"`
	MustNotMatch  []string `yaml:"must_not_match"`
	// MatchAllFiles, when true, requires every file matching Glob to satisfy
	// the rule. When false (default), at least one matching file must satisfy
	// it — useful when the agent has latitude over which file to create.
	MatchAllFiles bool `yaml:"match_all_files"`
}
