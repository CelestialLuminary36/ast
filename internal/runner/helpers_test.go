package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hhy/ast/internal/scenario"
)

func TestCopyDir(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create fixtures in src.
	mustWrite(t, filepath.Join(src, "a.txt"), "alpha")
	mustWrite(t, filepath.Join(src, "sub", "b.txt"), "beta")
	if err := os.MkdirAll(filepath.Join(src, "empty"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir: %v", err)
	}

	checkFile(t, filepath.Join(dst, "a.txt"), "alpha")
	checkFile(t, filepath.Join(dst, "sub", "b.txt"), "beta")

	// Empty directory should be preserved.
	info, err := os.Stat(filepath.Join(dst, "empty"))
	if err != nil || !info.IsDir() {
		t.Error("empty dir was not copied")
	}
}

func TestPrepareWorkspace(t *testing.T) {
	ws := t.TempDir()

	t.Run("no fixture, no init script", func(t *testing.T) {
		sc := scenario.Scenario{ID: "plain"}
		if err := prepareWorkspace(context.Background(), sc, ws); err != nil {
			t.Fatalf("prepareWorkspace: %v", err)
		}
		// git init should have created a .git directory.
		if _, err := os.Stat(filepath.Join(ws, ".git")); err != nil {
			t.Error(".git directory not created")
		}
	})

	t.Run("with fixture directory", func(t *testing.T) {
		ws2 := t.TempDir()
		fx := t.TempDir()
		mustWrite(t, filepath.Join(fx, "README.md"), "# hello")

		sc := scenario.Scenario{
			ID: "with-fixture",
			Environment: scenario.EnvConfig{
				FixtureDir: fx,
			},
		}
		if err := prepareWorkspace(context.Background(), sc, ws2); err != nil {
			t.Fatalf("prepareWorkspace: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(ws2, "README.md"))
		if err != nil {
			t.Fatalf("fixture file not copied: %v", err)
		}
		if string(data) != "# hello" {
			t.Errorf("fixture content = %q", string(data))
		}
	})

	t.Run("with init script", func(t *testing.T) {
		ws3 := t.TempDir()
		sc := scenario.Scenario{
			ID: "init-script",
			Environment: scenario.EnvConfig{
				InitScript: "echo init-ran > marker.txt",
			},
		}
		if err := prepareWorkspace(context.Background(), sc, ws3); err != nil {
			t.Fatalf("prepareWorkspace: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(ws3, "marker.txt"))
		if err != nil {
			t.Fatalf("init script marker: %v", err)
		}
		if strings.TrimSpace(string(data)) != "init-ran" {
			t.Errorf("marker = %q", strings.TrimSpace(string(data)))
		}
	})
}

func TestCaptureMutations(t *testing.T) {
	// Skip if git is not available.
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	ctx := context.Background()

	gitRun := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	gitRun("init")
	gitRun("config", "user.email", "test@test.com")
	gitRun("config", "user.name", "test")
	// Create and commit an initial file.
	mustWrite(t, filepath.Join(dir, "existing.txt"), "original")
	gitRun("add", "existing.txt")
	gitRun("commit", "-m", "init")

	// Modify the file to create a mutation.
	mustWrite(t, filepath.Join(dir, "existing.txt"), "modified")
	mustWrite(t, filepath.Join(dir, "new.go"), "package main")

	files, contents, err := captureMutations(ctx, dir)
	if err != nil {
		t.Fatalf("captureMutations: %v", err)
	}

	if len(files) < 2 {
		t.Fatalf("got %d mutated files, want at least 2", len(files))
	}

	if !slices.Contains(files, "new.go") {
		t.Error("missing new.go in mutated files")
	}
	if !slices.Contains(files, "existing.txt") {
		t.Error("missing existing.txt in mutated files")
	}

	if v, ok := contents["existing.txt"]; !ok || v != "modified" {
		t.Errorf("contents[existing.txt] = %q, want %q", v, "modified")
	}
	if v, ok := contents["new.go"]; !ok || v != "package main" {
		t.Errorf("contents[new.go] = %q, want %q", v, "package main")
	}
}

func TestCaptureMutationsDeletedFile(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	ctx := context.Background()

	gitRun := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	gitRun("init")
	gitRun("config", "user.email", "test@test.com")
	gitRun("config", "user.name", "test")
	mustWrite(t, filepath.Join(dir, "delete-me.txt"), "to be deleted")
	gitRun("add", "delete-me.txt")
	gitRun("commit", "-m", "init")
	os.Remove(filepath.Join(dir, "delete-me.txt"))

	files, contents, err := captureMutations(ctx, dir)
	if err != nil {
		t.Fatalf("captureMutations: %v", err)
	}

	if !slices.Contains(files, "delete-me.txt") {
		t.Error("deleted file should appear in mutated files list")
	}
	// Deleted files should NOT have captured content.
	if _, ok := contents["delete-me.txt"]; ok {
		t.Error("deleted file should not have captured content")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func checkFile(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("ReadFile %s: %v", path, err)
		return
	}
	if string(data) != want {
		t.Errorf("%s = %q, want %q", path, string(data), want)
	}
}
