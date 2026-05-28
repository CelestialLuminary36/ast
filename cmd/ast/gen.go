package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hhy/ast/internal/provider"
	"github.com/hhy/ast/internal/scenario"
	"github.com/hhy/ast/internal/skill"
	"gopkg.in/yaml.v3"
)

const genPromptTemplate = `You are generating regression-test scenarios for an "agent skill" — a
prompt + tool whitelist + instructions package that an LLM agent loads
to perform a specific job.

The user has authored the skill below. Your job: produce 3 scenarios
that exercise it from different angles. Each scenario is a YAML
document the test harness will run by spawning an agent in a temp git
workspace, feeding it the user_prompt, and judging the resulting file
mutations / commands / output.

# Scenario schema (the only valid output format)

id: <kebab-case unique within this skill>
name: "<short human-readable>"
description: >
  <2-3 sentence rationale for the scenario>
metadata:
  tags: [<tag>, ...]
  tier: smoke | full
environment:
  fixture_dir: ""          # leave empty unless the skill needs a starter repo
  init_script: ""
input:
  user_prompt: |
    <multi-line user message to the agent>
assert:
  file_mutations:
    allowed:   ["<glob>", ...]   # mutated files must match one of these (omit to allow anything)
    forbidden: ["<glob>", ...]   # mutated files must NOT match any of these
  command_execution:
    must_have:
      - {contains: "<substring>", min_count: 1}
    must_not_have:
      - {contains: "<substring>"}
  output_text:
    must_include: ["<substring>", ...]
    must_not_include: ["<substring>", ...]
  file_content:
    - glob: "<glob>"
      must_match:     ["<regex>", ...]
      must_not_match: ["<regex>", ...]

# Coverage rules

- Scenario 1: happy-path. The skill is asked to do the obvious thing
  it advertises. Assertions verify the success markers.
- Scenario 2: error-path or constraint. Probe a discipline the skill's
  instructions claim (e.g. "must not modify go.mod", "must use
  errors.Is"). Use file_content for code-level regex when output_text
  is too coarse.
- Scenario 3: adversarial or boundary. An input the skill might be
  tempted to over-reach on (refactor when only asked to test, fix a
  bug when asked to pin it, etc.). The assertions should catch the
  over-reach.

# Output format (strict — anything else is dropped)

Return exactly 3 YAML documents, each fenced like:

` + "```yaml" + `
id: ...
...
` + "```" + `

No prose between or around them. No commentary. Just three fenced
yaml blocks.

# The skill under test

Skill ID:   %s
Skill name: %s
Format:     %s
Tools available to the agent: %s

--- BEGIN INSTRUCTIONS ---
%s
--- END INSTRUCTIONS ---
`

func cmdGen(args []string) error {
	if wantsHelp(args) {
		fmt.Println(helpGen)
		return nil
	}
	if len(args) < 1 {
		return fmt.Errorf("missing skill directory\n\nUsage: ast gen <skill-dir> [--out=DIR] [--count=N]")
	}
	skillDir := args[0]
	outDir := ""
	count := 3
	for _, a := range args[1:] {
		switch {
		case strings.HasPrefix(a, "--out="):
			outDir = strings.TrimPrefix(a, "--out=")
		case strings.HasPrefix(a, "--count="):
			fmt.Sscanf(strings.TrimPrefix(a, "--count="), "%d", &count)
		}
	}
	if count < 1 || count > 10 {
		return fmt.Errorf("--count must be between 1 and 10, got %d", count)
	}

	sk, err := skill.LoadFromDir(skillDir)
	if err != nil {
		return fmt.Errorf("load skill: %w", err)
	}

	cfg := loadConfigOrDefault()
	if outDir == "" {
		outDir = filepath.Join(cfg.ScenariosDir, sk.ID)
	}

	prov, err := newProviderFromConfig(cfg.ResolveProvider())
	if err != nil {
		return fmt.Errorf("provider: %w", err)
	}

	toolNames := "all builtins (read_file, edit_file, run_command, list_files)"
	if len(sk.ToolDefs) > 0 {
		var names []string
		for _, td := range sk.ToolDefs {
			names = append(names, td.Name)
		}
		toolNames = strings.Join(names, ", ")
	}

	prompt := fmt.Sprintf(genPromptTemplate, sk.ID, sk.Name, sk.Format, toolNames, sk.Instructions)
	if count != 3 {
		// Mention the override but don't rewrite the whole template — the
		// model handles "give me 5 instead of 3" cleanly via a trailing note.
		prompt += fmt.Sprintf("\n\n(Generate exactly %d scenarios, not 3.)\n", count)
	}

	fmt.Printf("Generating %d scenarios for skill %q (model: %s) ...\n", count, sk.ID, cfg.ResolveProvider().Model)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.ResolveProvider().Timeout)*time.Second)
	defer cancel()

	resp, err := prov.Send(ctx, provider.Request{
		Messages: []provider.Message{{
			Role:    "user",
			Content: []provider.ContentBlock{{Type: "text", Text: prompt}},
		}},
		MaxTokens: 8000,
	})
	if err != nil {
		return fmt.Errorf("provider call: %w", err)
	}

	rawText := strings.Join(resp.TextBlocks, "\n")
	blocks := extractYAMLBlocks(rawText)
	if len(blocks) == 0 {
		return fmt.Errorf("model returned no fenced ```yaml blocks. raw output:\n%s", rawText)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create out dir: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	modelID := cfg.ResolveProvider().Model
	written := 0
	for i, block := range blocks {
		sc := scenario.Scenario{}
		if err := yaml.Unmarshal([]byte(block), &sc); err != nil {
			fmt.Fprintf(os.Stderr, "  block %d: parse failed, skipping: %v\n", i+1, err)
			continue
		}
		if sc.ID == "" {
			sc.ID = fmt.Sprintf("generated-%d", i+1)
		}
		// Stamp provenance so reports can distinguish self-tests from
		// human-authored ones, and so the file is self-describing.
		sc.Metadata.Generated = true
		sc.Metadata.GeneratedAt = now
		sc.Metadata.GeneratedBy = modelID

		out, err := yaml.Marshal(&sc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  block %d: marshal failed, skipping: %v\n", i+1, err)
			continue
		}
		header := fmt.Sprintf("# Generated by ast gen on %s using %s\n# Review before relying on the result — a model satisfying its own test is not proof.\n\n", now, modelID)
		path := filepath.Join(outDir, fmt.Sprintf("gen-%s.yaml", sc.ID))
		if err := os.WriteFile(path, append([]byte(header), out...), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		fmt.Printf("  wrote %s\n", path)
		written++
	}

	if written == 0 {
		return fmt.Errorf("all %d returned blocks failed to parse — see warnings above", len(blocks))
	}
	fmt.Printf("\nDone: %d scenario(s) written to %s\n", written, outDir)
	fmt.Println("NOTE: generated scenarios are a starting point, not a substitute for hand-authored coverage. Review before committing.")
	return nil
}

// extractYAMLBlocks pulls out every ```yaml ... ``` fenced block from text.
// Line-based rather than regex-based — a regex with lazy quantifiers
// over-matches when adjacent fences exist (empty body between two fences
// would capture the inner closing fence as content). Accepts ```yml as
// an alias and tolerates whitespace after the language tag.
func extractYAMLBlocks(s string) []string {
	out := make([]string, 0)
	lines := strings.Split(s, "\n")
	inBlock := false
	var current []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inBlock {
			if trimmed == "```yaml" || trimmed == "```yml" {
				inBlock = true
				current = current[:0]
			}
			continue
		}
		if trimmed == "```" {
			body := strings.TrimSpace(strings.Join(current, "\n"))
			if body != "" {
				out = append(out, body)
			}
			inBlock = false
			current = nil
			continue
		}
		current = append(current, line)
	}
	return out
}
