package scenario

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name  string
		sc    Scenario
		pass  bool
	}{
		{
			name: "valid scenario",
			sc:   Scenario{ID: "test", Input: InputConfig{UserPrompt: "do something"}},
			pass: true,
		},
		{
			name:  "empty id",
			sc:    Scenario{ID: "", Input: InputConfig{UserPrompt: "x"}},
			pass:  false,
		},
		{
			name:  "empty user_prompt",
			sc:    Scenario{ID: "test", Input: InputConfig{UserPrompt: ""}},
			pass:  false,
		},
		{
			name: "empty id and user_prompt",
			sc:   Scenario{ID: "", Input: InputConfig{UserPrompt: ""}},
			pass: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.sc)
			if tt.pass && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !tt.pass && err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestParse(t *testing.T) {
	t.Run("valid yaml", func(t *testing.T) {
		yaml := `id: my-scenario
name: My Scenario
input:
  user_prompt: "do it"
`
		sc, err := Parse(strings.NewReader(yaml))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sc.ID != "my-scenario" {
			t.Errorf("id = %q, want %q", sc.ID, "my-scenario")
		}
	})

	t.Run("missing id defaults to 'default'", func(t *testing.T) {
		yaml := `name: No ID
input:
  user_prompt: "run"
`
		sc, err := Parse(strings.NewReader(yaml))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sc.ID != "default" {
			t.Errorf("id = %q, want %q", sc.ID, "default")
		}
	})

	t.Run("missing name falls back to id", func(t *testing.T) {
		yaml := `id: only-id
input:
  user_prompt: "run"
`
		sc, err := Parse(strings.NewReader(yaml))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sc.Name != "only-id" {
			t.Errorf("name = %q, want %q", sc.Name, "only-id")
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		_, err := Parse(strings.NewReader(`:bad: yaml:`))
		if err == nil {
			t.Error("expected error for invalid yaml")
		}
	})
}

func TestLoadFromDir(t *testing.T) {
	t.Run("loads yaml files from directory", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "one.yaml", `id: one
name: One
input:
  user_prompt: "first"
`)
		// non-yaml file should be ignored
		writeFile(t, dir, "notes.txt", "just some notes")

		scenarios, err := LoadFromDir(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(scenarios) != 1 {
			t.Fatalf("got %d scenarios, want 1", len(scenarios))
		}
		if scenarios[0].ID != "one" {
			t.Errorf("id = %q, want %q", scenarios[0].ID, "one")
		}
	})

	t.Run("fills in id from filename", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "my-test.yaml", `name: My Test
input:
  user_prompt: "go"
`)
		scenarios, err := LoadFromDir(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if scenarios[0].ID != "my-test" {
			t.Errorf("id = %q, want %q", scenarios[0].ID, "my-test")
		}
	})

	t.Run("skips directories", func(t *testing.T) {
		dir := t.TempDir()
		subDir := filepath.Join(dir, "subdir")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, dir, "valid.yaml", `id: valid
input:
  user_prompt: "ok"
`)
		scenarios, err := LoadFromDir(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(scenarios) != 1 {
			t.Fatalf("got %d scenarios, want 1 (subdir should be skipped)", len(scenarios))
		}
	})

	t.Run("bad yaml returns error", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "bad.yaml", `:not: valid: yaml: !!!`)
		_, err := LoadFromDir(dir)
		if err == nil {
			t.Error("expected error for malformed yaml")
		}
	})

	t.Run("empty directory returns empty slice", func(t *testing.T) {
		dir := t.TempDir()
		scenarios, err := LoadFromDir(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(scenarios) != 0 {
			t.Errorf("got %d scenarios, want 0", len(scenarios))
		}
	})
}

func TestLoadFromFile(t *testing.T) {
	t.Run("loads single yaml file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.yaml")
		writeFile(t, dir, "test.yaml", `id: solo
name: Solo
input:
  user_prompt: "single"
`)
		sc, err := LoadFromFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sc.ID != "solo" {
			t.Errorf("id = %q, want %q", sc.ID, "solo")
		}
	})

	t.Run("missing file returns error", func(t *testing.T) {
		_, err := LoadFromFile("/nonexistent/path.yaml")
		if err == nil {
			t.Error("expected error for missing file")
		}
	})
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
