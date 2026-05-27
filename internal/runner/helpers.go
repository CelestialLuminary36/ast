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

func captureMutations(ctx context.Context, dir string) ([]string, map[string]string, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, nil, err
	}

	var files []string
	contents := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 3 {
			continue
		}
		// Porcelain status code lives in cols 1-2. " D" / "D " mean deletion,
		// in which case there is no file to read.
		status := line[:2]
		fname := strings.TrimSpace(line[2:])
		if fname == "" {
			continue
		}
		// Git renames render as "R  old -> new"; take the new path.
		if idx := strings.Index(fname, " -> "); idx >= 0 {
			fname = fname[idx+4:]
		}
		files = append(files, fname)

		if strings.ContainsRune(status, 'D') {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, fname))
		if readErr != nil {
			// Best-effort: a binary or unreadable file shouldn't fail the run.
			// The judge sees an absent key and can decide what to do.
			continue
		}
		contents[fname] = string(data)
	}
	return files, contents, nil
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
