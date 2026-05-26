package scenario

import (
	"fmt"
	"io"
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

// LoadFromFile reads a single scenario from a YAML file.
func LoadFromFile(path string) (Scenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Scenario{}, err
	}
	var s Scenario
	if err := yaml.Unmarshal(data, &s); err != nil {
		return Scenario{}, fmt.Errorf("parse %s: %w", path, err)
	}
	name := filepath.Base(path)
	if s.ID == "" {
		s.ID = strings.TrimSuffix(name, filepath.Ext(name))
	}
	if s.Name == "" {
		s.Name = s.ID
	}
	return s, nil
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

// Parse reads a single scenario from a YAML reader.
func Parse(r io.Reader) (Scenario, error) {
	var s Scenario
	if err := yaml.NewDecoder(r).Decode(&s); err != nil {
		return Scenario{}, fmt.Errorf("parse scenario: %w", err)
	}
	if s.ID == "" {
		s.ID = "default"
	}
	if s.Name == "" {
		s.Name = s.ID
	}
	return s, nil
}
