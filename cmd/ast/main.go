package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CelestialLuminary36/ast/internal/color"
	"github.com/CelestialLuminary36/ast/internal/config"
	"github.com/CelestialLuminary36/ast/internal/judge"
	"github.com/CelestialLuminary36/ast/internal/provider"
	"github.com/CelestialLuminary36/ast/internal/report"
	"github.com/CelestialLuminary36/ast/internal/runner"
	"github.com/CelestialLuminary36/ast/internal/scenario"
	"github.com/CelestialLuminary36/ast/internal/skill"
	"github.com/CelestialLuminary36/ast/internal/workspace"
)

const configFile = "ast.yaml"

// Build-time identity. Defaults make `go build` (no -ldflags) produce
// "dev" / "none" / "unknown"; goreleaser overrides all three with real
// values via -X flags in .goreleaser.yaml.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	// --no-color can appear anywhere in args; detect it early.
	for _, a := range os.Args {
		if a == "--no-color" {
			color.Enabled = false
			break
		}
	}

	switch os.Args[1] {
	case "--version", "-v", "version":
		fmt.Printf("ast %s (commit %s, built %s)\n", version, commit, date)
		return
	case "--help", "-h", "help":
		usage()
		return
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
	case "validate":
		if err := cmdValidate(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "validate failed: %v\n", err)
			os.Exit(1)
		}
	case "gen":
		if err := cmdGen(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "gen failed: %v\n", err)
			os.Exit(1)
		}
	case "list":
		if err := cmdList(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "list failed: %v\n", err)
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
  ast init                         Generate ast.yaml + ./scenarios/<example>/ + ./skills/example-skill/
  ast validate <skill-dir>         Lint a skill: structure, instructions, tools, scenarios
  ast gen <skill-dir> [--out=DIR] [--count=N]
                                   Ask the configured LLM to generate N (default 3) scenario
                                   drafts for the skill. Written to ./scenarios/<skill-id>/
                                   with metadata.generated=true for downstream visibility.
  ast list [--dir=DIR]             List skills under DIR (default ./skills) with id, format,
                                   scenario count, and a quick health status.
  ast test <skill-dir> [--scenarios=DIR]
                                   Run scenarios against a skill. Discovery order:
                                     1. --scenarios=DIR (explicit)
                                     2. <scenarios_dir>/<skill-id>/   (default)
                                     3. <scenarios_dir>/              (flat fallback)
                                     4. <skill-dir>/scenarios/        (deprecated, warns)
  ast report <report.json>         Display a previously generated report
  ast version                      Print the binary version and exit

Run 'ast <command> --help' for details on a specific command.

Environment:
  ANTHROPIC_API_KEY                API key for the anthropic provider
  OPENAI_API_KEY                   API key for the openai provider
  OLLAMA_API_KEY                   API key for the ollama provider (optional, Ollama is local)`)
}

// wantsHelp reports whether the user asked for help on a subcommand.
// We accept --help, -h, and the bare word help so users typing
// `ast gen help` get the same result as `ast gen --help`.
func wantsHelp(args []string) bool {
	for _, a := range args {
		switch a {
		case "--help", "-h", "help":
			return true
		}
	}
	return false
}

const helpInit = `ast init - Initialize a new ast project in the current directory.

Usage:
  ast init

Creates:
  ast.yaml                          Project config (provider, scenarios/reports dirs)
  ./scenarios/example-skill/        A starter scenario you can adapt
  ./skills/example-skill/           A minimal scaffold demonstrating skill.yaml + tools/
  ./reports/                        Empty directory for test reports

Refuses to overwrite an existing ast.yaml.`

const helpValidate = `ast validate - Lint a skill package.

Usage:
  ast validate <skill-dir>

Checks:
  - Skill format is detectable (anthropic | cursor | agents-md | frontmatter)
  - Instructions are non-empty
  - tools/*.json declarations are well-formed (anthropic format only)
  - scenarios/ subdir does not pollute the skill (warns if present)
  - <scenarios_dir>/<skill-id>/ exists and contains valid scenarios

Exits non-zero on the first error. Warnings do not affect exit code.`

const helpGen = `ast gen - Draft scenarios for a skill using the configured LLM.

Usage:
  ast gen <skill-dir> [--out=DIR] [--count=N]

Flags:
  --out=DIR        Override the output directory.
                   Default: <scenarios_dir>/<skill-id>/
  --count=N        Number of scenarios to draft (1-10). Default: 3.

Generated files are named gen-<id>.yaml and carry metadata.generated=true
plus the generating model id, so reports can distinguish self-generated
tests from human-authored ones. NOTE: a model satisfying its own generated
test is not proof of compliance — review before trusting the result.

Reads provider config from ast.yaml (or the env var for the provider).`

const helpTest = `ast test - Run scenarios against a skill and produce a report.

Usage:
  ast test <skill-dir> [--scenarios=DIR]

Flags:
  --scenarios=DIR  Use scenarios from DIR instead of the default discovery.

Discovery order (first match wins):
  1. --scenarios=DIR (explicit)
  2. <scenarios_dir>/<skill-id>/   (recommended layout)
  3. <scenarios_dir>/              (flat fallback)
  4. <skill-dir>/scenarios/        (deprecated; emits a stderr warning)

Each scenario runs in a fresh git-initialized temp workspace. The runner
captures the model's final output, every run_command invocation, and the
list + contents of mutated files; the judge evaluates these against the
scenario's assert block. Reports are written to <reports_dir>/ as both
JSON and Markdown, and a console summary is printed at the end.`

const helpReport = `ast report - Re-print a previously generated report.

Usage:
  ast report <report.json>

Reads the JSON report produced by 'ast test' and renders the same
console summary. Useful for sharing or revisiting older runs without
re-executing scenarios.`

// ---------- init ----------

func cmdInit(args []string) error {
	if wantsHelp(args) {
		fmt.Println(helpInit)
		return nil
	}
	if _, err := os.Stat(configFile); err == nil {
		return fmt.Errorf("%s already exists", configFile)
	}

	cfg := config.Default()
	if err := writeAnnotatedConfig(configFile, cfg); err != nil {
		return err
	}
	fmt.Printf("Created %s\n", configFile)

	// Per-skill layout: scenarios live at <scenarios_dir>/<skill-id>/, not nested in the skill.
	exampleDir := filepath.Join(cfg.ScenariosDir, "example-skill")
	if err := os.MkdirAll(exampleDir, 0755); err != nil {
		return err
	}

	examplePath := filepath.Join(exampleDir, "example.yaml")
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
	if wantsHelp(args) {
		fmt.Println(helpTest)
		return nil
	}
	if len(args) < 1 {
		return fmt.Errorf("missing skill directory\n\nUsage: ast test <skill-dir>")
	}

	skillDir := args[0]

	cfg := loadConfigOrDefault()

	sk, err := skill.LoadFromDir(skillDir)
	if err != nil {
		return fmt.Errorf("load skill: %w", err)
	}

	scenarios, src := resolveScenarios(skillDir, sk.ID, cfg.ScenariosDir, parseScenariosFlag(args[1:]))
	if src == sourceNotFound {
		return fmt.Errorf("no scenarios found for skill %q.\n  Tried: %s\n  Create one at %s/<name>.yaml, or pass --scenarios=DIR.",
			sk.ID,
			strings.Join(triedPaths(skillDir, sk.ID, cfg.ScenariosDir), ", "),
			filepath.Join(cfg.ScenariosDir, sk.ID),
		)
	}
	if src == sourceNestedDeprecated {
		fmt.Fprintf(os.Stderr, "warning: loaded scenarios from %s (nested inside skill). This layout is deprecated — move them to %s so the skill stays portable.\n",
			filepath.Join(skillDir, "scenarios"),
			filepath.Join(cfg.ScenariosDir, sk.ID),
		)
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

	fmt.Printf("%s Skill: %s\n", color.Cyan("[INFO]"), sk.Name)
	fmt.Printf("%s Provider: %s (%s)\n", color.Cyan("[INFO]"), pc.Type, pc.Model)
	fmt.Printf("%s %d scenario(s) to run...\n\n", color.Cyan("[INFO]"), len(scenarios))

	jdg := judge.NewRuleJudge()

	rep := &report.Report{
		Project:   cfg.Project,
		SkillName: sk.Name,
		SkillPath: sk.Path,
		Timestamp: time.Now(),
	}

	ctx := context.Background()
	for _, sc := range scenarios {
		fmt.Printf("\n%s %s\n", color.Bold("==="), color.Cyan(sc.ID))
		fmt.Printf("%s 初始化 Git 隔离沙盒 ... ", color.Cyan("[STEP 1]"))
		ws, err := workspace.New("")
		if err != nil {
			fmt.Printf("%s: %v\n", color.Red("FAILED"), err)
			rep.AddEntry(report.Entry{
				ScenarioID: sc.ID,
				Passed:     false,
				Errors:     []string{fmt.Sprintf("workspace init: %v", err)},
			})
			fmt.Printf("%s %s %s\n", color.Cyan("[RESULT]"), sc.ID, color.Red("ERROR"))
			continue
		}
		fmt.Println(color.Green("SUCCESS"))

		fmt.Printf("%s 调用 Agent ... ", color.Cyan("[STEP 2]"))
		result, runErr := rnr.Run(ctx, *sk, sc, ws.Root)
		ws.Cleanup()
		if runErr != nil {
			fmt.Printf("%s: %v\n", color.Red("FAILED"), runErr)
			rep.AddEntry(report.Entry{
				ScenarioID: sc.ID,
				Passed:     false,
				Errors:     []string{fmt.Sprintf("run error: %v", runErr)},
			})
			fmt.Printf("%s %s %s\n", color.Cyan("[RESULT]"), sc.ID, color.Red("ERROR"))
			continue
		}
		fmt.Printf("%s (耗时 %s)\n", color.Green("SUCCESS"), result.Duration.Round(time.Millisecond))

		fmt.Printf("%s 规则审计 ... ", color.Cyan("[STEP 3]"))
		judgement, err := jdg.Judge(result, sc)
		if err != nil {
			fmt.Printf("%s: %v\n", color.Red("ERROR"), err)
			rep.AddEntry(report.Entry{
				ScenarioID: sc.ID,
				Passed:     false,
				Output:     result.Output,
				Errors:     []string{fmt.Sprintf("judge error: %v", err)},
			})
			fmt.Printf("%s %s %s\n", color.Cyan("[RESULT]"), sc.ID, color.Red("ERROR"))
			continue
		}
		if judgement.Passed {
			fmt.Println(color.Green("PASSED"))
			fmt.Printf("%s %s %s\n", color.Cyan("[RESULT]"), sc.ID, color.Green("PASSED"))
		} else {
			fmt.Printf("%s (%d 条规则未通过)\n", color.Red("FAILED"), len(judgement.Errors))
			for _, e := range judgement.Errors {
				fmt.Printf("    - %s\n", color.Red(e))
			}
			fmt.Printf("%s %s %s\n", color.Cyan("[RESULT]"), sc.ID, color.Red("FAILED"))
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

// ---------- validate ----------

func cmdValidate(args []string) error {
	if wantsHelp(args) {
		fmt.Println(helpValidate)
		return nil
	}
	if len(args) < 1 {
		return fmt.Errorf("missing skill directory\n\nUsage: ast validate <skill-dir>")
	}
	skillDir := args[0]
	cfg := loadConfigOrDefault()

	issues, warns := validateSkill(skillDir, cfg.ScenariosDir)

	fmt.Printf("Validating skill: %s\n", skillDir)
	// Best-effort: re-load to print the detected format. Cheap, and avoids
	// changing validateSkill's signature just to surface this info.
	if sk, err := skill.LoadFromDir(skillDir); err == nil {
		fmt.Printf("  %s detected format: %s\n", color.Cyan("[INFO]"), sk.Format)
	}
	for _, w := range warns {
		fmt.Printf("  %s %s\n", color.Yellow("[WARN]"), w)
	}
	for _, e := range issues {
		fmt.Printf("  %s %s\n", color.Red("[FAIL]"), e)
	}
	if len(issues) == 0 {
		if len(warns) == 0 {
			fmt.Printf("  %s\n", color.Green("OK"))
		} else {
			fmt.Printf("  %s (%d warning(s))\n", color.Green("OK"), len(warns))
		}
		return nil
	}
	return fmt.Errorf("%d issue(s) found", len(issues))
}

// validateSkill is the testable core of `ast validate`. Returns (errors, warnings).
func validateSkill(skillDir, scenariosDir string) (issues, warns []string) {
	info, err := os.Stat(skillDir)
	if err != nil {
		return []string{fmt.Sprintf("skill dir not accessible: %v", err)}, nil
	}
	if !info.IsDir() {
		return []string{fmt.Sprintf("%s is not a directory", skillDir)}, nil
	}

	sk, err := skill.LoadFromDir(skillDir)
	if err != nil {
		issues = append(issues, fmt.Sprintf("load skill: %v", err))
		return issues, warns
	}
	if sk.Instructions == "" {
		issues = append(issues, "no instructions found (expected one of: instructions.md, skill.md, README.md, instruction.md, instruction.txt, AGENTS.md, .cursorrules, .cursor/rules/*.mdc)")
	} else if len(strings.TrimSpace(sk.Instructions)) < 16 {
		warns = append(warns, "instructions are very short — agents may not have enough context to follow them")
	}
	// Format-specific advice. Non-Anthropic formats cannot express a tool
	// whitelist, so the runner exposes all builtins — flag this so users
	// know they aren't getting the same isolation they'd get from skill.yaml.
	switch sk.Format {
	case skill.FormatAnthropic:
		if _, err := os.Stat(filepath.Join(skillDir, "skill.yaml")); os.IsNotExist(err) {
			warns = append(warns, "no skill.yaml — metadata (id, name, version) will be derived from the directory name")
		}
	case skill.FormatCursorRules, skill.FormatAgentsMD, skill.FormatFrontmatter:
		warns = append(warns, fmt.Sprintf("%s format has no tool whitelist — runner will expose all builtins. For tool isolation, port to the Anthropic package format (skill.yaml + tools/).", sk.Format))
	}
	for _, td := range sk.ToolDefs {
		if td.Name == "" {
			issues = append(issues, "tools/: a definition is missing required 'name' field")
		}
	}

	perSkillDir := filepath.Join(scenariosDir, sk.ID)

	// Pollution check: scenarios should not live inside the skill package.
	nested := filepath.Join(skillDir, "scenarios")
	if ni, err := os.Stat(nested); err == nil && ni.IsDir() {
		warns = append(warns, fmt.Sprintf("found %s — scenarios should live outside the skill (e.g. %s) so the skill stays portable across agents", nested, perSkillDir))
	}

	// If a per-skill scenarios dir exists, lint every scenario in it.
	if si, err := os.Stat(perSkillDir); err == nil && si.IsDir() {
		scs, err := scenario.LoadFromDir(perSkillDir)
		if err != nil {
			issues = append(issues, fmt.Sprintf("scenarios in %s: %v", perSkillDir, err))
		}
		for _, sc := range scs {
			if err := scenario.Validate(sc); err != nil {
				issues = append(issues, fmt.Sprintf("scenario %s: %v", sc.ID, err))
			}
		}
		if len(scs) == 0 {
			warns = append(warns, fmt.Sprintf("%s exists but contains no scenarios", perSkillDir))
		}
	} else {
		warns = append(warns, fmt.Sprintf("no scenarios at %s — `ast test` will have nothing to run", perSkillDir))
	}

	return issues, warns
}

// ---------- report ----------

func cmdReport(args []string) error {
	if wantsHelp(args) {
		fmt.Println(helpReport)
		return nil
	}
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

type scenarioSource int

const (
	sourceNotFound scenarioSource = iota
	sourceExplicit
	sourcePerSkill
	sourceFlatFallback
	sourceNestedDeprecated
)

// resolveScenarios walks the discovery order and returns the first hit.
// Order: explicit --scenarios → <scenarios_dir>/<skill-id>/ → <scenarios_dir>/
//        → <skill-dir>/scenarios/ (deprecated).
func resolveScenarios(skillDir, skillID, scenariosDir, explicit string) ([]scenario.Scenario, scenarioSource) {
	if explicit != "" {
		if scs := loadScenariosFromPath(explicit); len(scs) > 0 {
			return scs, sourceExplicit
		}
		return nil, sourceNotFound
	}
	perSkill := filepath.Join(scenariosDir, skillID)
	if scs := loadScenariosFromPath(perSkill); len(scs) > 0 {
		return scs, sourcePerSkill
	}
	if scs := loadScenariosFromPath(scenariosDir); len(scs) > 0 {
		return scs, sourceFlatFallback
	}
	nested := filepath.Join(skillDir, "scenarios")
	if scs := loadScenariosFromPath(nested); len(scs) > 0 {
		return scs, sourceNestedDeprecated
	}
	return nil, sourceNotFound
}

func triedPaths(skillDir, skillID, scenariosDir string) []string {
	return []string{
		filepath.Join(scenariosDir, skillID),
		scenariosDir,
		filepath.Join(skillDir, "scenarios"),
	}
}

func loadScenariosFromPath(path string) []scenario.Scenario {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	if info.IsDir() {
		scs, err := scenario.LoadFromDir(path)
		if err != nil {
			return nil
		}
		return scs
	}
	sc, err := scenario.LoadFromFile(path)
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
