package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/CelestialLuminary36/agent-skill-test/internal/scenario"
	"github.com/CelestialLuminary36/agent-skill-test/internal/skill"
)

func TestSandboxRunner_InitScriptNotInExecutedCmds(t *testing.T) {
	tmp := t.TempDir()
	marker := filepath.Join(tmp, "marker.txt")

	sk := skill.Skill{Name: "t", Instructions: ""}
	sc := scenario.Scenario{
		ID: "init-not-leaked",
		Environment: scenario.EnvConfig{
			InitScript: "touch " + marker,
		},
		Input: scenario.InputConfig{UserPrompt: "noop"},
	}

	r := NewSandboxRunner()
	res, err := r.Run(context.Background(), sk, sc, tmp)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// init_script 应当执行（标记文件应存在）
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("init script did not run: %v", err)
	}

	// 但不应出现在 ExecutedCmds 里
	for _, c := range res.ExecutedCmds {
		if c == sc.Environment.InitScript {
			t.Fatalf("init script leaked into ExecutedCmds: %q", c)
		}
	}
}
