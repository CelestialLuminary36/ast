package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CelestialLuminary36/agent-skill-test/internal/config"
	"github.com/CelestialLuminary36/agent-skill-test/internal/judge"
	"github.com/CelestialLuminary36/agent-skill-test/internal/report"
	"github.com/CelestialLuminary36/agent-skill-test/internal/runner"
	"github.com/CelestialLuminary36/agent-skill-test/internal/scenario"
	"github.com/CelestialLuminary36/agent-skill-test/internal/skill"
	"github.com/CelestialLuminary36/agent-skill-test/internal/workspace"
)

const configFile = "ast.yaml"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		if err := cmdInit(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "init failed: %v\n", err)
			os.Exit(1)
		}
	case "test":
		if err := cmdTest(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "test failed: %v\n", err)
			os.Exit(1)
		}
	case "report":
		if err := cmdReport(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "report failed: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`ast - Agent Skill Tester

Usage:
  ast init                                Initialize a new project (ast.yaml + sample scenario)
  ast test <skill-dir> [--runner=NAME]    Run scenarios against a skill directory
                                          runner: mock | sandbox | api (default: ast.yaml default_runner)
  ast report <report.json>                Display a previously generated report

Environment:
  ANTHROPIC_API_KEY                       API key for the 'api' runner (overrides api.key in ast.yaml)`)
}

// ---------- init ----------

func cmdInit(_ []string) error {
	if _, err := os.Stat(configFile); err == nil {
		return fmt.Errorf("%s already exists", configFile)
	}

	cfg := config.Default()
	if err := cfg.Save(configFile); err != nil {
		return err
	}
	fmt.Printf("Created %s\n", configFile)

	scenariosDir := cfg.ScenariosDir
	if err := os.MkdirAll(scenariosDir, 0755); err != nil {
		return err
	}

	examplePath := filepath.Join(scenariosDir, "example.yaml")
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		example := `id: example
name: "示例场景"
description: "验证 Agent 是否能生成 React 按钮组件且不触碰禁止文件"
metadata:
  tags: [frontend, react]
  tier: smoke
environment:
  fixture_dir: ""
  init_script: ""
input:
  user_prompt: "Generate a simple React button component"
assert:
  file_mutations:
    allowed:
      - "src/**"
    forbidden:
      - "node_modules/**"
      - "package-lock.json"
  command_execution:
    must_have: []
    must_not_have:
      - contains: "rm -rf"
  output_text:
    must_include:
      - "React"
    must_not_include:
      - "Vue"
`
		if err := os.WriteFile(examplePath, []byte(example), 0644); err != nil {
			return err
		}
		fmt.Printf("Created %s\n", examplePath)
	}

	reportsDir := cfg.ReportsDir
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		return err
	}
	fmt.Printf("Created %s\n", reportsDir)
	fmt.Println("\nProject initialized. Edit ast.yaml and add scenarios to", scenariosDir)
	return nil
}

// ---------- test ----------

func cmdTest(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing skill directory\n\nUsage: ast test <skill-dir> [--runner=mock|sandbox|api]")
	}

	skillDir := args[0]
	runnerOverride := parseRunnerFlag(args[1:])

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	sk, err := skill.LoadFromDir(skillDir)
	if err != nil {
		return fmt.Errorf("load skill: %w", err)
	}

	scenarios, err := scenario.LoadFromDir(cfg.ScenariosDir)
	if err != nil {
		return fmt.Errorf("load scenarios: %w", err)
	}
	if len(scenarios) == 0 {
		return fmt.Errorf("no scenarios found in %s", cfg.ScenariosDir)
	}

	for i := range scenarios {
		if err := scenario.Validate(scenarios[i]); err != nil {
			return err
		}
	}

	runnerName := cfg.DefaultRunner
	if runnerOverride != "" {
		runnerName = runnerOverride
	}
	rnr, err := selectRunner(runnerName, cfg)
	if err != nil {
		return err
	}

	fmt.Printf("[INFO] Loaded Skill: %s\n", sk.Name)
	fmt.Printf("[INFO] Runner: %s\n", runnerName)
	fmt.Printf("[INFO] Found %d scenario(s) to run...\n\n", len(scenarios))

	jdg := judge.NewRuleJudge()

	rep := &report.Report{
		Project:   cfg.Project,
		SkillName: sk.Name,
		SkillPath: sk.Path,
		Timestamp: time.Now(),
	}

	ctx := context.Background()
	for _, sc := range scenarios {
		ws, err := workspace.New("")
		if err != nil {
			return fmt.Errorf("create workspace: %w", err)
		}

		fmt.Printf("运行场景: %s ...\n", sc.ID)
		result, runErr := rnr.Run(ctx, *sk, sc, ws.Root)
		ws.Cleanup()

		if runErr != nil {
			fmt.Printf("  └── [RESULT] RUN ERROR: %v\n\n", runErr)
			rep.AddEntry(report.Entry{
				ScenarioID: sc.ID,
				Passed:     false,
				Errors:     []string{fmt.Sprintf("run error: %v", runErr)},
			})
			continue
		}

		judgement, err := jdg.Judge(result, sc)
		if err != nil {
			fmt.Printf("  └── [RESULT] JUDGE ERROR: %v\n\n", err)
			rep.AddEntry(report.Entry{
				ScenarioID: sc.ID,
				Passed:     false,
				Output:     result.Output,
				Errors:     []string{fmt.Sprintf("judge error: %v", err)},
			})
			continue
		}

		if judgement.Passed {
			fmt.Printf("  └── [RESULT] PASSED\n\n")
		} else {
			fmt.Printf("  └── [RESULT] FAILED\n")
			for _, e := range judgement.Errors {
				fmt.Printf("      - %s\n", e)
			}
			fmt.Println()
		}

		rep.AddEntry(report.Entry{
			ScenarioID:   sc.ID,
			Passed:       judgement.Passed,
			Output:       result.Output,
			ExecutedCmds: result.ExecutedCmds,
			MutatedFiles: result.MutatedFiles,
			Errors:       judgement.Errors,
			Duration:     result.Duration,
		})
	}

	reportName := fmt.Sprintf("report-%s", time.Now().Format("20060102-150405"))
	jsonPath := filepath.Join(cfg.ReportsDir, reportName+".json")
	mdPath := filepath.Join(cfg.ReportsDir, reportName+".md")

	if err := report.Save(jsonPath, rep); err != nil {
		return fmt.Errorf("save json report: %w", err)
	}
	if err := rep.SaveMarkdown(mdPath); err != nil {
		return fmt.Errorf("save markdown report: %w", err)
	}

	fmt.Printf("Report saved to %s\n", jsonPath)
	fmt.Printf("Report saved to %s\n\n", mdPath)
	rep.PrintConsole()
	return nil
}

// ---------- report ----------

func cmdReport(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing report file\n\nUsage: ast report <report.json>")
	}

	path := args[0]
	rep, err := report.Load(path)
	if err != nil {
		return fmt.Errorf("load report: %w", err)
	}

	rep.PrintConsole()
	return nil
}

// ---------- helpers ----------

func loadConfig() (*config.Config, error) {
	cfg, err := config.Load(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s not found. Run 'ast init' first", configFile)
		}
		return nil, err
	}
	return cfg, nil
}

func parseRunnerFlag(args []string) string {
	for _, a := range args {
		if v, ok := strings.CutPrefix(a, "--runner="); ok {
			return strings.TrimSpace(v)
		}
		if a == "--runner" {
			// `--runner mock` form not supported here to keep parsing simple
			fmt.Fprintln(os.Stderr, "warning: use --runner=NAME form (e.g. --runner=api)")
		}
	}
	return ""
}

func selectRunner(name string, cfg *config.Config) (runner.Runner, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "mock":
		return runner.NewMockRunner(), nil
	case "sandbox":
		return runner.NewSandboxRunner(), nil
	case "api":
		return runner.NewAPIRunner(&cfg.API), nil
	default:
		return nil, fmt.Errorf("unknown runner %q (expected: mock | sandbox | api)", name)
	}
}
