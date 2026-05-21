package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/CelestialLuminary36/agent-skill-test/internal/scenario"
	"github.com/CelestialLuminary36/agent-skill-test/internal/skill"
)

type MockRunner struct{}

func NewMockRunner() *MockRunner {
	return &MockRunner{}
}

func (r *MockRunner) Run(ctx context.Context, sk skill.Skill, sc scenario.Scenario, ws string) (*RunResult, error) {
	start := time.Now()

	output := fmt.Sprintf("[mock output for skill %q]\nInput: %s\nInstruction length: %d chars",
		sk.Name, sc.Input.UserPrompt, len(sk.Instructions))

	lowerInst := strings.ToLower(sk.Instructions)
	if strings.Contains(lowerInst, "react") {
		output += "\nReact component generated."
	}
	if strings.Contains(lowerInst, "css") {
		output += "\nStyles applied."
	}

	return &RunResult{
		ScenarioID: sc.ID,
		Output:     output,
		Duration:   time.Since(start),
	}, nil
}
