package runner

import (
	"context"
	"time"

	"github.com/CelestialLuminary36/agent-skill-test/internal/scenario"
	"github.com/CelestialLuminary36/agent-skill-test/internal/skill"
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
