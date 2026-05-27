package runner

import (
	"context"
	"time"

	"github.com/hhy/ast/internal/scenario"
	"github.com/hhy/ast/internal/skill"
)

type Runner interface {
	Run(ctx context.Context, sk skill.Skill, sc scenario.Scenario, ws string) (*RunResult, error)
}

type RunResult struct {
	ScenarioID   string
	Output       string
	ExecutedCmds []string
	MutatedFiles []string
	// FileContents holds the post-run contents of every entry in MutatedFiles,
	// keyed by the same path. Captured at runner-exit so the workspace can be
	// cleaned up before the judge runs. Files that were deleted (status "D ")
	// are omitted — there is no content to inspect.
	FileContents map[string]string
	Duration     time.Duration
}
