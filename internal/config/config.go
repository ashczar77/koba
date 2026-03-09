package config

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ProjectConfig holds project-scoped overrides (subset of Config).
type ProjectConfig struct {
	DefaultProvider string  `yaml:"default_provider"`
	DefaultModel    string  `yaml:"default_model"`
	Temperature     float32 `yaml:"temperature"`
	SystemPrompt   string  `yaml:"system_prompt"`
}

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

	// OllamaBaseURL is the Ollama API base URL (default: http://localhost:11434).
	OllamaBaseURL string `yaml:"ollama_base_url"`

	// ProjectRoot is set when a project config was found; empty otherwise.
	ProjectRoot string `yaml:"-"`
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
	return LoadForDir("")
}

// LoadForDir loads config, merging project-scoped .koba/config.yaml from
// dir or any parent up to repo root. Pass "" to use current working directory.
func LoadForDir(dir string) (Config, error) {
	cfg := Defaults()

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, err
	}

	if dir == "" {
		dir, err = os.Getwd()
		if err != nil {
			return cfg, err
		}
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return cfg, err
	}

	// Global config: ~/.agent/config.yaml
	globalPath := filepath.Join(home, ".agent", "config.yaml")
	data, err := os.ReadFile(globalPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return cfg, err
	}
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, err
		}
	}

	// Project config: .koba/config.yaml in dir or parents (stop at home or repo root)
	for d := dir; d != "" && d != filepath.Dir(d); d = filepath.Dir(d) {
		if d == home {
			break
		}
		projPath := filepath.Join(d, ".koba", "config.yaml")
		pdata, err := os.ReadFile(projPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return cfg, err
		}
		var proj ProjectConfig
		if err := yaml.Unmarshal(pdata, &proj); err != nil {
			return cfg, err
		}
		if proj.DefaultProvider != "" {
			cfg.DefaultProvider = proj.DefaultProvider
		}
		if proj.DefaultModel != "" {
			cfg.DefaultModel = proj.DefaultModel
		}
		if proj.Temperature != 0 {
			cfg.Temperature = proj.Temperature
		}
		cfg.ProjectRoot = d
		break
	}

	// Environment overrides.
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
		cfg.AnthropicAPIKey = v
	}
	if v := os.Getenv("OLLAMA_HOST"); v != "" {
		cfg.OllamaBaseURL = "http://" + v
	}

	return cfg, nil
}

