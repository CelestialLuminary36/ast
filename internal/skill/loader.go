package skill

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	// Load tools/*.json into ToolDefs. Each file must be one Anthropic-format tool definition.
	toolsDir := filepath.Join(dir, "tools")
	toolsInfo, toolsErr := os.Stat(toolsDir)
	if toolsErr == nil && toolsInfo.IsDir() {
		entries, err := os.ReadDir(toolsDir)
		if err != nil {
			return nil, fmt.Errorf("read tools dir: %w", err)
		}
		var names []string
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			p := filepath.Join(toolsDir, e.Name())
			data, err := os.ReadFile(p)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", p, err)
			}
			var td ToolDef
			if err := json.Unmarshal(data, &td); err != nil {
				return nil, fmt.Errorf("parse %s: %w", p, err)
			}
			if td.Name == "" {
				return nil, fmt.Errorf("%s: tool definition is missing required field 'name'", p)
			}
			s.ToolDefs = append(s.ToolDefs, td)
			names = append(names, td.Name)
		}
		if len(names) > 0 {
			s.Meta["tools"] = names
		}
	}

	return s, nil
}
