# read-only-reviewer

A code-review skill that deliberately **does not grant `run_command`**.
The agent can read and edit files but cannot execute shell commands.

This example demonstrates:
- **Tool whitelist with only 2 builtins**: `read_file` and `edit_file`
- **`command_execution` assertions**: `must_have: []` (no commands expected)
- Why you'd restrict tools: for a review skill that should only annotate,
  not run arbitrary code

## Try it

```bash
ast test ./examples/read-only-reviewer
```

Compare the `tools/` directory with `go-nil-fixer` — same format,
different whitelist. `ast validate` will confirm the whitelist is
correct.

## Layout

```
read-only-reviewer/
├── skill.yaml
├── instructions.md
├── tools/
│   ├── read_file.json
│   └── edit_file.json
└── README.md
```
