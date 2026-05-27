package judge

import (
	"github.com/hhy/ast/internal/runner"
	"github.com/hhy/ast/internal/scenario"
)

type Judge interface {
	Judge(result *runner.RunResult, scenario scenario.Scenario) (*Judgement, error)
}

type Judgement struct {
	Passed bool
	Errors []string
}
