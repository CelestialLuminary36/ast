package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ToolResult is the outcome of a single tool invocation.
type ToolResult struct {
	Type    string `json:"type"`    // "success" | "error"
	Content string `json:"content"` // result or error message
}

// ToolExecutor holds the workspace path and records executed commands.
type ToolExecutor struct {
	Workspace    string
	ExecutedCmds []string
}

func NewToolExecutor(ws string) *ToolExecutor {
	return &ToolExecutor{Workspace: ws}
}

// Execute routes tool_name to the actual handler.
func (e *ToolExecutor) Execute(toolName string, input map[string]any) ToolResult {
	switch toolName {
	case "read_file":
		return e.readFile(input)
	case "edit_file":
		return e.editFile(input)
	case "run_command":
		return e.runCommand(input)
	case "list_files":
		return e.listFiles(input)
	default:
		return ToolResult{Type: "error", Content: fmt.Sprintf("unknown tool: %s", toolName)}
	}
}

func (e *ToolExecutor) readFile(input map[string]any) ToolResult {
	path, ok := str(input, "path")
	if !ok {
		return ToolResult{Type: "error", Content: "missing 'path'"}
	}
	full := filepath.Join(e.Workspace, path)
	data, err := os.ReadFile(full)
	if err != nil {
		return ToolResult{Type: "error", Content: err.Error()}
	}
	return ToolResult{Type: "success", Content: string(data)}
}

func (e *ToolExecutor) editFile(input map[string]any) ToolResult {
	path, ok := str(input, "path")
	if !ok {
		return ToolResult{Type: "error", Content: "missing 'path'"}
	}
	content, _ := str(input, "content")
	full := filepath.Join(e.Workspace, path)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return ToolResult{Type: "error", Content: err.Error()}
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		return ToolResult{Type: "error", Content: err.Error()}
	}
	return ToolResult{Type: "success", Content: fmt.Sprintf("wrote %d bytes to %s", len(content), path)}
}

func (e *ToolExecutor) runCommand(input map[string]any) ToolResult {
	cmdStr, ok := str(input, "command")
	if !ok {
		return ToolResult{Type: "error", Content: "missing 'command'"}
	}

	// Safety block: reject dangerous commands
	if isBlocked(cmdStr) {
		return ToolResult{Type: "error", Content: fmt.Sprintf("command blocked by safety policy: %s", cmdStr)}
	}

	// Record command before execution
	e.ExecutedCmds = append(e.ExecutedCmds, cmdStr)

	// Execute
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return ToolResult{Type: "error", Content: "empty command"}
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = e.Workspace
	out, err := cmd.CombinedOutput()
	result := string(out)
	if err != nil {
		result += fmt.Sprintf("\n[exit error: %v]", err)
	}
	return ToolResult{Type: "success", Content: result}
}

func (e *ToolExecutor) listFiles(input map[string]any) ToolResult {
	dir, _ := str(input, "path")
	if dir == "" {
		dir = "."
	}
	full := filepath.Join(e.Workspace, dir)
	entries, err := os.ReadDir(full)
	if err != nil {
		return ToolResult{Type: "error", Content: err.Error()}
	}
	var names []string
	for _, entry := range entries {
		suffix := ""
		if entry.IsDir() {
			suffix = "/"
		}
		names = append(names, entry.Name()+suffix)
	}
	return ToolResult{Type: "success", Content: strings.Join(names, "\n")}
}

// isBlocked checks if a command matches a dangerous pattern.
func isBlocked(cmd string) bool {
	lower := strings.ToLower(cmd)
	blocked := []string{
		"rm -rf /", "rm -rf /*", "rm -rf ~", "rm -rf ~/*",
		":(){ :|: & };:", "> /dev/sda", "dd if=/dev/zero",
		"mkfs.", "chmod -R 777 /", "chown -R",
	}
	for _, p := range blocked {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

func str(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}
