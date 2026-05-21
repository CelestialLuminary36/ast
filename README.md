# ast — Agent Skill Tester

A CLI for running scenario-based regression tests against Claude **Skills**. Each scenario gives the agent a prompt, an isolated workspace, and a set of assertions; `ast` runs the skill, observes file mutations / executed commands / model output, and produces a pass/fail report.

## Quick start

```bash
go build -o ast.exe ./cmd/ast
./ast.exe init
./ast.exe test ./skills/my-skill
```

`init` creates `ast.yaml`, a sample scenario, and the `reports/` directory.

## Runners

`ast` supports three runner backends. Pick one via `default_runner` in `ast.yaml` or `--runner=NAME` on the command line.

| Runner   | What it does                                                                                  |
|----------|-----------------------------------------------------------------------------------------------|
| `mock`   | Deterministic stub. No model calls. Useful for harness smoke-tests.                           |
| `sandbox`| Simulates an agent locally by pattern-matching the skill instructions. No model calls.        |
| `api`    | Real Claude API agent. Loads skill instructions into the system prompt and lets the model drive `read_file` / `edit_file` / `run_command` / `list_files` tools inside a per-scenario workspace. |

### Using the `api` runner

1. Get a Claude API key from <https://console.anthropic.com/>.
2. Set it via either:
   - `ANTHROPIC_API_KEY` environment variable (recommended), or
   - `api.key` in `ast.yaml`.
3. Run with `--runner=api`:

```bash
export ANTHROPIC_API_KEY=sk-ant-...
./ast.exe test ./skills/my-skill --runner=api
```

### `ast.yaml` reference

```yaml
project: agent-skill-test
scenarios_dir: ./scenarios
reports_dir: ./reports
default_runner: mock          # mock | sandbox | api
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
ast init                                Initialize project (ast.yaml + sample scenario)
ast test <skill-dir> [--runner=NAME]    Run scenarios; runner overrides default_runner
ast report <report.json>                Re-print a previously generated report
```
