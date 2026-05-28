package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	ws, err := New("")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer ws.Cleanup()

	if ws.Root == "" {
		t.Error("Root is empty")
	}
	info, err := os.Stat(ws.Root)
	if err != nil {
		t.Fatalf("Stat root: %v", err)
	}
	if !info.IsDir() {
		t.Error("Root is not a directory")
	}
}

func TestPath(t *testing.T) {
	ws, err := New("")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer ws.Cleanup()

	p := ws.Path("src", "main.go")
	expected := filepath.Join(ws.Root, "src", "main.go")
	if p != expected {
		t.Errorf("Path = %q, want %q", p, expected)
	}
}

func TestCleanup(t *testing.T) {
	ws, err := New("")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	root := ws.Root
	if err := ws.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	_, err = os.Stat(root)
	if !os.IsNotExist(err) {
		t.Errorf("directory %s should not exist after cleanup", root)
	}
}

func TestNewWithBaseDir(t *testing.T) {
	base := t.TempDir()
	ws, err := New(base)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer ws.Cleanup()

	// Root should be inside the base directory.
	if filepath.Dir(ws.Root) != base {
		t.Errorf("Root dir = %q, want parent %q", filepath.Dir(ws.Root), base)
	}
}
