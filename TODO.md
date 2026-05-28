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
- [ ] **`golangci-lint` config + lint job in CI.**
- [ ] **Backfill missing test packages.** `internal/runner/`,
  `internal/workspace/`, `internal/scenario/`, `internal/report/` are
  all `[no test files]` today.
- [ ] **`examples/` directory.** A handful of complete skills users
  can copy-paste. The current [skills/go-test-author/](skills/go-test-author/) is a good
  starting template.
- [ ] **Color output / TTY detection.** Optional `--no-color` flag.

---

## Release readiness — non-code (needs user action)

These steps need GitHub credentials and so cannot be done from inside
the local sandbox. Heads-up on current local state: `git remote -v`
still points `origin` at `git@github.com:CelestialLuminary36/agent-skill-test.git`,
the old name. That has to be retargeted before any push will reach the
right place.

- [ ] **Create the GitHub repo at `github.com/hhy/ast`.**
  Easiest via the web UI (Settings → New repository → `ast`, public,
  no README/license/.gitignore — we already have them). Or with `gh`
  installed:
  ```bash
  gh repo create hhy/ast --public --source=. --remote=origin --push
  ```
  If you used the web UI, retarget the existing remote instead of
  pushing to the old one:
  ```bash
  git remote set-url origin git@github.com:hhy/ast.git
  git push -u origin dev
  git push origin main
  ```
- [ ] **Merge `dev` → `main`.** Open a PR on GitHub (or `gh pr create
  --base main --head dev --title "v0.1.0 prep" --fill`), let CI go
  green, and merge.
- [ ] **Cut `v0.1.0`** from main:
  ```bash
  git checkout main
  git pull --ff-only
  git tag -a v0.1.0 -m "v0.1.0 — first credible cross-platform release"
  git push origin v0.1.0
  ```
  The release workflow in `.github/workflows/release.yml` picks up
  `v*` tags and runs goreleaser; archives land on the GitHub Releases
  page automatically.

---

## Conceptual gap — explicit decision still pending

- [ ] **What does "compatible with most agents" actually mean for this tool?**
  Two very different interpretations:
  - **(A) Loader-only** — read Claude Code / Cursor / Codex skill
    formats and run them all against ast's internal tool-use loop.
    Scoped as "Multi-format skill loader" above. Modest work.
  - **(B) Runner-level** — actually invoke the external agent CLIs
    (`claude-code`, `cursor-cli`, `codex`) as subprocesses, parse
    their output, intercept their tool calls. Effectively a whole new
    `internal/runner/external/` layer per agent. Larger than the
    entire current project.

  Recommendation: ship (A) in v0.1, defer (B) to a v0.2+ design doc.
