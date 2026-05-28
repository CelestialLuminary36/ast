# TODO — ast (Agent Skill Tester)

Roadmap to first credible open-source cross-platform release (`v0.1.0`).
Tasks are grouped by priority. Each item lists the concrete files to
touch so later sessions can pick up cold.

---

## P0 — Ship-blockers ✅ all done

- [x] **Fix broken glob matcher.** `internal/judge/rule.go` — switched
  `doublestar.PathMatch` to `doublestar.Match` (commit `3dad9b1`).
- [x] **Add LICENSE.** Apache-2.0 + NOTICE at repo root (commit `46ca622`).
- [x] **Wire up CI on GitHub Actions.** `.github/workflows/ci.yml` —
  ubuntu/macos/windows matrix, plus `.gitattributes` to normalise line
  endings (commit `b0e60a7`).

---

## P1 — Required for v0.1.0

### Release plumbing ✅ all done

- [x] **Rename Go module** to `github.com/hhy/ast` (commit `fa839ca`).
- [x] **`ast --version` flag** with ldflags wiring (commit `5285505`).
- [x] **Release pipeline.** `.goreleaser.yaml` + `.github/workflows/release.yml`
  triggered on `v*` tags, linux/macos/windows × amd64/arm64 (commit `5d0f3d8`).
- [x] **README install section.** Three install paths: binary release,
  `go install`, source (commit `7bffb1e`).

### Functional gaps surfaced during development ✅ all done

- [x] **Multi-format skill loader (the "compatible with most agents" pillar).**
  `internal/skill/loader.go` now detects: anthropic (skill.yaml + tools/),
  cursor (`.cursorrules` / `.cursor/rules/*.mdc`), agents-md (`AGENTS.md`
  / `.agents/*.md`), and a frontmatter fallback. Format is stored on the
  `Skill` struct and surfaced in `ast validate` warnings (commit `3393af7`).

- [x] **`ast gen <skill-dir>` — LLM-generated scenario drafts.**
  `cmd/ast/gen.go` asks the configured provider to draft N scenarios
  (default 3, --count 1..10), stamps `metadata.generated=true` + model
  id + timestamp, writes them under `<scenarios_dir>/<skill-id>/gen-*.yaml`
  with a header warning that a model satisfying its own test is not proof
  of compliance (commit `d1d85d3`).

- [x] **`file_content` assertion in scenarios.**
  `internal/scenario/types.go` `FileContentAssert` + `internal/judge/rule.go`
  `auditFileContent`. Runner captures contents of mutated files so the
  judge can match regex against post-run state. Supports `match_all_files`
  for "every match must pass" vs default "at least one match must pass"
  (commit `d0444ee`).

---

## P2 — Polish (post-v0.1)

- [x] **Per-subcommand `--help`.** `init`, `validate`, `gen`, `test`, `report`, `list`
  all accept `--help` / `-h` / `help`. Top-level `--help` already worked.
- [x] **`ast list`.** Enumerates skills under `./skills` (override with
  `--dir=DIR`) and prints id, format, scenario count, and status
  (OK / WARN / ERROR). Failed loads surface as ERROR rather than being
  silently skipped, so half-finished directories stay visible.
- [x] **`CONTRIBUTING.md`.** Covers prerequisites, repository layout,
  extension recipes (new provider / new assertion / new skill format),
  code style, test conventions, commit/PR conventions, and license.
- [x] **`golangci-lint` config + lint job in CI.** `.golangci.yml` with
  errcheck, gosimple, govet, ineffassign, staticcheck, unused, gofmt,
  goimports, misspell, unconvert, unparam, prealloc. Lint job runs on
  ubuntu-latest before the test matrix.
- [x] **Backfill missing test packages.** All four previously
  `[no test files]` packages now have coverage: `internal/runner/`
  (ToolExecutor, isBlocked, buildSystemPrompt, helpers), `internal/workspace/`
  (New, Path, Cleanup), `internal/scenario/` (Validate, Parse, LoadFromDir,
  LoadFromFile), `internal/report/` (AddEntry, Save/Load round-trip,
  SaveMarkdown). 19 new test functions total.
- [x] **`examples/` directory.** Two copy-paste ready skills under
  `examples/`: `go-nil-fixer` (full Anthropic format, 3 tools) and
  `read-only-reviewer` (2-tool whitelist, no run_command). Each has
  a README, skill.yaml, instructions.md, tools/, and a corresponding
  scenario under `scenarios/`.
- [x] **Color output / TTY detection.** `internal/color/color.go` provides
  ANSI escape wrappers (Green, Red, Yellow, Cyan, Bold) with auto-detection
  via NO_COLOR / FORCE_COLOR / TERM conventions. `--no-color` flag forces
  plain output. Applied to: `ast test` (steps, PASSED/FAILED, SUCCESS),
  `ast validate` (OK/WARN/FAIL), `ast list` (table headers, status),
  `ast report` (PASSED/FAILED in console view).

---

## Release readiness — done

- [x] **Create the GitHub repo** — `github.com:CelestialLuminary36/ast`
- [x] **Retarget remote.** `git remote set-url origin git@github.com:CelestialLuminary36/ast.git`
- [x] **Push branches.** `dev` and `main` pushed; `dev` (19 commits) fast-forwarded into `main`.
- [x] **Cut `v0.1.0`.** Tag pushed; release workflow in `.github/workflows/release.yml`
  picks up `v*` tags and runs goreleaser — archives land on GitHub Releases
  automatically once CI completes.

---

## Conceptual gap — resolved

- [x] **What does "compatible with most agents" actually mean for this tool?**
  Resolution: shipped (A) in v0.1 via multi-format loader (commit `3393af7`).
  Four formats supported: Anthropic (skill.yaml + tools/), Cursor
  (.cursorrules / .cursor/rules/*.mdc), AGENTS.md (.agents/*.md),
  and a frontmatter fallback. (B) — external runner-level invocation of
  agent CLIs — is deferred to v0.2+. The March 2026 skill-creator evals
  update covers much of the use case (B) would target, making it less
  urgent than it appeared when this was drafted. If user demand emerges,
  the right architecture is probably the opposite direction: let users
  POST execution traces to ast's judge, not have ast drive their agent.
  See the `ast judge` discussion elsewhere.
