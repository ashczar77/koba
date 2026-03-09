package errors

import "strings"

// FriendlyProvider returns a user-friendly message for common provider errors.
func FriendlyProvider(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()

	if strings.Contains(s, "ANTHROPIC_API_KEY") || strings.Contains(s, "not set") {
		return "Anthropic API key is not set. Set ANTHROPIC_API_KEY or add it to ~/.agent/config.yaml"
	}
	if strings.Contains(s, "connection refused") {
		return "Ollama is not running. Start it: open the Ollama app (macOS) or run 'ollama serve'"
	}
	if strings.Contains(s, "404") && strings.Contains(s, "not found") {
		return "Model not found. For Ollama, run: ollama pull codellama (or your chosen model)"
	}
	if strings.Contains(s, "credit balance") || strings.Contains(s, "too low") {
		return "Anthropic account has no credits. Add credits at console.anthropic.com or use Ollama (KOBA_PROVIDER=ollama)"
	}
	if strings.Contains(s, "anthropic API error") {
		return "Anthropic API error: " + s
	}
	if strings.Contains(s, "ollama") && strings.Contains(s, "failed") {
		return "Ollama request failed. Is Ollama running? Try: ollama serve"
	}

	return s
}

// FriendlyGit returns a user-friendly message for common git errors.
func FriendlyGit(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()

	if strings.Contains(s, "not inside a git repository") || strings.Contains(s, "not a git repository") {
		return "Not inside a git repository. Run this command from a project directory that uses git."
	}
	if strings.Contains(s, "uncommitted changes") {
		return "Working tree has uncommitted changes. Commit or stash them first, or use --force to override."
	}

	return s
}
