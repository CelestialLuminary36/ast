# go-nil-fixer

A skill that teaches an agent to fix nil-pointer panics in Go with
discipline: always add nil checks, always run `go test`, never touch
`go.mod` or `vendor/`.

This example demonstrates:
- A **complete Anthropic-format skill** (skill.yaml + instructions.md + tools/)
- **Tool whitelist**: grants `read_file`, `edit_file`, `run_command`
- **Scenario assertions**: file mutation allow/forbid lists, mandatory
  command execution, output text checks

## Try it

```bash
ast test ./examples/go-nil-fixer
```

Scenarios live under `scenarios/go-nil-fixer/` — `ast test` discovers
them automatically. No API key needed with the mock runner:

```bash
ast test ./examples/go-nil-fixer --runner=mock
```

## Layout

```
go-nil-fixer/
├── skill.yaml           # id, name, version
├── instructions.md      # the prompt the agent loads
├── tools/
│   ├── read_file.json   # builtin: read_file
│   ├── edit_file.json   # builtin: edit_file
│   └── run_command.json # builtin: run_command
└── README.md
```
