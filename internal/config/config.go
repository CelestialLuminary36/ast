package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Project       string `yaml:"project"`
	ScenariosDir  string `yaml:"scenarios_dir"`
	ReportsDir    string `yaml:"reports_dir"`
	DefaultRunner string `yaml:"default_runner"`
}

func Default() *Config {
	return &Config{
		Project:       "agent-skill-test",
		ScenariosDir:  "./scenarios",
		ReportsDir:    "./reports",
		DefaultRunner: "mock",
	}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.ScenariosDir == "" {
		cfg.ScenariosDir = "./scenarios"
	}
	if cfg.ReportsDir == "" {
		cfg.ReportsDir = "./reports"
	}
	if cfg.DefaultRunner == "" {
		cfg.DefaultRunner = "mock"
	}
	return &cfg, nil
}

func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
