package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"koba/internal/config"
	"koba/internal/term"
)

// RunDoctor runs provider diagnostics and prints a summary.
func RunDoctor(cfg config.Config, out, errOut io.Writer) error {
	providerName := providerNameFromEnv(cfg)
	model := modelForDisplay(providerName, cfg, "")

	w := out
	fmt.Fprintf(w, "%sKoba Doctor%s\n\n", term.ColorMagenta(), term.ColorReset())
	fmt.Fprintf(w, "Provider:  %s%s%s (from %s)\n",
		term.ColorCyan(), strings.ToUpper(providerName), term.ColorReset(),
		providerSource(cfg))
	fmt.Fprintf(w, "Model:      %s%s%s\n\n", term.ColorGreen(), model, term.ColorReset())

	switch providerName {
	case "anthropic":
		checkAnthropic(cfg, w)
	case "ollama":
		checkOllama(cfg, w, providerName)
	case "mock":
		fmt.Fprintf(w, "%sMock provider: no external checks needed.%s\n", term.ColorDim(), term.ColorReset())
	default:
		fmt.Fprintf(w, "%sUnknown provider.%s\n", term.ColorDim(), term.ColorReset())
	}

	if cfg.ProjectRoot != "" {
		fmt.Fprintf(w, "\nProject config: %s%s%s\n", term.ColorDim(), cfg.ProjectRoot, term.ColorReset())
	}

	return nil
}

func providerSource(cfg config.Config) string {
	if os.Getenv("KOBA_PROVIDER") != "" {
		return "KOBA_PROVIDER"
	}
	if cfg.ProjectRoot != "" {
		return ".koba/config.yaml"
	}
	return "~/.agent/config.yaml or default"
}

func checkAnthropic(cfg config.Config, w io.Writer) {
	key := cfg.AnthropicAPIKey
	if key == "" {
		fmt.Fprintf(w, "%s✗ ANTHROPIC_API_KEY is not set.%s\n", term.ColorRed(), term.ColorReset())
		fmt.Fprintf(w, "  Set it: export ANTHROPIC_API_KEY=sk-ant-...\n")
		return
	}
	masked := key
	if len(key) > 12 {
		masked = key[:8] + "..." + key[len(key)-4:]
	}
	fmt.Fprintf(w, "%s✓ Anthropic key set%s (%s)\n", term.ColorGreen(), term.ColorReset(), masked)
	fmt.Fprintf(w, "  (We don't validate the key here to avoid API cost.)\n")
}

func checkOllama(cfg config.Config, w io.Writer, providerName string) {
	baseURL := cfg.OllamaBaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL + "/api/tags")
	if err != nil {
		fmt.Fprintf(w, "%s✗ Ollama not reachable%s at %s\n", term.ColorRed(), term.ColorReset(), baseURL)
		fmt.Fprintf(w, "  Error: %v\n", err)
		fmt.Fprintf(w, "  Start Ollama: open the app or run 'ollama serve'\n")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Fprintf(w, "%s✗ Ollama returned %s%s\n", term.ColorRed(), resp.Status, term.ColorReset())
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(w, "%s✗ Failed to read Ollama response%s\n", term.ColorRed(), term.ColorReset())
		return
	}

	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &tags); err != nil {
		fmt.Fprintf(w, "%s✓ Ollama reachable%s (could not parse model list)\n", term.ColorGreen(), term.ColorReset())
		return
	}

	fmt.Fprintf(w, "%s✓ Ollama reachable%s at %s\n", term.ColorGreen(), term.ColorReset(), baseURL)
	if len(tags.Models) == 0 {
		fmt.Fprintf(w, "  No models pulled yet. Run: ollama pull codellama\n")
		return
	}
	fmt.Fprintf(w, "  Models: ")
	var names []string
	for _, m := range tags.Models {
		names = append(names, m.Name)
	}
	fmt.Fprintf(w, "%s\n", strings.Join(names, ", "))

	// Check if configured model is available
	model := modelForDisplay(providerName, cfg, "")
	found := false
	for _, m := range tags.Models {
		if m.Name == model || strings.HasPrefix(m.Name, model+":") {
			found = true
			break
		}
	}
	if !found {
		fmt.Fprintf(w, "  %s⚠ Model '%s' not found. Pull it: ollama pull %s%s\n",
			term.ColorYellow(), model, model, term.ColorReset())
	}
}
