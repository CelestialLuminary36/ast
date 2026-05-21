package judge

import (
	"github.com/CelestialLuminary36/agent-skill-test/internal/runner"
	"github.com/CelestialLuminary36/agent-skill-test/internal/scenario"
)

type Judge interface {
	Judge(result *runner.RunResult, scenario scenario.Scenario) (*Judgement, error)
}

type Judgement struct {
	Passed bool
	Errors []string
}
