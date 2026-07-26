package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ModelConfig holds configuration for a single LLM model.
type ModelConfig struct {
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
	ModelID string `yaml:"model_id"`
}

// Config holds application configuration loaded from YAML.
type Config struct {
	ModelSelect string                `yaml:"model_select"`
	Models      map[string]ModelConfig `yaml:"models"`
}

// DefaultPath returns the default config file path.
func DefaultPath() string {
	return "config.yaml"
}

// SelectedModel returns the active ModelConfig based on ModelSelect.
func (c *Config) SelectedModel() (ModelConfig, error) {
	if c.ModelSelect == "" {
		return ModelConfig{}, fmt.Errorf("model_select is not set")
	}
	m, ok := c.Models[c.ModelSelect]
	if !ok {
		return ModelConfig{}, fmt.Errorf("model %q not found in models", c.ModelSelect)
	}
	return m, nil
}

// Load reads and parses the YAML config file at path.
// Environment variables BLEATCODE_API_KEY, BLEATCODE_BASE_URL, BLEATCODE_MODEL_ID
// override the selected model's values when set.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	// Environment variable overrides on the selected model
	if cfg.Models == nil {
		cfg.Models = make(map[string]ModelConfig)
	}
	if cfg.ModelSelect != "" {
		m := cfg.Models[cfg.ModelSelect]
		if v := os.Getenv("BLEATCODE_API_KEY"); v != "" {
			m.APIKey = v
		}
		if v := os.Getenv("BLEATCODE_BASE_URL"); v != "" {
			m.BaseURL = v
		}
		if v := os.Getenv("BLEATCODE_MODEL_ID"); v != "" {
			m.ModelID = v
		}
		cfg.Models[cfg.ModelSelect] = m
	}

	// Validate
	sel, err := cfg.SelectedModel()
	if err != nil {
		return nil, err
	}
	if sel.APIKey == "" {
		return nil, fmt.Errorf("api_key is required for model %q", cfg.ModelSelect)
	}
	if sel.ModelID == "" {
		return nil, fmt.Errorf("model_id is required for model %q", cfg.ModelSelect)
	}

	return &cfg, nil
}
