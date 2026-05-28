package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Project      string         `yaml:"project"`
	ScenariosDir string         `yaml:"scenarios_dir"`
	ReportsDir   string         `yaml:"reports_dir"`
	API          APIConfig      `yaml:"api"`
	Provider     ProviderConfig `yaml:"provider"`
}

type APIConfig struct {
	Key      string `yaml:"key"`
	Model    string `yaml:"model"`
	Endpoint string `yaml:"endpoint"`
	Timeout  int    `yaml:"timeout"` // seconds
}

type ProviderConfig struct {
	Type     string `yaml:"type"` // "anthropic", "openai", "ollama"
	Key      string `yaml:"key"`
	Model    string `yaml:"model"`
	Endpoint string `yaml:"endpoint"`
	Timeout  int    `yaml:"timeout"` // seconds
}

// ResolveProvider merges the new provider: section and the deprecated api:
// section. If provider.type is set, it takes precedence. Otherwise the legacy
// api: fields are mapped to an anthropic provider for backward compatibility.
func (c *Config) ResolveProvider() ProviderConfig {
	if c.Provider.Type != "" {
		pc := c.Provider
		if pc.Timeout <= 0 {
			pc.Timeout = 120
		}
		return pc
	}
	// Backward compat: api: section → anthropic provider
	pc := ProviderConfig{
		Type:     "anthropic",
		Key:      c.API.Key,
		Model:    c.API.Model,
		Endpoint: c.API.Endpoint,
		Timeout:  c.API.Timeout,
	}
	if pc.Model == "" {
		pc.Model = "claude-sonnet-4-6"
	}
	if pc.Endpoint == "" {
		pc.Endpoint = "https://api.anthropic.com/v1/messages"
	}
	if pc.Timeout <= 0 {
		pc.Timeout = 120
	}
	return pc
}

func Default() *Config {
	return &Config{
		Project:      "ast",
		ScenariosDir: "./scenarios",
		ReportsDir:   "./reports",
		API: APIConfig{
			Model:    "claude-sonnet-4-6",
			Endpoint: "https://api.anthropic.com/v1/messages",
			Timeout:  120,
		},
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
	if cfg.API.Model == "" {
		cfg.API.Model = "claude-sonnet-4-6"
	}
	if cfg.API.Endpoint == "" {
		cfg.API.Endpoint = "https://api.anthropic.com/v1/messages"
	}
	if cfg.API.Timeout <= 0 {
		cfg.API.Timeout = 120
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
