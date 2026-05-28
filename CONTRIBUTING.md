# Contributing to ast

Thanks for taking the time. This document covers the practical things:
the layout, how to add the most common extension points, and the
conventions the maintainers expect to see in PRs.

If you're unsure whether a change fits, open a discussion or draft PR
first — it's cheaper than rewriting.

## Prerequisites

- Go **1.25+** (the toolchain pin in [go.mod](go.mod) is authoritative).
- `git` on `$PATH` — the runner does `git init` in each scenario workspace
  to capture file mutations.
- A Claude API key in `ANTHROPIC_API_KEY` (or another provider key) if
  you want to run the `api` runner end-to-end. The `mock` and `sandbox`
  runners need no key and are fine for unit-test-level work.

```bash
git clone https://github.com/hhy/ast.git
cd ast
go build ./...
go test ./...
```

All tests should pass on a fresh clone. CI runs the same suite on
ubuntu / macos / windows — if you can't reproduce a Windows failure
locally, push a draft PR and let the matrix do it.

## Repository layout

```
cmd/ast/                 CLI dispatcher + per-subcommand entry points
internal/skill/          Skill loaders (anthropic, cursor, agents-md, frontmatter)
internal/scenario/       Scenario YAML schema + loader
internal/runner/         Tool-use loop: api / mock / sandbox runners
internal/judge/          Rule evaluation against RunResult
internal/provider/       LLM backends (anthropic / openai / ollama)
internal/workspace/      Per-scenario temp dirs + git wrapping
internal/report/         JSON + Markdown + console report rendering
internal/config/         ast.yaml loader and defaults
scripts/                 e2e smoke tests (bash + powershell)
.github/workflows/       CI matrix + goreleaser tag-trigger
```

Anything under `internal/` is, by Go convention, only callable from
within this module. If you need to expose something publicly, lift it
to `pkg/` and explain why in the PR.

## Common extension points

### Adding a new provider

Implement the `provider.Provider` interface from
[internal/provider/provider.go](internal/provider/provider.go):

```go
type Provider interface {
    Send(ctx context.Context, req Request) (*Response, error)
}
```

Steps:

1. Create `internal/provider/<name>.go` with `New<Name>(cfg config.ProviderConfig) (*<Name>Provider, error)`.
2. Wire the constructor into `newProviderFromConfig` in
   [cmd/ast/main.go](cmd/ast/main.go) so users can select it via
   `provider.type: <name>` in `ast.yaml`.
3. Add an env-var fallback for the API key (look at how `NewAnthropic`
   reads `ANTHROPIC_API_KEY`).
4. Translate the canonical `Request` / `Response` types — particularly
   tool calls — to and from your backend's wire format. Tool-use loops
   in `internal/runner/api.go` will not work if you drop `ToolCall`
   blocks or stop reasons on the floor.
5. Write a test under `internal/provider/<name>_test.go`. At minimum
   exercise constructor errors (missing key, bad endpoint) and one
   round-trip with an `httptest.Server`.

### Adding a new scenario assertion

Assertions are evaluated against a `runner.RunResult`. The flow is:

1. Add a new field to `scenario.AssertConfig` in
   [internal/scenario/types.go](internal/scenario/types.go).
2. If your assertion needs new captured data, extend
   [internal/runner/types.go](internal/runner/types.go) `RunResult`
   and populate it in the runners that should support it.
3. Add the evaluation logic to
   [internal/judge/rule.go](internal/judge/rule.go) — see
   `auditFileContent` for a representative example.
4. Update [README.md](README.md) and `ast init`'s example scenario in
   [cmd/ast/main.go](cmd/ast/main.go) to document the new assertion.
5. Write table-driven tests in `internal/judge/rule_test.go`. Cover
   the success path, each failure path, and at least one edge case
   (e.g. invalid input, empty input).

Treat the YAML schema as a public contract: do not rename or remove
existing fields without a deprecation window.

### Adding a new skill format

Format detection lives in `detectFormat` in
[internal/skill/loader.go](internal/skill/loader.go). To add a new one:

1. Add a `Format` constant in
   [internal/skill/types.go](internal/skill/types.go).
2. Add a detector branch — most-specific first, so a directory that
   matches both your new format and `frontmatter` picks yours.
3. Implement `load<Format>(dir string) (*Skill, error)` that normalises
   into the existing `Skill` struct. Drop fields with no analogue
   (e.g. tool whitelists for formats that don't have one) rather than
   inventing semantics.
4. Add a warning branch in `validateSkill` (in
   [cmd/ast/main.go](cmd/ast/main.go)) if your format can't express
   tool isolation.
5. Add detection tests to
   [internal/skill/loader_test.go](internal/skill/loader_test.go).

## Code style

Standard `go fmt`. CI runs `gofmt -l` on linux and fails on any output,
so commit only formatted code. Run `go vet ./...` before pushing.

A few project-specific habits:

- **Comments explain WHY, not WHAT.** The reader can see what the code
  does. They cannot see why you wrote `Match` instead of `PathMatch`
  unless you tell them (see the comment on `matchGlob` in `judge/rule.go`).
- **Error messages name the file or operation that failed.** `read tools dir: <err>` beats `failed: <err>`.
- **No emoji in source or commit messages.** They render inconsistently
  in terminals and tooling.
- **Stable output.** The JSON report shape and console grep tokens
  like `[RESULT] <id> PASSED` are used by downstream automation. Don't
  reorder or rename them without thought.

## Testing conventions

- Tests live next to the code they exercise (`foo.go` → `foo_test.go`).
- Prefer table-driven tests for any function with more than one
  branch. Name each case so failures point at the scenario, not the
  index.
- Use `t.TempDir()` for filesystem fixtures — never `os.TempDir()` +
  manual cleanup. The harness wipes `t.TempDir()` for you and prevents
  accidental sharing between parallel tests.
- For provider-shaped tests, prefer `httptest.Server` over interface
  mocks. We want to validate the on-the-wire bytes, not a hand-rolled
  reimplementation of them.
- Cross-platform glob behaviour is load-bearing — if you touch
  `internal/judge/rule.go` `matchGlob`, add a case for both forward
  and backslash separators.

## Commit messages

Conventional Commits with a scope:

```
feat(cli): add 'ast list' to enumerate skills with health status
fix(judge): use doublestar.Match for cross-platform glob matching
docs(readme): add Install section
test(cli): cover parseRunnerFlag + warn on --scenarios space form
```

Scopes in use today: `cli`, `judge`, `skill`, `runner`, `provider`,
`scenario`, `report`, `workspace`, `config`, `ci`, `docs`, `style`,
`test`. Add a new scope only if none of those fit.

Body should explain the *why* — what bug, what user pain, what
constraint. The diff already shows the *what*.

## Pull requests

- Keep PRs scoped. A loader change and a CLI flag and a new provider
  do not belong in one PR. Reviewers will ask you to split.
- Update [TODO.md](TODO.md) if your change closes one of its items.
- Run the full test suite locally before pushing. CI is the safety net,
  not the first reader.
- If the PR adds or changes a user-visible flag, update the relevant
  help string in [cmd/ast/main.go](cmd/ast/main.go) **and** the
  matching section of [README.md](README.md). Out-of-sync docs are a
  bigger problem than missing ones.

## License

By contributing you agree your work will be licensed under Apache-2.0,
the same terms as the rest of the project. See [LICENSE](LICENSE) and
[NOTICE](NOTICE).
