package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/CelestialLuminary36/agent-skill-test/internal/scenario"
	"github.com/CelestialLuminary36/agent-skill-test/internal/skill"
)

type SandboxRunner struct{}

func NewSandboxRunner() *SandboxRunner {
	return &SandboxRunner{}
}

func (r *SandboxRunner) Run(ctx context.Context, sk skill.Skill, sc scenario.Scenario, ws string) (*RunResult, error) {
	start := time.Now()

	// 1. Copy fixture into workspace if specified
	if sc.Environment.FixtureDir != "" {
		if err := copyDir(sc.Environment.FixtureDir, ws); err != nil {
			return nil, fmt.Errorf("copy fixture: %w", err)
		}
	}

	// 2. Run init script if specified
	var cmds []string
	if sc.Environment.InitScript != "" {
		cmd := exec.CommandContext(ctx, "sh", "-c", sc.Environment.InitScript)
		cmd.Dir = ws
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("init script failed: %w\noutput: %s", err, out)
		}
		// (no append to cmds — init_script is framework setup, not agent behavior)
	}

	// 3. Ensure git repo for diff tracking
	_ = runGit(ctx, ws, "init")
	_ = runGit(ctx, ws, "add", ".")
	_ = runGit(ctx, ws, "commit", "-m", "init", "--allow-empty")

	// 4. Simulate agent execution based on skill + input
	output := simulateAgent(sk, sc)

	// Simulate some commands based on skill instructions
	lowerInst := strings.ToLower(sk.Instructions)
	if strings.Contains(lowerInst, "test") {
		cmds = append(cmds, "go test ./...")
	}
	if strings.Contains(lowerInst, "format") {
		cmds = append(cmds, "gofmt -w .")
	}
	if strings.Contains(lowerInst, "lint") {
		cmds = append(cmds, "golangci-lint run")
	}

	// 5. Capture mutated files via git diff
	mutatedFiles, _ := captureMutations(ctx, ws)

	return &RunResult{
		ScenarioID:   sc.ID,
		Output:       output,
		ExecutedCmds: cmds,
		MutatedFiles: mutatedFiles,
		Duration:     time.Since(start),
	}, nil
}

func simulateAgent(sk skill.Skill, sc scenario.Scenario) string {
	prompt := sc.Input.UserPrompt
	output := fmt.Sprintf("[Agent Output]\nPrompt: %s\nSkill: %s\n", prompt, sk.Name)

	lowerInst := strings.ToLower(sk.Instructions)
	if strings.Contains(lowerInst, "react") || strings.Contains(lowerInst, "frontend") || strings.Contains(lowerInst, "css") {
		output += "React component generated with proper styling.\n"
	}
	if strings.Contains(lowerInst, "test") {
		output += "已完成单元测试验证\n"
	}
	if strings.Contains(lowerInst, "panic") || strings.Contains(lowerInst, "fix") {
		output += "Fixed the panic by adding nil check.\n"
	}

	return output
}

func runGit(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	_, err := cmd.CombinedOutput()
	return err
}

func captureMutations(ctx context.Context, dir string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 3 {
			continue
		}
		// git status --porcelain format: XY filename or XY "filename with spaces"
		fname := strings.TrimSpace(line[2:])
		if fname != "" {
			files = append(files, fname)
		}
	}
	return files, nil
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				return err
			}
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				return err
			}
		}
	}
	return nil
}
