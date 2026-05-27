# TODO — ast (Agent Skill Tester)

Roadmap to first credible open-source cross-platform release (`v0.1.0`).
Tasks are grouped by priority. Check off as you go. Each item lists
the concrete files to touch so later sessions can pick up cold.

---

## P0 — Ship-blockers (must fix before any tag)

- [ ] **Fix broken glob matcher.**
  `TestMatchGlob_Doublestar` in [internal/judge/rule_test.go](internal/judge/rule_test.go) currently FAILs.
  Symptoms: `*.go` matches `src/main.go` (should not); `src/**/*.go`
  does not match `src/a.go` (should). The `bmatcuk/doublestar/v4`
  dependency is already in [go.mod](go.mod) — wire it into
  [internal/judge/rule.go](internal/judge/rule.go) `matchGlob` and
  delete the hand-rolled logic. Without this, `file_mutations`
  allow/forbid lists silently lie. Estimate: ~half day.

- [ ] **Add LICENSE file.**
  Without one, the project is legally not open source. Recommend
  Apache-2.0 or MIT at repo root.

- [ ] **Wire up CI on GitHub Actions.**
  `.github/workflows/ci.yml`: matrix over `ubuntu-latest`,
  `macos-latest`, `windows-latest`; runs `go test ./...` plus
  `./scripts/e2e-test.sh` / `e2e-test.ps1`. Without CI the
  "cross-platform" claim is unevidenced. Estimate: ~half day.

---

## P1 — Required for v0.1.0

### Release plumbing

- [ ] **Rename Go module.**
  Current path `github.com/CelestialLuminary36/agent-skill-test` is a
  placeholder. Decide the org/repo name (suggestion:
  `github.com/<org>/ast` or `github.com/<org>/skill-check`) and
  rewrite imports across all `.go` files plus [go.mod](go.mod).

- [ ] **`ast --version` flag.**
  Inject version via `-ldflags "-X main.version=…"` so release
  binaries self-identify. Touch [cmd/ast/main.go](cmd/ast/main.go).

- [ ] **Release pipeline.**
  `.github/workflows/release.yml` triggered on `v*` tags. Use
  `goreleaser` to produce binaries for linux/macos/windows ×
  amd64/arm64 + checksums. Attach to GitHub Release.

- [ ] **README install section.**
  Cover three install paths: `go install`, downloading a release
  binary, building from source. Currently [README.md](README.md)
  assumes `go build`.

### Functional gaps surfaced during development

- [ ] **Multi-format skill loader (the "compatible with most agents" pillar).**
  Today only Anthropic-style packages load. Add a format detector to
  [internal/skill/loader.go](internal/skill/loader.go) that recognizes:
  - Cursor: `.cursorrules` or `.cursor/rules/*.mdc`
  - Codex / generic: `AGENTS.md`, `.agents/*.md`
  - Fallback: any `.md` with YAML frontmatter
  Each detector normalizes into the existing `Skill` struct. Drop
  fields with no analogue (Cursor has no `tools/`) rather than
  inventing semantics.

- [ ] **`ast gen <skill-dir>` — LLM-generated scenario drafts.**
  Adoption-bar feature. Reads the skill's instructions, asks the
  configured provider to produce 2–3 scenario YAMLs, writes them to
  `./scenarios/<skill-id>/` with `metadata.generated: true`. Judge
  must report generated-scenario results separately so users do not
  mistake "agent passed its own generated test" for compliance.

- [ ] **`file_content` assertion in scenarios.**
  `output_text` substring matching is too brittle for code-level
  requirements (e.g. "the test must use `errors.Is`"). Today's
  scenario 02 had to drop the `errors.Is` assertion for this reason.
  Add a block to [internal/scenario/types.go](internal/scenario/types.go):
  ```yaml
  assert:
    file_content:
      - glob: "*_test.go"
        must_match:
          - "errors\\.Is\\(.*ErrEmptyInput\\)"
        must_not_match:
          - "testify"
  ```
  Implement in [internal/judge/rule.go](internal/judge/rule.go). Update
  the three `go-test-author` scenarios to use it once available.

---

## P2 — Polish (post-v0.1)

- [ ] **Per-subcommand `--help`.** Today only the top-level usage
  exists. Each of `init`, `validate`, `test`, `report` should respond
  to `--help`.
- [ ] **`ast list`.** Enumerate skills in the workspace and show
  validate status / scenario count at a glance.
- [ ] **`CONTRIBUTING.md`.** How to add a provider, how to add a new
  scenario assertion, code style, test conventions.
- [ ] **`golangci-lint` config + lint job in CI.**
- [ ] **Backfill missing test packages.** `internal/runner/`,
  `internal/workspace/`, `internal/scenario/`, `internal/report/` are
  all `[no test files]` today.
- [ ] **`examples/` directory.** A handful of complete skills users
  can copy-paste. The current [skills/go-test-author/](skills/go-test-author/) is a good
  starting template.
- [ ] **Color output / TTY detection.** Optional `--no-color` flag.

---

## Conceptual gap — needs explicit decision before scope grows

- [ ] **What does "compatible with most agents" actually mean for this tool?**
  Two very different interpretations:
  - **(A) Loader-only** — read Claude Code / Cursor / Codex skill
    formats and run them all against ast's internal tool-use loop.
    Already scoped above as "Multi-format skill loader". Modest work.
  - **(B) Runner-level** — actually invoke the external agent CLIs
    (`claude-code`, `cursor-cli`, `codex`) as subprocesses, parse
    their output, intercept their tool calls. Effectively a whole new
    `internal/runner/external/` layer per agent, plus per-agent
    fixtures for the tool-call protocol. Larger than the entire
    current project.

  v0.1 should ship interpretation (A) and explicitly defer (B) to
  a v0.2+ design doc, so the README does not over-promise.

---

## Minimal release path (suggested order)

```
1. Fix matchGlob               (P0, ~half day)
2. Add LICENSE + rename module (P0+P1, ~1 hr)
3. CI workflow                 (P0, ~half day)
4. Release workflow            (P1, ~half day)
5. README install section      (P1, ~1–2 hr)
6. Tag v0.1.0 — first release.
```

Multi-format loader, `ast gen`, and `file_content` assertions are
v0.2/v0.3 territory and do not block v0.1.
