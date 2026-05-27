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
	Duration     time.Duration
}
