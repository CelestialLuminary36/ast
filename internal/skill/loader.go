package skill

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func LoadFromDir(dir string) (*Skill, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, os.ErrInvalid
	}

	s := &Skill{
		Path: dir,
		Meta: make(map[string]any),
	}

	// Try skill.yaml for metadata
	skillYamlPath := filepath.Join(dir, "skill.yaml")
	if data, err := os.ReadFile(skillYamlPath); err == nil {
		var meta struct {
			ID          string         `yaml:"id"`
			Name        string         `yaml:"name"`
			Description string         `yaml:"description"`
			Version     string         `yaml:"version"`
			Extra       map[string]any `yaml:",inline"`
		}
		if err := yaml.Unmarshal(data, &meta); err == nil {
			s.ID = meta.ID
			s.Name = meta.Name
			if meta.Description != "" {
				s.Meta["description"] = meta.Description
			}
			if meta.Version != "" {
				s.Meta["version"] = meta.Version
			}
			for k, v := range meta.Extra {
				if k != "id" && k != "name" && k != "description" && k != "version" {
					s.Meta[k] = v
				}
			}
		}
	}

	// Fallback name from directory base name
	if s.Name == "" {
		s.Name = filepath.Base(dir)
	}
	if s.ID == "" {
		s.ID = s.Name
	}

	// Load instructions from instructions.md, then skill.md, then README.md
	for _, fname := range []string{"instructions.md", "skill.md", "README.md", "instruction.txt", "instruction.md"} {
		p := filepath.Join(dir, fname)
		if data, err := os.ReadFile(p); err == nil {
			s.Instructions = string(data)
			break
		}
	}

	// Load tools directory if exists
	toolsDir := filepath.Join(dir, "tools")
	if info, err := os.Stat(toolsDir); err == nil && info.IsDir() {
		// For MVP, we just note that tools exist; full parsing is future work
		entries, _ := os.ReadDir(toolsDir)
		var toolNames []string
		for _, e := range entries {
			if !e.IsDir() {
				toolNames = append(toolNames, e.Name())
			}
		}
		if len(toolNames) > 0 {
			s.Meta["tools"] = toolNames
		}
	}

	return s, nil
}
