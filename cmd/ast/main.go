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
	"github.com/CelestialLuminary36/agent-skill-test/internal/provider"
	"github.com/CelestialLuminary36/agent-skill-test/internal/report"
	"github.com/CelestialLuminary36/agent-skill-test/internal/runner"
	"github.com/CelestialLuminary36/agent-skill-test/internal/scenario"
	"github.com/CelestialLuminary36/agent-skill-test/internal/skill"
	"github.com/CelestialLuminary36/agent-skill-test/internal/workspace"
)

const configFile = "ast.yaml"

// embeddedDefaultScenario is used when no scenario files are found.
const embeddedDefaultScenario = `id: default
name: "默认场景"
description: "验证 Agent 是否能根据用户提示创建文件"
environment:
  fixture_dir: ""
  init_script: ""
input:
  user_prompt: "Create a file named hello.txt with the content 'hello world'"
assert:
  file_mutations:
    allowed:
      - "hello.txt"
  command_execution:
    must_have: []
    must_not_have:
      - contains: "rm -rf"
  output_text:
    must_include:
      - "hello"
`

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
  ast init                         Generate ast.yaml (optional)
  ast test <skill-dir>             Run scenarios against a skill directory
                                   (looks for scenarios/ inside the skill dir first)
  ast report <report.json>         Display a previously generated report

Environment:
  ANTHROPIC_API_KEY                API key for the anthropic provider
  OPENAI_API_KEY                   API key for the openai provider
  OLLAMA_API_KEY                   API key for the ollama provider (optional, Ollama is local)`)
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
	fmt.Println("  Set ANTHROPIC_API_KEY (or configure another provider in ast.yaml), then try:")
	fmt.Println("  ast test ./skills/example-skill")
	return nil
}

// ---------- test ----------

func cmdTest(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing skill directory\n\nUsage: ast test <skill-dir>")
	}

	skillDir := args[0]

	cfg := loadConfigOrDefault()

	sk, err := skill.LoadFromDir(skillDir)
	if err != nil {
		return fmt.Errorf("load skill: %w", err)
	}

	// Default: look for scenarios/ inside the skill directory first,
	// fall back to the global scenarios_dir from config.
	scenariosDir := filepath.Join(skillDir, "scenarios")
	if override := parseScenariosFlag(args[1:]); override != "" {
		scenariosDir = override
	}
	scenarios := loadScenarios(scenariosDir)
	if len(scenarios) == 0 {
		// Fall back to global scenarios dir
		scenarios = loadScenarios(cfg.ScenariosDir)
	}

	for i := range scenarios {
		if err := scenario.Validate(scenarios[i]); err != nil {
			return err
		}
	}

	pc := cfg.ResolveProvider()
	p, err := newProviderFromConfig(pc)
	if err != nil {
		return err
	}
	rnr := runner.NewLLMRunner(p, pc)

	fmt.Printf("[INFO] Skill: %s\n", sk.Name)
	fmt.Printf("[INFO] Provider: %s (%s)\n", pc.Type, pc.Model)
	fmt.Printf("[INFO] %d scenario(s) to run...\n\n", len(scenarios))

	jdg := judge.NewRuleJudge()

	rep := &report.Report{
		Project:   cfg.Project,
		SkillName: sk.Name,
		SkillPath: sk.Path,
		Timestamp: time.Now(),
	}

	ctx := context.Background()
	for _, sc := range scenarios {
		fmt.Printf("\n=== 场景: %s ===\n", sc.ID)
		fmt.Printf("[STEP 1] 初始化 Git 隔离沙盒 ... ")
		ws, err := workspace.New("")
		if err != nil {
			fmt.Printf("FAILED: %v\n", err)
			rep.AddEntry(report.Entry{
				ScenarioID: sc.ID,
				Passed:     false,
				Errors:     []string{fmt.Sprintf("workspace init: %v", err)},
			})
			fmt.Printf("[RESULT] %s ERROR\n", sc.ID)
			continue
		}
		fmt.Println("SUCCESS")

		fmt.Printf("[STEP 2] 调用 Agent ... ")
		result, runErr := rnr.Run(ctx, *sk, sc, ws.Root)
		ws.Cleanup()
		if runErr != nil {
			fmt.Printf("FAILED: %v\n", runErr)
			rep.AddEntry(report.Entry{
				ScenarioID: sc.ID,
				Passed:     false,
				Errors:     []string{fmt.Sprintf("run error: %v", runErr)},
			})
			fmt.Printf("[RESULT] %s ERROR\n", sc.ID)
			continue
		}
		fmt.Printf("SUCCESS (耗时 %s)\n", result.Duration.Round(time.Millisecond))

		fmt.Printf("[STEP 3] 规则审计 ... ")
		judgement, err := jdg.Judge(result, sc)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			rep.AddEntry(report.Entry{
				ScenarioID: sc.ID,
				Passed:     false,
				Output:     result.Output,
				Errors:     []string{fmt.Sprintf("judge error: %v", err)},
			})
			fmt.Printf("[RESULT] %s ERROR\n", sc.ID)
			continue
		}
		if judgement.Passed {
			fmt.Println("PASSED")
			fmt.Printf("[RESULT] %s PASSED\n", sc.ID)
		} else {
			fmt.Printf("FAILED (%d 条规则未通过)\n", len(judgement.Errors))
			for _, e := range judgement.Errors {
				fmt.Printf("    - %s\n", e)
			}
			fmt.Printf("[RESULT] %s FAILED\n", sc.ID)
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

	fmt.Println("\n[STEP 4] 生成测试报告 ...")

	reportsDir := cfg.ReportsDir
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		return fmt.Errorf("create reports dir: %w", err)
	}

	reportName := fmt.Sprintf("report-%s", time.Now().Format("20060102-150405"))
	jsonPath := filepath.Join(reportsDir, reportName+".json")
	mdPath := filepath.Join(reportsDir, reportName+".md")

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

func loadConfigOrDefault() *config.Config {
	cfg, err := config.Load(configFile)
	if err != nil {
		return config.Default()
	}
	return cfg
}

func loadScenarios(path string) []scenario.Scenario {
	info, err := os.Stat(path)
	if err != nil {
		return defaultScenario()
	}
	if info.IsDir() {
		scenarios, err := scenario.LoadFromDir(path)
		if err != nil || len(scenarios) == 0 {
			return defaultScenario()
		}
		return scenarios
	}
	// Single file
	sc, err := scenario.LoadFromFile(path)
	if err != nil {
		return defaultScenario()
	}
	return []scenario.Scenario{sc}
}

func defaultScenario() []scenario.Scenario {
	sc, err := scenario.Parse(strings.NewReader(embeddedDefaultScenario))
	if err != nil {
		return nil
	}
	return []scenario.Scenario{sc}
}

func parseScenariosFlag(args []string) string {
	for _, a := range args {
		if v, ok := strings.CutPrefix(a, "--scenarios="); ok {
			return strings.TrimSpace(v)
		}
		if a == "--scenarios" {
			fmt.Fprintln(os.Stderr, "warning: use --scenarios=DIR form (e.g. --scenarios=./scenarios)")
		}
	}
	return ""
}

func newProviderFromConfig(cfg config.ProviderConfig) (provider.Provider, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Type)) {
	case "anthropic":
		return provider.NewAnthropic(cfg)
	case "openai":
		return provider.NewOpenAI(cfg)
	case "ollama":
		return provider.NewOllama(cfg)
	default:
		return nil, fmt.Errorf("unknown provider type %q (expected: anthropic | openai | ollama)", cfg.Type)
	}
}

func writeAnnotatedConfig(path string, cfg *config.Config) error {
	pc := cfg.ResolveProvider()
	body := fmt.Sprintf(`project: %s
scenarios_dir: %s
reports_dir: %s

# LLM provider backend.
provider:
  type: %s            # anthropic | openai | ollama
  key: %q             # leave empty to read env var (ANTHROPIC_API_KEY / OPENAI_API_KEY)
  model: %s
  endpoint: %s
  timeout: %d         # seconds per scenario
`,
		cfg.Project, cfg.ScenariosDir, cfg.ReportsDir,
		pc.Type, pc.Key, pc.Model, pc.Endpoint, pc.Timeout,
	)
	return os.WriteFile(path, []byte(body), 0o644)
}

func scaffoldExampleSkill(dir string) error {
	if _, err := os.Stat(dir); err == nil {
		return nil
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
