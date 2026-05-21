package scenario

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func LoadFromDir(dir string) ([]Scenario, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var scenarios []Scenario
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		var s Scenario
		if err := yaml.Unmarshal(data, &s); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		if s.ID == "" {
			s.ID = strings.TrimSuffix(name, filepath.Ext(name))
		}
		if s.Name == "" {
			s.Name = s.ID
		}
		scenarios = append(scenarios, s)
	}
	return scenarios, nil
}

func Validate(s Scenario) error {
	if s.ID == "" {
		return fmt.Errorf("scenario id is required")
	}
	if s.Input.UserPrompt == "" {
		return fmt.Errorf("scenario %s: input.user_prompt is required", s.ID)
	}
	return nil
}
