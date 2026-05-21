package workspace

import (
	"os"
	"path/filepath"
)

type Workspace struct {
	Root string
}

func New(base string) (*Workspace, error) {
	root, err := os.MkdirTemp(base, "ast-*")
	if err != nil {
		return nil, err
	}
	return &Workspace{Root: root}, nil
}

func (w *Workspace) Cleanup() error {
	return os.RemoveAll(w.Root)
}

func (w *Workspace) Path(elem ...string) string {
	return filepath.Join(append([]string{w.Root}, elem...)...)
}
