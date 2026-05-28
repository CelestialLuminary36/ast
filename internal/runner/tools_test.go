package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hhy/ast/internal/skill"
)

func TestToolExecutorReadFile(t *testing.T) {
	ws := t.TempDir()
	path := filepath.Join(ws, "hello.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	exec := NewToolExecutor(ws)

	t.Run("read existing file", func(t *testing.T) {
		r := exec.readFile(map[string]any{"path": "hello.txt"})
		if r.Type != "success" {
			t.Fatalf("type = %q, want success", r.Type)
		}
		if r.Content != "hello world" {
			t.Errorf("content = %q, want %q", r.Content, "hello world")
		}
	})

	t.Run("read missing file", func(t *testing.T) {
		r := exec.readFile(map[string]any{"path": "missing.txt"})
		if r.Type != "error" {
			t.Errorf("type = %q, want error", r.Type)
		}
	})

	t.Run("missing path key", func(t *testing.T) {
		r := exec.readFile(map[string]any{})
		if r.Type != "error" {
			t.Errorf("type = %q, want error", r.Type)
		}
	})
}

func TestToolExecutorEditFile(t *testing.T) {
	ws := t.TempDir()
	exec := NewToolExecutor(ws)

	t.Run("create new file", func(t *testing.T) {
		r := exec.editFile(map[string]any{
			"path":    "src/main.go",
			"content": "package main\n",
		})
		if r.Type != "success" {
			t.Fatalf("type = %q, want success (%s)", r.Type, r.Content)
		}
		data, err := os.ReadFile(filepath.Join(ws, "src/main.go"))
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if string(data) != "package main\n" {
			t.Errorf("content = %q", string(data))
		}
	})

	t.Run("overwrite existing file", func(t *testing.T) {
		r := exec.editFile(map[string]any{
			"path":    "src/main.go",
			"content": "package main\n\nfunc main() {}\n",
		})
		if r.Type != "success" {
			t.Fatalf("type = %q, want success", r.Type)
		}
	})

	t.Run("missing path key", func(t *testing.T) {
		r := exec.editFile(map[string]any{"content": "x"})
		if r.Type != "error" {
			t.Errorf("type = %q, want error", r.Type)
		}
	})
}

func TestToolExecutorRunCommand(t *testing.T) {
	ws := t.TempDir()
	exec := NewToolExecutor(ws)

	t.Run("execute simple command", func(t *testing.T) {
		r := exec.runCommand(map[string]any{"command": "echo hello"})
		if r.Type != "success" {
			t.Fatalf("type = %q, want success (%s)", r.Type, r.Content)
		}
		if !strings.Contains(r.Content, "hello") {
			t.Errorf("output should contain 'hello', got %q", r.Content)
		}
	})

	t.Run("records executed command", func(t *testing.T) {
		exec2 := NewToolExecutor(ws)
		exec2.runCommand(map[string]any{"command": "echo one"})
		exec2.runCommand(map[string]any{"command": "echo two"})
		if len(exec2.ExecutedCmds) != 2 {
			t.Fatalf("got %d commands, want 2", len(exec2.ExecutedCmds))
		}
		if exec2.ExecutedCmds[0] != "echo one" {
			t.Errorf("cmd[0] = %q", exec2.ExecutedCmds[0])
		}
		if exec2.ExecutedCmds[1] != "echo two" {
			t.Errorf("cmd[1] = %q", exec2.ExecutedCmds[1])
		}
	})

	t.Run("blocked command", func(t *testing.T) {
		r := exec.runCommand(map[string]any{"command": "rm -rf /"})
		if r.Type != "error" {
			t.Fatalf("type = %q, want error", r.Type)
		}
		if !strings.Contains(r.Content, "blocked") {
			t.Errorf("error should mention 'blocked', got %q", r.Content)
		}
	})

	t.Run("missing command key", func(t *testing.T) {
		r := exec.runCommand(map[string]any{})
		if r.Type != "error" {
			t.Errorf("type = %q, want error", r.Type)
		}
	})
}

func TestToolExecutorListFiles(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "a.txt"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(ws, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "sub", "b.txt"), []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}

	exec := NewToolExecutor(ws)

	t.Run("list root directory", func(t *testing.T) {
		r := exec.listFiles(map[string]any{})
		if r.Type != "success" {
			t.Fatalf("type = %q, want success", r.Type)
		}
		if !strings.Contains(r.Content, "a.txt") {
			t.Errorf("content missing 'a.txt': %q", r.Content)
		}
		if !strings.Contains(r.Content, "sub/") {
			t.Errorf("content missing 'sub/': %q", r.Content)
		}
	})

	t.Run("list subdirectory", func(t *testing.T) {
		r := exec.listFiles(map[string]any{"path": "sub"})
		if r.Type != "success" {
			t.Fatalf("type = %q, want success", r.Type)
		}
		if !strings.Contains(r.Content, "b.txt") {
			t.Errorf("content missing 'b.txt': %q", r.Content)
		}
	})

	t.Run("list missing directory", func(t *testing.T) {
		r := exec.listFiles(map[string]any{"path": "nope"})
		if r.Type != "error" {
			t.Errorf("type = %q, want error", r.Type)
		}
	})
}

func TestToolExecutorUnknownTool(t *testing.T) {
	exec := NewToolExecutor(t.TempDir())
	r := exec.Execute("some_nonexistent_tool", nil)
	if r.Type != "error" {
		t.Errorf("type = %q, want error", r.Type)
	}
	if !strings.Contains(r.Content, "unknown tool") {
		t.Errorf("error message = %q", r.Content)
	}
}

func TestIsBlocked(t *testing.T) {
	tests := []struct {
		cmd     string
		blocked bool
	}{
		{"rm -rf /", true},
		{"rm -rf /*", true},
		{":(){ :|: & };:", true},
		{"> /dev/sda", true},
		{"dd if=/dev/zero of=/dev/sda", true},
		{"mkfs.ext4 /dev/sda", true},
		{"chmod -R 777 /", true},
		{"chown -R root:root /", true},
		{"echo hello", false},
		{"go test ./...", false},
		{"rm some_file.txt", false},
		{"mkdir -p /tmp/build", false},
	}
	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			if got := isBlocked(tt.cmd); got != tt.blocked {
				t.Errorf("isBlocked(%q) = %v, want %v", tt.cmd, got, tt.blocked)
			}
		})
	}
}

func TestStr(t *testing.T) {
	t.Run("string value", func(t *testing.T) {
		v, ok := str(map[string]any{"key": "value"}, "key")
		if !ok || v != "value" {
			t.Errorf("str = (%q, %v), want (\"value\", true)", v, ok)
		}
	})
	t.Run("missing key", func(t *testing.T) {
		_, ok := str(map[string]any{}, "key")
		if ok {
			t.Error("expected false for missing key")
		}
	})
	t.Run("non-string value", func(t *testing.T) {
		_, ok := str(map[string]any{"key": 42}, "key")
		if ok {
			t.Error("expected false for non-string value")
		}
	})
}

func TestBuildSystemPrompt(t *testing.T) {
	sk := skill.Skill{
		ID:           "test-skill",
		Name:         "Test Skill",
		Instructions: "You must use errors.Is for all error comparisons.",
	}
	prompt := buildSystemPrompt(sk)
	if !strings.Contains(prompt, sk.Instructions) {
		t.Error("system prompt missing skill instructions")
	}
	if !strings.Contains(prompt, "Skill Instructions") {
		t.Error("system prompt missing header")
	}
}
