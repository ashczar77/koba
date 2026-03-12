package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"koba/internal/config"
	"koba/internal/errors"
	"koba/internal/provider"
	"koba/internal/term"
)

// RunChat implements an interactive chat loop with the configured provider.
func RunChat(
	ctx context.Context,
	cfg config.Config,
	in io.Reader,
	out, errOut io.Writer,
	modelOverride string,
	systemPrompt string,
	stream bool,
) error {
	client, err := newProviderClient(cfg, modelOverride)
	if err != nil {
		fmt.Fprintln(errOut, errors.FriendlyProvider(err))
		return err
	}

	w := bufio.NewWriter(out)
	defer w.Flush()

	providerName := providerNameFromEnv(cfg)
	mode := "LIVE"
	switch providerName {
	case "mock":
		mode = "MOCK"
	case "ollama":
		mode = "LOCAL"
	}
	banner := term.Banner(strings.ToUpper(providerName), modelForDisplay(providerName, cfg, modelOverride), mode)
	fmt.Fprintln(w, banner)

	var messages []provider.Message
	if systemPrompt != "" {
		messages = append(messages, provider.Message{
			Role:    provider.RoleSystem,
			Content: systemPrompt,
		})
	}

	scanner := bufio.NewScanner(in)
	for {
		fmt.Fprint(w, term.UserPrefix())
		w.Flush()

		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		messages = append(messages, provider.Message{
			Role:    provider.RoleUser,
			Content: line,
		})

		stopSpinner := term.StartSpinner(errOut, "Koba is thinking...")
		streamObj, err := client.Chat(ctx, messages, provider.ChatOptions{
			Model:       modelOverride,
			Temperature: cfg.Temperature,
			Stream:      stream,
		})
		if err != nil {
			stopSpinner()
			fmt.Fprintln(errOut, errors.FriendlyProvider(err))
			continue
		}

		var respText strings.Builder
		prefixPrinted := false
		for {
			chunk, err := streamObj.Recv(ctx)
			if err != nil {
				if err != io.EOF {
					stopSpinner()
					fmt.Fprintln(errOut, "stream error:", err)
				}
				break
			}
			if chunk.Text != "" {
				stopSpinner()
				if !prefixPrinted {
					fmt.Fprint(w, term.AssistantPrefix())
					prefixPrinted = true
				}
				respText.WriteString(chunk.Text)
				fmt.Fprint(w, chunk.Text)
				w.Flush()
			}
			if chunk.Done {
				break
			}
		}
		stopSpinner() // ensure stopped on any exit path
		_ = streamObj.Close()
		if prefixPrinted {
			fmt.Fprintln(w)
		}

		messages = append(messages, provider.Message{
			Role:    provider.RoleAssistant,
			Content: respText.String(),
		})
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		fmt.Fprintln(errOut, "input error:", err)
	}

	return nil
}

// RunAsk performs a single-turn question and prints the answer, then exits.
func RunAsk(
	ctx context.Context,
	cfg config.Config,
	in io.Reader,
	out, errOut io.Writer,
	args []string,
	modelOverride string,
	systemPrompt string,
) error {
	question := strings.TrimSpace(strings.Join(args, " "))
	if question == "" {
		data, err := io.ReadAll(in)
		if err != nil {
			return err
		}
		question = strings.TrimSpace(string(data))
	}
	if question == "" {
		return fmt.Errorf("no question provided")
	}

	client, err := newProviderClient(cfg, modelOverride)
	if err != nil {
		fmt.Fprintln(errOut, errors.FriendlyProvider(err))
		return err
	}

	var messages []provider.Message
	if systemPrompt != "" {
		messages = append(messages, provider.Message{
			Role:    provider.RoleSystem,
			Content: systemPrompt,
		})
	}
	messages = append(messages, provider.Message{
		Role:    provider.RoleUser,
		Content: question,
	})

	stopSpinner := term.StartSpinner(errOut, "Koba is thinking...")
	streamObj, err := client.Chat(ctx, messages, provider.ChatOptions{
		Model:       modelOverride,
		Temperature: cfg.Temperature,
		Stream:      true,
	})
	if err != nil {
		stopSpinner()
		return err
	}
	defer stopSpinner()
	defer streamObj.Close()

	w := bufio.NewWriter(out)
	defer w.Flush()

	prefixPrinted := false
	for {
		chunk, err := streamObj.Recv(ctx)
		if err != nil {
			if err != io.EOF {
				stopSpinner()
				fmt.Fprintln(errOut, "stream error:", err)
			}
			break
		}
		if chunk.Text != "" {
			stopSpinner()
			if !prefixPrinted {
				fmt.Fprint(w, term.AssistantPrefix())
				prefixPrinted = true
			}
			fmt.Fprint(w, chunk.Text)
			w.Flush()
		}
		if chunk.Done {
			break
		}
	}
	if prefixPrinted {
		fmt.Fprintln(w)
	}
	return nil
}

func chooseModel(cfg config.Config, override string) string {
	if override != "" {
		return override
	}
	if cfg.DefaultModel != "" {
		return cfg.DefaultModel
	}
	return "claude-3-haiku-20240307"
}

// modelForDisplay returns the model name to show in the UI (banner, doctor).
// It applies the same provider-specific mapping as newProviderClient so the
// displayed model matches what is actually used (e.g. Ollama shows "codellama"
// when config default is an Anthropic model).
func modelForDisplay(providerName string, cfg config.Config, override string) string {
	model := chooseModel(cfg, override)
	switch providerName {
	case "ollama":
		if strings.Contains(model, "claude") {
			// Map Anthropic-style default (e.g. "claude-3-haiku") to an Ollama model that supports tools.
			return "qwen3"
		}
		if model == "" {
			// Default Ollama model; choose one that supports tools.
			return "qwen3"
		}
		return model
	case "mock":
		return "mock"
	default:
		return model
	}
}

// newProviderClient chooses which provider to use based on environment and
// config. Env var KOBA_PROVIDER takes precedence over config.DefaultProvider.
// Supported values: "anthropic", "ollama", "mock".
func newProviderClient(cfg config.Config, modelOverride string) (provider.Provider, error) {
	name := providerNameFromEnv(cfg)
	switch name {
	case "mock":
		return provider.NewMockClient(), nil
	case "ollama":
		baseURL := cfg.OllamaBaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		model := chooseModel(cfg, modelOverride)
		if strings.Contains(model, "claude") {
			// When config/default model is a Claude model name, map it to an Ollama model that supports tools.
			model = "qwen3"
		}
		if model == "" {
			// Default Ollama model; choose one that supports tools.
			model = "qwen3"
		}
		return provider.NewOllamaClient(baseURL, model)
	case "anthropic", "":
		fallthrough
	default:
		return provider.NewAnthropicClient(cfg.AnthropicAPIKey, chooseModel(cfg, modelOverride))
	}
}

func providerNameFromEnv(cfg config.Config) string {
	name := os.Getenv("KOBA_PROVIDER")
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(cfg.DefaultProvider))
	}
	if name == "" {
		name = "anthropic"
	}
	return name
}




