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
	if err := writeAnnotatedConfig(configFile, cfg); err != nil {
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

	if err := scaffoldExampleSkill("./skills/example-skill"); err != nil {
		return fmt.Errorf("scaffold example skill: %w", err)
	}

	fmt.Println("\nProject initialized.")
	fmt.Println("  Set ANTHROPIC_API_KEY, then try: ast test ./skills/example-skill")
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

	warnIfStubRunner(runnerName)
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

// warnIfStubRunner prints a loud warning when a non-agent runner is used.
// mock and sandbox are keyword-matching stubs — their pass/fail signal cannot
// be used to ship a skill.
func warnIfStubRunner(name string) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "mock", "sandbox":
		fmt.Fprintln(os.Stderr, "[WARN] ────────────────────────────────────────────────────────────")
		fmt.Fprintf(os.Stderr, "[WARN] Runner %q does NOT invoke a real agent.\n", name)
		fmt.Fprintln(os.Stderr, "[WARN] Pass/fail results CANNOT be used to validate skill compliance.")
		fmt.Fprintln(os.Stderr, "[WARN] Use --runner=api (or set default_runner: api in ast.yaml) for")
		fmt.Fprintln(os.Stderr, "[WARN] real behavior testing before treating any result as shippable.")
		fmt.Fprintln(os.Stderr, "[WARN] ────────────────────────────────────────────────────────────")
	}
}

func writeAnnotatedConfig(path string, cfg *config.Config) error {
	body := fmt.Sprintf(`project: %s
scenarios_dir: %s
reports_dir: %s

# Runner backend used by 'ast test'. Override per-invocation with --runner=NAME.
#   api      — real Claude agent (only mode that can validate a skill for release)
#   sandbox  — keyword-matching stub; framework smoke test only, NOT skill validation
#   mock     — fixed-output stub; framework smoke test only, NOT skill validation
default_runner: %s

api:
  key: %q            # leave empty to read ANTHROPIC_API_KEY
  model: %s
  endpoint: %s
  timeout: %d        # seconds per scenario
`,
		cfg.Project, cfg.ScenariosDir, cfg.ReportsDir,
		cfg.DefaultRunner,
		cfg.API.Key, cfg.API.Model, cfg.API.Endpoint, cfg.API.Timeout,
	)
	return os.WriteFile(path, []byte(body), 0o644)
}

func scaffoldExampleSkill(dir string) error {
	if _, err := os.Stat(dir); err == nil {
		return nil // don't clobber existing
	}
	files := map[string]string{
		"skill.yaml": `id: example-skill
name: "Example Skill"
description: "Minimal scaffold demonstrating ast's tools/ whitelist"
version: "0.1.0"
`,
		"instructions.md": `You are an assistant operating in an isolated workspace.

When the user asks you to inspect or modify files, use the provided
tools. Do not execute shell commands — this skill does not grant
run_command access.

When done, reply with a short summary.
`,
		"tools/read_file.json": `{"name": "read_file"}
`,
		"tools/edit_file.json": `{"name": "edit_file"}
`,
	}
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("Created %s\n", dir)
	return nil
}
