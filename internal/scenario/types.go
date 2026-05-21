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
}

type EnvConfig struct {
	FixtureDir string `yaml:"fixture_dir"`
	InitScript string `yaml:"init_script"`
}

type InputConfig struct {
	UserPrompt string `yaml:"user_prompt"`
}

type AssertConfig struct {
	FileMutations    FileMutationAssert `yaml:"file_mutations"`
	CommandExecution CommandAssert      `yaml:"command_execution"`
	OutputText       TextAssert         `yaml:"output_text"`
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
