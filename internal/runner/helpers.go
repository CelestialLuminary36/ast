package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hhy/ast/internal/scenario"
)

func prepareWorkspace(ctx context.Context, sc scenario.Scenario, ws string) error {
	if sc.Environment.FixtureDir != "" {
		if err := copyDir(sc.Environment.FixtureDir, ws); err != nil {
			return fmt.Errorf("copy fixture: %w", err)
		}
	}
	if sc.Environment.InitScript != "" {
		cmd := exec.CommandContext(ctx, "sh", "-c", sc.Environment.InitScript)
		cmd.Dir = ws
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("init script: %w\n%s", err, out)
		}
	}
	_ = runGit(ctx, ws, "init")
	_ = runGit(ctx, ws, "add", ".")
	_ = runGit(ctx, ws, "commit", "-m", "init", "--allow-empty")
	return nil
}

func runGit(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	_, err := cmd.CombinedOutput()
	return err
}

func captureMutations(ctx context.Context, dir string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 3 {
			continue
		}
		fname := strings.TrimSpace(line[2:])
		if fname != "" {
			files = append(files, fname)
		}
	}
	return files, nil
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				return err
			}
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				return err
			}
		}
	}
	return nil
}
