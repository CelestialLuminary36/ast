# ast — Agent Skill Tester

[![License: Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](LICENSE)
[![CI](https://github.com/hhy/ast/actions/workflows/ci.yml/badge.svg)](https://github.com/hhy/ast/actions/workflows/ci.yml)

A CLI for running scenario-based regression tests against Claude **Skills**. Each scenario gives the agent a prompt, an isolated workspace, and a set of assertions; `ast` runs the skill, observes file mutations / executed commands / model output, and produces a pass/fail report.

## Install

Pick whichever fits your setup:

**Pre-built binary** — grab the archive for your OS/arch from the [latest release](https://github.com/hhy/ast/releases/latest), extract, and put `ast` (or `ast.exe`) on your `$PATH`.

**`go install`** — needs Go 1.25+:

```bash
go install github.com/hhy/ast/cmd/ast@latest
```

**From source**:

```bash
git clone https://github.com/hhy/ast.git
cd ast
go build -o ast ./cmd/ast        # produces ./ast (or ./ast.exe on Windows)
```

Verify with `ast version`.

## Quick start

```bash
ast init
ast test ./skills/my-skill
```

`init` creates `ast.yaml`, a sample scenario, and the `reports/` directory.

## Supported skill formats

`ast` detects the on-disk layout of `<skill-dir>` and normalises it
internally, so the same scenario suite can run against skills authored
for different agents.

| Layout                          | Format         | Notes                                             |
|---------------------------------|----------------|---------------------------------------------------|
| `skill.yaml` + `instructions.md` + `tools/` | `anthropic`    | Full fidelity — id/name/version + tool whitelist. |
| `.cursor/rules/*.mdc`           | `cursor`       | Cursor rule files with frontmatter; concatenated. |
| `.cursorrules` (file)           | `cursor`       | Legacy single-file Cursor format.                 |
| `AGENTS.md` or `.agents/*.md`   | `agents-md`    | Codex / generic convention. Optional frontmatter. |
| Any `.md` with YAML frontmatter | `frontmatter`  | Last-resort fallback for custom layouts.          |

Detection runs in the order shown — most-specific first. Only the
Anthropic format can declare a tool whitelist; for the others the runner
exposes all builtins and `ast validate` emits a warning.

## Runners

`ast` ships three runner backends. **Only `api` invokes a real agent — it is
the only mode whose pass/fail result can be used to validate a skill for
release.** Pick one via `default_runner` in `ast.yaml` or `--runner=NAME` on
the command line.

| Runner    | What it does                                                                  | Use for                          |
|-----------|-------------------------------------------------------------------------------|----------------------------------|
| `api`     | Real Claude API agent. Loads skill instructions into the system prompt and exposes the skill's declared tool whitelist (or all four builtins if none declared). The model drives `read_file` / `edit_file` / `run_command` / `list_files` inside a per-scenario workspace. | **Skill validation. Default.**   |
| `sandbox` | Keyword-matching stub; does not call any model. Prints a warning on use.      | Smoke-testing `ast` itself.      |
| `mock`    | Fixed-output stub; does not call any model. Prints a warning on use.          | Smoke-testing `ast` itself.      |

Selecting `mock` or `sandbox` prints a stderr warning that the result
**cannot** be used to validate skill compliance.

### Using the `api` runner

1. Get a Claude API key from <https://console.anthropic.com/>.
2. Set it via either:
   - `ANTHROPIC_API_KEY` environment variable (recommended), or
   - `api.key` in `ast.yaml`.
3. The default runner is already `api`, so:

```bash
export ANTHROPIC_API_KEY=sk-ant-...
./ast.exe test ./skills/my-skill
```

### Skill tool whitelist

A skill that declares a `tools/` directory restricts the agent to only those
tools. Each `tools/*.json` file is one Anthropic-format tool definition.

The simplest form references a builtin by name:

```json
{"name": "read_file"}
```

The fuller form declares a custom tool the model can call (note: ast has no
executor for custom tools; calls will return an "unknown tool" error — useful
when you want to detect that the agent reaches for a forbidden capability):

```json
{
  "name": "run_test",
  "description": "Run the project's test suite",
  "input_schema": {
    "type": "object",
    "properties": {"package": {"type": "string"}},
    "required": ["package"]
  }
}
```

If a skill has no `tools/` directory, all four builtins are exposed
(backwards-compatible default).

### `ast.yaml` reference

```yaml
project: agent-skill-test
scenarios_dir: ./scenarios
reports_dir: ./reports
default_runner: api           # api | sandbox | mock — only api validates skills
api:
  key: ""                     # leave empty to read ANTHROPIC_API_KEY
  model: claude-sonnet-4-6    # any messages-API model id
  endpoint: https://api.anthropic.com/v1/messages
  timeout: 120                # seconds per scenario
```

## How the API runner works

For each scenario the runner:

1. Copies `environment.fixture_dir` into a fresh temp workspace and runs `environment.init_script` (if set).
2. `git init` + commit, so file mutations can be diffed at the end.
3. Sends the skill's `instructions` as the system prompt and the scenario's `input.user_prompt` as the first user turn, along with tool definitions for `read_file`, `edit_file`, `run_command`, `list_files`.
4. Runs the standard tool-use loop: parse `tool_use` blocks → execute against the workspace → return `tool_result` → repeat until the model stops calling tools (`stop_reason: end_turn`) or the round limit (30) is hit.
5. Captures: model's final text output, every `run_command` invocation, `git status --porcelain` of mutated files.
6. The judge then evaluates these against the scenario's `assert` block.

`run_command` blocks a small list of obviously destructive commands (`rm -rf /`, fork bombs, `mkfs.`, …) — the workspace is still your safety net, not these patterns.

## Commands

```
ast init                                              Initialize project (ast.yaml + ./scenarios/example-skill/ + ./skills/example-skill/)
ast validate <skill-dir>                              Lint a skill (structure, instructions, tools, scenarios; warns on nested scenarios/)
ast test <skill-dir> [--runner=NAME] [--scenarios=DIR]
                                                      Run scenarios; flags override ast.yaml defaults
ast report <report.json>                              Re-print a previously generated report
```

## Project layout

Skill packages should remain **portable** — they get shared between agents
(Claude Code, Cursor, Codex, ...). Tests for a skill are not part of the
skill; they live next to it:

```
project/
├── ast.yaml
├── skills/
│   └── go-reviewer/             # the portable artifact
│       ├── skill.yaml
│       ├── instructions.md
│       └── tools/
└── scenarios/
    └── go-reviewer/             # tests, named after the skill's id
        └── nil-panic.yaml
```

`ast test ./skills/go-reviewer` discovers scenarios in this order:

1. `--scenarios=DIR` if you passed it
2. `./scenarios/<skill-id>/` &nbsp; ← recommended
3. `./scenarios/` &nbsp; (flat fallback)
4. `./skills/<skill>/scenarios/` &nbsp; ← **deprecated**, emits a stderr warning. This layout pollutes the skill package; move the files out.

If none of those exist, `ast test` aborts with a message telling you where
to put a scenario file. There is no longer a silent "default" scenario.
