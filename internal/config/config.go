package config

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds user-configurable settings for koba.
type Config struct {
	// DefaultProvider is the name of the LLM provider to use, e.g. "anthropic".
	DefaultProvider string `yaml:"default_provider"`

	// DefaultModel is the default model identifier for the selected provider.
	DefaultModel string `yaml:"default_model"`

	// Temperature controls sampling randomness.
	Temperature float32 `yaml:"temperature"`

	// AnthropicAPIKey is optionally read from config; environment takes precedence.
	AnthropicAPIKey string `yaml:"anthropic_api_key"`
}

// Defaults returns a Config populated with sensible defaults.
func Defaults() Config {
	return Config{
		DefaultProvider: "anthropic",
		DefaultModel:    "claude-3-haiku-20240307",
		Temperature:     0.2,
	}
}

// Load reads configuration from ~/.agent/config.yaml if it exists and
// overlays environment variables such as ANTHROPIC_API_KEY.
func Load() (Config, error) {
	cfg := Defaults()

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, err
	}

	path := filepath.Join(home, ".agent", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return cfg, err
	}

	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, err
		}
	}

	// Environment overrides.
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
		cfg.AnthropicAPIKey = v
	}

	return cfg, nil
}

