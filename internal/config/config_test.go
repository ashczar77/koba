package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()
	if cfg.DefaultProvider != "anthropic" {
		t.Errorf("expected provider anthropic, got %s", cfg.DefaultProvider)
	}
	if cfg.Temperature != 0.2 {
		t.Errorf("expected temperature 0.2, got %f", cfg.Temperature)
	}
}

func TestLoadForDir_ProjectConfig(t *testing.T) {
	dir := t.TempDir()
	kobaDir := filepath.Join(dir, ".koba")
	os.MkdirAll(kobaDir, 0755)
	os.WriteFile(filepath.Join(kobaDir, "config.yaml"), []byte(`
default_provider: ollama
default_model: codellama
temperature: 0
system_prompt: "test prompt"
max_tokens: 8192
`), 0644)

	cfg, err := LoadForDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultProvider != "ollama" {
		t.Errorf("expected ollama, got %s", cfg.DefaultProvider)
	}
	if cfg.DefaultModel != "codellama" {
		t.Errorf("expected codellama, got %s", cfg.DefaultModel)
	}
	if cfg.Temperature != 0 {
		t.Errorf("expected temperature 0, got %f", cfg.Temperature)
	}
	if cfg.SystemPrompt != "test prompt" {
		t.Errorf("expected system prompt, got %q", cfg.SystemPrompt)
	}
	if cfg.MaxTokens != 8192 {
		t.Errorf("expected max_tokens 8192, got %d", cfg.MaxTokens)
	}
	if cfg.ProjectRoot != dir {
		t.Errorf("expected project root %s, got %s", dir, cfg.ProjectRoot)
	}
}

func TestOllamaHostNoDoublePrefix(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://myhost:11434")
	cfg, err := LoadForDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OllamaBaseURL != "http://myhost:11434" {
		t.Errorf("expected http://myhost:11434, got %s", cfg.OllamaBaseURL)
	}
}

func TestOllamaHostPlainHost(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "myhost:11434")
	cfg, err := LoadForDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OllamaBaseURL != "http://myhost:11434" {
		t.Errorf("expected http://myhost:11434, got %s", cfg.OllamaBaseURL)
	}
}
